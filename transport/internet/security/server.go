package security

import (
	gotls "crypto/tls"
	"net"

	goreality "github.com/xtls/reality"
	"github.com/xtls/xray-core/transport/internet"
	"github.com/xtls/xray-core/transport/internet/reality"
	"github.com/xtls/xray-core/transport/internet/tls"
)

// ServerCaps declares which securities a transport's server side supports.
// They keep the historical capability matrix explicit instead of silently
// growing it: kcp/websocket/httpupgrade never had REALITY listeners, and
// gRPC has no TLS listener.
type ServerCaps struct {
	WithTLS     bool
	WithReality bool
}

// ServerSecurity holds the server-side engines, resolved once per listener.
type ServerSecurity struct {
	TLS     *gotls.Config     // crypto/tls engine, nil when absent
	Reality *goreality.Config // REALITY engine, nil when absent
}

// ResolveServerSecurity extracts the configured engines from stream
// settings, gated by the transport's declared capabilities. The two engines
// are resolved independently: connection-shaped sites pick between them,
// listener-shaped sites stack them.
func ResolveServerSecurity(mss *internet.MemoryStreamConfig, caps ServerCaps) ServerSecurity {
	var sec ServerSecurity
	if caps.WithTLS {
		if config := tls.ConfigFromStreamSettings(mss); config != nil {
			sec.TLS = config.GetTLSConfig()
		}
	}
	if caps.WithReality {
		if config := reality.ConfigFromStreamSettings(mss); config != nil {
			sec.Reality = config.GetREALITYConfig()
		}
	}
	return sec
}

// WrapConnServer layers server security onto a single accepted connection,
// using the xray Conn wrappers (Close-timeout semantics). TLS takes
// precedence; REALITY fails eagerly with an error, matching RAW-TCP hub
// behaviour where the connection is dropped on handshake failure.
// Connection-shaped sites (tcp hub, kcp listener) use this form.
func WrapConnServer(sec ServerSecurity, conn net.Conn) (net.Conn, error) {
	if sec.TLS != nil {
		return tls.Server(conn, sec.TLS), nil
	}
	if sec.Reality != nil {
		return reality.Server(conn, sec.Reality)
	}
	return conn, nil
}

// WrapSecureListener wraps a listener so every accepted connection is
// secured with the stdlib-family wrappers (lazy crypto/tls conns, the
// upstream REALITY listener). Listener-shaped sites (websocket,
// httpupgrade, splithttp, gRPC) use this form; when both engines are
// present they stack, mirroring the historical splithttp hub.
func WrapSecureListener(sec ServerSecurity, l net.Listener) net.Listener {
	if sec.TLS != nil {
		l = gotls.NewListener(l, sec.TLS)
	}
	if sec.Reality != nil {
		l = goreality.NewListener(l, sec.Reality)
	}
	return l
}
