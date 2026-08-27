package security

import (
	"context"
	gotls "crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	cnet "github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/protocol/tls/cert"
	"github.com/xtls/xray-core/transport/internet"
	"github.com/xtls/xray-core/transport/internet/reality"
	xraytls "github.com/xtls/xray-core/transport/internet/tls"
)

// recordingMask records each client-side wrap into a shared order slice.
type recordingMask struct {
	rec *[]string
}

func (m *recordingMask) TCP() {}

func (m *recordingMask) WrapConnClient(raw net.Conn) (net.Conn, error) {
	*m.rec = append(*m.rec, "mask")
	return raw, nil
}

func (m *recordingMask) WrapConnServer(raw net.Conn) (net.Conn, error) {
	*m.rec = append(*m.rec, "mask")
	return raw, nil
}

// failingWrap fails every client-side wrap and tracks Close on the raw conn.
type failingWrap struct{}

func (failingWrap) TCP() {}

func (failingWrap) WrapConnClient(raw net.Conn) (net.Conn, error) { return nil, io.ErrClosedPipe }
func (failingWrap) WrapConnServer(raw net.Conn) (net.Conn, error) { return raw, nil }

type closeTrackingConn struct {
	net.Conn
	closed bool
}

func (c *closeTrackingConn) Close() error {
	c.closed = true
	return c.Conn.Close()
}

// loopbackTLS starts a go-tls echo server on 127.0.0.1 and returns its TCP
// port, the pinned leaf cert hash, and a shutdown func.
func loopbackTLS(t *testing.T, nextProtos []string) (port cnet.Port, pin []byte, shutdown func()) {
	t.Helper()
	crt, hash := cert.MustGenerate(nil, cert.CommonName("localhost"), cert.DNSNames("localhost"))
	keyParsed, err := x509.ParsePKCS8PrivateKey(crt.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	serverTLS := &gotls.Config{
		Certificates: []gotls.Certificate{{Certificate: [][]byte{crt.Certificate}, PrivateKey: keyParsed}},
		NextProtos:   nextProtos,
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			raw, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				srv := gotls.Server(raw, serverTLS)
				_ = srv.SetDeadline(time.Now().Add(5 * time.Second))
				if srv.Handshake() == nil {
					_, _ = io.Copy(srv, srv) // echo
				}
				_ = srv.Close()
			}()
		}
	}()
	return cnet.Port(ln.Addr().(*net.TCPAddr).Port), hash[:], func() { ln.Close(); <-done }
}

// TestWrapConnClientMaskBeforeTLSEager guards the core invariant of this
// package through its public entry: the mask chain wraps BEFORE any TLS
// handshake runs, regardless of which eager style the transport requests.
// The red proof is structural: applySecurity running before the mask chain,
// or the mask chain being dropped, reorders/shrinks the recorded sequence.
func TestWrapConnClientMaskBeforeTLSEager(t *testing.T) {
	for _, tc := range []struct {
		name    string
		hooks   SecurityHooks
		alpn    []string // server ALPN
		reqALPN string
	}{
		{name: "plain-eager", hooks: SecurityHooks{PlainTLSHandshake: true}},
		{name: "utls-auto-forced-http1", hooks: SecurityHooks{UTLSHandshake: HandshakeAuto}, alpn: []string{"http/1.1"}, reqALPN: "http/1.1"},
		{name: "utls-tls", hooks: SecurityHooks{UTLSHandshake: HandshakeTLS}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			port, pin, shutdown := loopbackTLS(t, tc.alpn)
			defer shutdown()

			var order []string
			mss := &internet.MemoryStreamConfig{
				TcpmaskManager:   internet.NewTcpmaskManager([]internet.Tcpmask{&recordingMask{rec: &order}}),
				SecuritySettings: &xraytls.Config{ServerName: "localhost", PinnedPeerCertSha256: [][]byte{pin}},
			}
			dest := cnet.TCPDestination(cnet.LocalHostIP, port)

			raw, err := net.Dial("tcp", "127.0.0.1:"+port.String())
			if err != nil {
				t.Fatal(err)
			}
			defer raw.Close()

			hooks := tc.hooks
			hooks.RequestALPN = tc.reqALPN
			hooks.PostHandshake = func(*xraytls.UConn) error {
				order = append(order, "tls-handshake")
				return nil
			}
			conn, err := WrapConnClient(mss, context.Background(), dest, raw, &hooks)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

			if len(order) != 2 || order[0] != "mask" || order[1] != "tls-handshake" {
				t.Fatalf("wrap order = %v, want [mask tls-handshake]", order)
			}

			// Echo roundtrip proves the stacked conn is functional end to end.
			msg := []byte("ping")
			if _, err := conn.Write(msg); err != nil {
				t.Fatal(err)
			}
			buf := make([]byte, len(msg))
			if _, err := io.ReadFull(conn, buf); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// TestWrapConnClientNilHooksStaysMaskOnly pins the nil-hooks contract that
// websocket's gorilla-managed TLS relies on: security settings alone must
// not wrap anything without hooks asking for it.
func TestWrapConnClientNilHooksStaysMaskOnly(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	var order []string
	mss := &internet.MemoryStreamConfig{
		SecuritySettings: &xraytls.Config{ServerName: "localhost"},
		TcpmaskManager:   internet.NewTcpmaskManager([]internet.Tcpmask{&recordingMask{rec: &order}}),
	}
	conn, err := WrapConnClient(mss, context.Background(), cnet.TCPDestination(cnet.LocalHostIP, 443), client, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if conn != net.Conn(client) {
		t.Fatalf("nil hooks must return the masked raw conn unchanged, got %T", conn)
	}
	if len(order) != 1 || order[0] != "mask" {
		t.Fatalf("wrap order = %v, want [mask]", order)
	}
}

// TestWrapConnClientLazyDefersHandshake: net.Pipe is synchronous, so an
// eager handshake would deadlock with no reader on the far end. Returning
// at all proves the handshake was deferred to first use.
func TestWrapConnClientLazyDefersHandshake(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	mss := &internet.MemoryStreamConfig{
		SecuritySettings: &xraytls.Config{ServerName: "localhost"},
	}
	conn, err := WrapConnClient(mss, context.Background(), cnet.TCPDestination(cnet.LocalHostIP, 443), client, &SecurityHooks{})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	switch conn.(type) {
	case *xraytls.Conn, *xraytls.UConn:
		// wrapped for lazy TLS, no handshake attempted
	default:
		t.Fatalf("lazy path = %T, want a TLS-wrapped conn", conn)
	}
}

// TestWrapConnClientMaskErrorPropagatesWithHooks: even when the caller
// asks for security, a failing mask wrap must short-circuit with its own
// error (and close the raw conn) instead of reaching TLS.
func TestWrapConnClientMaskErrorPropagatesWithHooks(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()

	mss := &internet.MemoryStreamConfig{
		TcpmaskManager:   internet.NewTcpmaskManager([]internet.Tcpmask{failingWrap{}}),
		SecuritySettings: &xraytls.Config{ServerName: "localhost"},
	}
	raw := &closeTrackingConn{Conn: client}
	_, err := WrapConnClient(mss, context.Background(), cnet.TCPDestination(cnet.LocalHostIP, 443), raw, &SecurityHooks{UTLSHandshake: HandshakeTLS})
	if err == nil || !strings.Contains(err.Error(), "mask err") {
		t.Fatalf("err = %v, want mask err", err)
	}
	if !raw.closed {
		t.Fatal("raw conn was not closed on mask error")
	}
}

// TestWrapConnClientRealityRouting proves the REALITY branch is reachable
// when (and only when) hooks opt in: an invalid public key makes
// reality.UClient fail loudly instead of silently passing plaintext.
func TestWrapConnClientRealityRouting(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	mss := &internet.MemoryStreamConfig{
		SecuritySettings: &reality.Config{PublicKey: []byte{1, 2, 3}},
	}
	_, err := WrapConnClient(mss, context.Background(), cnet.TCPDestination(cnet.LocalHostIP, 443), client, &SecurityHooks{WithReality: true})
	if err == nil || !strings.Contains(err.Error(), "REALITY") {
		t.Fatalf("expected REALITY branch error, got %v", err)
	}
}
