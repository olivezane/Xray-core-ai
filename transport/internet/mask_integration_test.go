package internet_test

import (
	"context"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	cnet "github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/protocol/tls/cert"
	protocoludp "github.com/xtls/xray-core/common/protocol/udp"
	"github.com/xtls/xray-core/common/serial"
	"github.com/xtls/xray-core/transport/internet"
	"github.com/xtls/xray-core/transport/internet/finalmask/header/custom"
	"github.com/xtls/xray-core/transport/internet/finalmask/salamander"
	"github.com/xtls/xray-core/transport/internet/security"
	"github.com/xtls/xray-core/transport/internet/stat"
	_ "github.com/xtls/xray-core/transport/internet/tcp" // register the tcp transport dialer/listener
	tlsx "github.com/xtls/xray-core/transport/internet/tls"
	"github.com/xtls/xray-core/transport/internet/udp"
)

// TestMaskTCPTransportProxyProtocol drives a real mask through the real tcp
// transport into the PROXY protocol: the server listener consumes the PROXY
// header at the socket level (below the mask, REQUIRE policy — a missing or
// malformed header drops the connection), then the mask unwraps the stream.
// The client writes the header on the raw dialed conn, then wraps with the
// builder — exactly the first half of tcp.Dial.
func TestMaskTCPTransportProxyProtocol(t *testing.T) {
	mss := tcpMaskMSS(t, nil)
	mss.SocketSettings = &internet.SocketConfig{AcceptProxyProtocol: true}

	serverDone := make(chan error, 1)
	ln, err := internet.ListenTCP(context.Background(), cnet.LocalHostIP, cnet.Port(freeTCPPort(t)), mss, func(conn stat.Connection) {
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
		req := make([]byte, 5)
		if _, err := io.ReadFull(conn, req); err != nil {
			serverDone <- err
			return
		}
		if string(req) != "hello" {
			serverDone <- fmt.Errorf("unexpected payload: %q", req)
			return
		}
		_, err := conn.Write([]byte("world"))
		serverDone <- err
	})
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	dest := cnet.TCPDestination(cnet.LocalHostIP, cnet.Port(ln.Addr().(*net.TCPAddr).Port))
	raw, err := internet.DialSystem(context.Background(), dest, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	// PROXY protocol v1 header, written below the mask exactly as the wire
	// order requires.
	if _, err := raw.Write([]byte("PROXY TCP4 192.168.1.1 192.168.1.2 56324 443\r\n")); err != nil {
		t.Fatal(err)
	}

	conn, err := security.WrapConnClient(mss, context.Background(), dest, raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))

	if _, err := conn.Write([]byte("hello")); err != nil {
		t.Fatalf("client write: %v", err)
	}
	response := make([]byte, 5)
	if _, err := io.ReadFull(conn, response); err != nil {
		t.Fatalf("client read: %v", err)
	}
	if string(response) != "world" {
		t.Fatalf("unexpected response: %q", response)
	}

	select {
	case err := <-serverDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("server side did not complete")
	}
}

// TestMaskUDPTransport drives a real UDP mask through the real udp transport:
// internet.Dial (udp dialer, mask wraps the dialed PacketConn) and
// udp.ListenUDP (mask wraps the listening PacketConn).
func TestMaskUDPTransport(t *testing.T) {
	mss := udpMaskMSS(t, serial.ToTypedMessage(&salamander.Config{Password: "test-password"}))

	hub, err := udp.ListenUDP(context.Background(), cnet.LocalHostIP, cnet.Port(0), mss)
	if err != nil {
		t.Fatal(err)
	}
	defer hub.Close()
	serverAddr := hub.Addr().(*net.UDPAddr)

	conn, err := internet.Dial(context.Background(), cnet.UDPDestination(cnet.IPAddress(serverAddr.IP), cnet.Port(serverAddr.Port)), mss)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	if _, err := conn.Write([]byte("ping")); err != nil {
		t.Fatalf("client write: %v", err)
	}

	var pkt *protocoludp.Packet
	select {
	case pkt = <-hub.Receive():
	case <-time.After(5 * time.Second):
		t.Fatal("no packet received at the udp hub")
	}
	if string(pkt.Payload.Bytes()) != "ping" {
		t.Fatalf("unexpected payload: %q", pkt.Payload.Bytes())
	}
	if _, err := hub.WriteTo([]byte("pong"), pkt.Source); err != nil {
		t.Fatal(err)
	}
	pkt.Payload.Release()

	response := make([]byte, 4)
	if _, err := io.ReadFull(conn, response); err != nil {
		t.Fatalf("client read: %v", err)
	}
	if string(response) != "pong" {
		t.Fatalf("unexpected response: %q", response)
	}
}

// TestMaskBeforeTLSOrderThroughTCPTransport proves the mask-before-TLS order
// end-to-end: the TLS handshake runs inside the masked stream, so the order is
// enforced by the transport actually working (a TLS-first wrap would fail the
// handshake). Client and server go through the real tcp transport with the
// same mask + TLS settings.
func TestMaskBeforeTLSOrderThroughTCPTransport(t *testing.T) {
	ct, fingerprint := cert.MustGenerate(nil, cert.DNSNames("localhost"))
	serverTLS := &tlsx.Config{Certificate: []*tlsx.Certificate{tlsx.ParseCertificate(ct)}}
	clientTLS := &tlsx.Config{ServerName: "localhost", PinnedPeerCertSha256: [][]byte{fingerprint[:]}}

	serverMSS := tcpMaskMSS(t, serverTLS)
	clientMSS := tcpMaskMSS(t, clientTLS)

	serverDone := make(chan error, 1)
	ln, err := internet.ListenTCP(context.Background(), cnet.LocalHostIP, cnet.Port(freeTCPPort(t)), serverMSS, func(conn stat.Connection) {
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
		req := make([]byte, len("hello tls"))
		if _, err := io.ReadFull(conn, req); err != nil {
			serverDone <- err
			return
		}
		if string(req) != "hello tls" {
			serverDone <- fmt.Errorf("unexpected payload: %q", req)
			return
		}
		_, err := conn.Write([]byte("world tls"))
		serverDone <- err
	})
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	conn, err := internet.Dial(context.Background(), cnet.TCPDestination(cnet.LocalHostIP, cnet.Port(ln.Addr().(*net.TCPAddr).Port)), clientMSS)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))

	if _, err := conn.Write([]byte("hello tls")); err != nil {
		t.Fatalf("client write: %v", err)
	}
	response := make([]byte, len("world tls"))
	if _, err := io.ReadFull(conn, response); err != nil {
		t.Fatalf("client read: %v", err)
	}
	if string(response) != "world tls" {
		t.Fatalf("unexpected response: %q", response)
	}

	select {
	case err := <-serverDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("server side did not complete")
	}
}

// tcpMaskMSS builds a MemoryStreamConfig for the tcp transport with a
// deterministic custom header mask and optional TLS security settings.
func tcpMaskMSS(t *testing.T, tlsConfig *tlsx.Config) *internet.MemoryStreamConfig {
	t.Helper()
	seq := []*custom.TCPSequence{{Sequence: []*custom.TCPItem{{Packet: []byte{0xAA}}, {Rand: 1}}}}
	mask := &custom.TCPConfig{Clients: seq, Servers: seq}

	sc := &internet.StreamConfig{
		ProtocolName: "tcp",
		Tcpmasks:     []*serial.TypedMessage{serial.ToTypedMessage(mask)},
	}
	if tlsConfig != nil {
		tm := serial.ToTypedMessage(tlsConfig)
		sc.SecurityType = tm.Type
		sc.SecuritySettings = []*serial.TypedMessage{tm}
	}
	mss, err := internet.ToMemoryStreamConfig(sc)
	if err != nil {
		t.Fatal(err)
	}
	return mss
}

// udpMaskMSS builds a MemoryStreamConfig for the udp transport with the given
// UDP mask message.
func udpMaskMSS(t *testing.T, mask *serial.TypedMessage) *internet.MemoryStreamConfig {
	t.Helper()
	sc := &internet.StreamConfig{
		ProtocolName: "udp",
		Udpmasks:     []*serial.TypedMessage{mask},
	}
	mss, err := internet.ToMemoryStreamConfig(sc)
	if err != nil {
		t.Fatal(err)
	}
	return mss
}

func freeTCPPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}
