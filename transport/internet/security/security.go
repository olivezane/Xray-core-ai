// Package security is the single owner of client-side transport security:
// it layers the TCP mask chain and TLS/REALITY behind one deep entry point
// (WrapConnClient), enforcing the fixed mask-before-security order in
// exactly one place. Transports declare their variation points via
// SecurityHooks instead of hand-writing TLS branches.
//
// This is a leaf package: it imports internet, tls and reality, which all
// import internet back, so the code cannot live inside internet itself.
package security

import (
	"context"
	gotls "crypto/tls"
	"slices"
	"strings"

	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/session"
	"github.com/xtls/xray-core/transport/internet"
	"github.com/xtls/xray-core/transport/internet/reality"
	"github.com/xtls/xray-core/transport/internet/tls"
)

// HandshakeStyle selects when and how the client-side TLS handshake runs.
type HandshakeStyle int

const (
	// HandshakeLazy defers the handshake to the first read/write on the
	// connection (the crypto/tls default).
	HandshakeLazy HandshakeStyle = iota
	// HandshakeTLS handshakes eagerly with the standard client flow.
	HandshakeTLS
	// HandshakeWebsocket handshakes eagerly, forcing outer ALPN to
	// http/1.1 (websocket-style camouflage).
	HandshakeWebsocket
	// HandshakeAuto handshakes eagerly, choosing websocket-style ALPN iff
	// the effective NextProtos is exactly ["http/1.1"] (RAW TCP behaviour,
	// including manually configured ALPN).
	HandshakeAuto
)

// SecurityHooks declares how a transport wants client-side TLS/REALITY
// layered by WrapConnClient. The mask-before-security order is enforced by
// the implementation; transports only declare their variation points here
// instead of hand-writing TLS branches.
type SecurityHooks struct {
	// RequestALPN, when non-empty, requests the ALPN via tls.WithNextProto
	// (user-configured ALPN still wins inside GetTLSConfig).
	RequestALPN string

	// UTLSHandshake selects the eager handshake style for the uTLS
	// (fingerprinted) path. Zero value keeps the handshake lazy.
	UTLSHandshake HandshakeStyle

	// PlainTLSHandshake eagerly handshakes plain go-tls clients
	// (no fingerprint configured). Only RAW TCP wants this today.
	PlainTLSHandshake bool

	// PostHandshake runs after a successful eager uTLS handshake (e.g.
	// websocket hostname verification). Ignored on other paths.
	PostHandshake func(*tls.UConn) error

	// WithReality opts the transport into REALITY wrapping when a REALITY
	// config is present in the stream settings. Transports that never
	// supported REALITY must leave this false to preserve behaviour.
	WithReality bool
}

// WrapConnClient applies the configured TCP mask chain to a client-side raw
// connection, then layers client security (TLS/REALITY per hooks) on top —
// the fixed mask-before-security order. hooks == nil keeps the connection
// mask-only (for transports that layer their own security elsewhere, e.g.
// websocket handing the masked conn to gorilla). The raw connection is
// returned unchanged when no mask is configured. On wrap error the raw
// connection is closed.
func WrapConnClient(mss *internet.MemoryStreamConfig, ctx context.Context, dest net.Destination, raw net.Conn, hooks *SecurityHooks) (net.Conn, error) {
	if mss != nil && mss.TcpmaskManager != nil {
		newConn, err := mss.TcpmaskManager.WrapConnClient(raw)
		if err != nil {
			raw.Close()
			return nil, errors.New("mask err").Base(err)
		}
		raw = newConn
	}
	return applySecurity(ctx, dest, mss, raw, hooks)
}

// applySecurity layers TLS or REALITY on top of the given (already masked)
// connection according to hooks. A nil hooks means the connection stays
// mask-only: the caller layers its own security elsewhere.
func applySecurity(ctx context.Context, dest net.Destination, mss *internet.MemoryStreamConfig, conn net.Conn, hooks *SecurityHooks) (net.Conn, error) {
	if hooks == nil {
		return conn, nil
	}
	tlsConfig := tls.ConfigFromStreamSettings(mss)
	var realityConfig *reality.Config
	if hooks.WithReality {
		realityConfig = reality.ConfigFromStreamSettings(mss)
	}
	if tlsConfig != nil {
		return applyTLSClient(ctx, dest, conn, tlsConfig, *hooks)
	}
	if realityConfig != nil {
		return reality.UClient(conn, realityConfig, ctx, dest)
	}
	return conn, nil
}

// applyTLSClient builds the effective go-tls config (including the freedom
// MITM overrides driven by the session context), applies the uTLS/go-tls
// branch per hooks, and runs the eager handshake when requested.
func applyTLSClient(ctx context.Context, dest net.Destination, conn net.Conn, config *tls.Config, hooks SecurityHooks) (net.Conn, error) {
	mitmServerName := session.MitmServerNameFromContext(ctx)
	mitmAlpn11 := session.MitmAlpn11FromContext(ctx)
	var tlsConfig *gotls.Config
	if tls.IsFromMitm(config.ServerName) {
		tlsConfig = config.GetTLSConfig(tls.WithOverrideName(mitmServerName))
	} else {
		opts := []tls.Option{tls.WithDestination(dest)}
		if hooks.RequestALPN != "" {
			opts = append(opts, tls.WithNextProto(hooks.RequestALPN))
		}
		tlsConfig = config.GetTLSConfig(opts...)
	}

	isFromMitmVerify := false
	if r, ok := tlsConfig.Rand.(*tls.RandCarrier); ok && len(r.VerifyPeerCertByName) > 0 {
		for i, name := range r.VerifyPeerCertByName {
			if tls.IsFromMitm(name) {
				isFromMitmVerify = true
				r.VerifyPeerCertByName[0], r.VerifyPeerCertByName[i] = r.VerifyPeerCertByName[i], r.VerifyPeerCertByName[0]
				r.VerifyPeerCertByName = r.VerifyPeerCertByName[1:]
				after := mitmServerName
				for {
					if len(after) > 0 {
						r.VerifyPeerCertByName = append(r.VerifyPeerCertByName, after)
					}
					_, after, _ = strings.Cut(after, ".")
					if !strings.Contains(after, ".") {
						break
					}
				}
				slices.Reverse(r.VerifyPeerCertByName)
				break
			}
		}
	}
	isFromMitmAlpn := len(tlsConfig.NextProtos) == 1 && tls.IsFromMitm(tlsConfig.NextProtos[0])
	if isFromMitmAlpn {
		if mitmAlpn11 {
			tlsConfig.NextProtos[0] = "http/1.1"
		} else {
			tlsConfig.NextProtos = []string{"h2", "http/1.1"}
		}
	}
	var handshakeErr error
	if fingerprint := tls.GetFingerprint(config.Fingerprint); fingerprint != nil {
		conn = tls.UClient(conn, tlsConfig, fingerprint)
		switch hooks.UTLSHandshake {
		case HandshakeWebsocket:
			handshakeErr = conn.(*tls.UConn).WebsocketHandshakeContext(ctx)
		case HandshakeAuto:
			if len(tlsConfig.NextProtos) == 1 && tlsConfig.NextProtos[0] == "http/1.1" { // allow manually specify
				handshakeErr = conn.(*tls.UConn).WebsocketHandshakeContext(ctx)
			} else {
				handshakeErr = conn.(*tls.UConn).HandshakeContext(ctx)
			}
		case HandshakeTLS:
			handshakeErr = conn.(*tls.UConn).HandshakeContext(ctx)
		}
		if handshakeErr == nil && hooks.PostHandshake != nil {
			handshakeErr = hooks.PostHandshake(conn.(*tls.UConn))
		}
	} else {
		conn = tls.Client(conn, tlsConfig)
		if hooks.PlainTLSHandshake {
			handshakeErr = conn.(*tls.Conn).HandshakeContext(ctx)
		}
	}
	if handshakeErr != nil {
		if isFromMitmVerify {
			return nil, errors.New("MITM freedom RAW TLS: failed to verify Domain Fronting certificate from " + mitmServerName).Base(handshakeErr).AtWarning()
		}
		return nil, handshakeErr
	}
	if isFromMitmAlpn && !mitmAlpn11 { // MITM requires h2 unless http/1.1 was forced
		negotiatedProtocol := conn.(tls.Interface).NegotiatedProtocol()
		if negotiatedProtocol != "h2" {
			conn.Close()
			return nil, errors.New("MITM freedom RAW TLS: unexpected Negotiated Protocol (" + negotiatedProtocol + ") with " + mitmServerName).AtWarning()
		}
	}
	return conn, nil
}
