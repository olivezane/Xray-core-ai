package security

import (
	gotls "crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"io"
	"net"
	"testing"
	"time"

	"github.com/xtls/xray-core/common/protocol/tls/cert"
	"github.com/xtls/xray-core/transport/internet"
	"github.com/xtls/xray-core/transport/internet/reality"
	xraytls "github.com/xtls/xray-core/transport/internet/tls"
)

// serverCert builds a self-signed cert and the matching go-tls server config.
func serverCert(t *testing.T) (*gotls.Config, []byte) {
	t.Helper()
	crt, hash := cert.MustGenerate(nil, cert.CommonName("localhost"), cert.DNSNames("localhost"))
	keyParsed, err := x509.ParsePKCS8PrivateKey(crt.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &gotls.Config{Certificates: []gotls.Certificate{{Certificate: [][]byte{crt.Certificate}, PrivateKey: keyParsed}}}
	return cfg, hash[:]
}

// tlsSettings builds stream settings whose security config carries a real
// self-signed cert, exactly as a parsed JSON config would.
func tlsSettings(t *testing.T) *internet.MemoryStreamConfig {
	t.Helper()
	crt, _ := cert.MustGenerate(nil, cert.CommonName("localhost"), cert.DNSNames("localhost"))
	return &internet.MemoryStreamConfig{
		SecuritySettings: &xraytls.Config{
			ServerName: "localhost",
			Certificate: []*xraytls.Certificate{{
				Certificate: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: crt.Certificate}),
				Key:         pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: crt.PrivateKey}),
			}},
		},
	}
}

func TestResolveServerSecurityCaps(t *testing.T) {
	realitySettings := func() *internet.MemoryStreamConfig {
		return &internet.MemoryStreamConfig{SecuritySettings: &reality.Config{PublicKey: []byte{1, 2, 3}}}
	}

	for _, tc := range []struct {
		name     string
		caps     ServerCaps
		mss      *internet.MemoryStreamConfig
		wantTLS  bool
		wantReal bool
	}{
		{name: "none-requested", caps: ServerCaps{}, mss: tlsSettings(t)},
		{name: "tls-only", caps: ServerCaps{WithTLS: true}, mss: tlsSettings(t), wantTLS: true},
		{name: "reality-only", caps: ServerCaps{WithReality: true}, mss: realitySettings(), wantReal: true},
		{name: "cap-gates-present-config", caps: ServerCaps{WithTLS: true}, mss: realitySettings()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sec := ResolveServerSecurity(tc.mss, tc.caps)
			if got := sec.TLS != nil; got != tc.wantTLS {
				t.Fatalf("TLS resolved = %v, want %v", got, tc.wantTLS)
			}
			if got := sec.Reality != nil; got != tc.wantReal {
				t.Fatalf("REALITY resolved = %v, want %v", got, tc.wantReal)
			}
		})
	}
}

// TestWrapConnServerForms pins the connection-shaped contract used by the
// RAW-TCP hub and the KCP listener: xray Conn wrappers, TLS precedence over
// REALITY, and passthrough when nothing resolves.
func TestWrapConnServerForms(t *testing.T) {
	t.Run("passthrough", func(t *testing.T) {
		client, server := net.Pipe()
		defer client.Close()
		defer server.Close()

		conn, err := WrapConnServer(ServerSecurity{}, server)
		if err != nil {
			t.Fatal(err)
		}
		if conn != net.Conn(server) {
			t.Fatalf("empty security must pass the conn through, got %T", conn)
		}
	})

	t.Run("tls-precedence", func(t *testing.T) {
		tlsCfg, _ := serverCert(t)
		sec := ServerSecurity{TLS: tlsCfg}
		client, server := net.Pipe()
		defer client.Close()
		defer server.Close()

		conn, err := WrapConnServer(sec, server)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		if _, ok := conn.(*xraytls.Conn); !ok {
			t.Fatalf("resolved TLS must win the per-conn form, got %T", conn)
		}
	})

	t.Run("tls-echo-loopback", func(t *testing.T) {
		tlsCfg, _ := serverCert(t)
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer ln.Close()

		go func() {
			raw, err := ln.Accept()
			if err != nil {
				return
			}
			secured, err := WrapConnServer(ServerSecurity{TLS: tlsCfg}, raw)
			if err != nil {
				return
			}
			_ = secured.SetDeadline(time.Now().Add(5 * time.Second))
			_, _ = io.Copy(secured, secured) // echo
			_ = secured.Close()
		}()

		rawClient, err := net.Dial("tcp", ln.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		defer rawClient.Close()
		client := gotls.Client(rawClient, &gotls.Config{InsecureSkipVerify: true, ServerName: "localhost"})
		_ = client.SetDeadline(time.Now().Add(5 * time.Second))
		if err := client.Handshake(); err != nil {
			t.Fatal(err)
		}
		msg := []byte("ping")
		if _, err := client.Write(msg); err != nil {
			t.Fatal(err)
		}
		buf := make([]byte, len(msg))
		if _, err := io.ReadFull(client, buf); err != nil {
			t.Fatal(err)
		}
	})
}

// TestSecureListenerOverMaskedListener is the listener-shaped counterpart of
// the client seam guard: the mask chain (applied by the caller through
// internet.WrapListener) must stay underneath the security layer, and the
// composed listener must carry a real TLS handshake end to end.
func TestSecureListenerOverMaskedListener(t *testing.T) {

	var order []string
	mss := tlsSettings(t)
	mss.TcpmaskManager = internet.NewTcpmaskManager([]internet.Tcpmask{&recordingMask{rec: &order}})

	rawLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer rawLn.Close()

	maskedLn, err := internet.WrapListener(mss, rawLn)
	if err != nil {
		t.Fatal(err)
	}
	secureLn := WrapSecureListener(ResolveServerSecurity(mss, ServerCaps{WithTLS: true}), maskedLn)

	go func() {
		conn, err := secureLn.Accept()
		if err != nil {
			return
		}
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		_, _ = io.Copy(conn, conn) // echo through mask + TLS
		_ = conn.Close()
	}()

	rawClient, err := net.Dial("tcp", rawLn.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer rawClient.Close()
	client := gotls.Client(rawClient, &gotls.Config{InsecureSkipVerify: true, ServerName: "localhost"})
	_ = client.SetDeadline(time.Now().Add(5 * time.Second))
	if err := client.Handshake(); err != nil {
		t.Fatal(err)
	}

	if len(order) != 1 || order[0] != "mask" {
		t.Fatalf("wrap order = %v, want [mask] recorded beneath the TLS layer", order)
	}

	msg := []byte("ping")
	if _, err := client.Write(msg); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, len(msg))
	if _, err := io.ReadFull(client, buf); err != nil {
		t.Fatal(err)
	}
}
