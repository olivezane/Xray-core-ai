package internet_test

import (
	"errors"
	"io"
	"net"
	"strings"
	"testing"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/net/cnc"
	"github.com/xtls/xray-core/transport/internet"
	"github.com/xtls/xray-core/transport/pipe"
)

// orderMaskConn is the wrapper type produced by orderMask, used to prove that
// the mask ran before the TLS wrap.
type orderMaskConn struct {
	net.Conn
}

type orderMaskPacketConn struct {
	net.PacketConn
}

type orderMask struct{}

func (orderMask) TCP()                                          {}
func (orderMask) WrapConnClient(raw net.Conn) (net.Conn, error) { return &orderMaskConn{raw}, nil }
func (orderMask) WrapConnServer(raw net.Conn) (net.Conn, error) { return raw, nil }
func (orderMask) UDP()                                          {}
func (orderMask) WrapPacketConnClient(raw net.PacketConn, _, _ int) (net.PacketConn, error) {
	return &orderMaskPacketConn{raw}, nil
}
func (orderMask) WrapPacketConnServer(raw net.PacketConn, _, _ int) (net.PacketConn, error) {
	return raw, nil
}

type failingMask struct{}

func (failingMask) TCP()                                          {}
func (failingMask) WrapConnClient(raw net.Conn) (net.Conn, error) { return nil, io.ErrClosedPipe }
func (failingMask) WrapConnServer(raw net.Conn) (net.Conn, error) { return raw, nil }
func (failingMask) UDP()                                          {}
func (failingMask) WrapPacketConnClient(raw net.PacketConn, _, _ int) (net.PacketConn, error) {
	return nil, io.ErrClosedPipe
}
func (failingMask) WrapPacketConnServer(raw net.PacketConn, _, _ int) (net.PacketConn, error) {
	return raw, nil
}

func maskStreamConfig(manager *internet.TcpmaskManager) *internet.MemoryStreamConfig {
	return &internet.MemoryStreamConfig{TcpmaskManager: manager}
}

func TestWrapConnClientMaskBeforeTLS(t *testing.T) {
	mss := maskStreamConfig(internet.NewTcpmaskManager([]internet.Tcpmask{orderMask{}}))
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	var tlsGot net.Conn
	result, err := internet.WrapConnClient(mss, client, func(conn net.Conn) (net.Conn, error) {
		tlsGot = conn
		return conn, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := tlsGot.(*orderMaskConn); !ok {
		t.Fatalf("TLS wrap got %T, want the masked conn", tlsGot)
	}
	if _, ok := result.(*orderMaskConn); !ok {
		t.Fatalf("result is %T, want the masked conn", result)
	}
}

func TestWrapConnClientNoMaskPassthrough(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	for _, mss := range []*internet.MemoryStreamConfig{nil, {}} {
		var tlsGot net.Conn
		result, err := internet.WrapConnClient(mss, client, func(conn net.Conn) (net.Conn, error) {
			tlsGot = conn
			return conn, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if result != client || tlsGot != client {
			t.Fatalf("mss=%v: expected raw conn passthrough, got %v/%v", mss, result, tlsGot)
		}
	}
}

func TestWrapConnClientMaskErrorClosesRaw(t *testing.T) {
	mss := maskStreamConfig(internet.NewTcpmaskManager([]internet.Tcpmask{failingMask{}}))
	client, server := net.Pipe()
	defer server.Close()

	raw := &closedTrackingConn{Conn: client}
	_, err := internet.WrapConnClient(mss, raw, nil)
	if err == nil || !errors.Is(err, io.ErrClosedPipe) || !strings.Contains(err.Error(), "mask err") {
		t.Fatalf("err = %v, want mask err wrapping io.ErrClosedPipe", err)
	}
	if !raw.closed {
		t.Fatal("raw conn was not closed on mask error")
	}
}

func TestWrapListenerNoMaskPassthrough(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	for _, mss := range []*internet.MemoryStreamConfig{nil, {}} {
		out, err := internet.WrapListener(mss, ln)
		if err != nil {
			t.Fatal(err)
		}
		if out != ln {
			t.Fatalf("mss=%v: expected listener passthrough", mss)
		}
	}
}

func TestUnwrapPacketConnTypes(t *testing.T) {
	// PacketConnWrapper
	udpAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1234}
	rawPC, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer rawPC.Close()
	wrapped := &internet.PacketConnWrapper{PacketConn: rawPC, Dest: udpAddr}
	pc, addr, err := internet.UnwrapPacketConn(wrapped)
	if err != nil {
		t.Fatal(err)
	}
	if pc != rawPC || addr.Port != 1234 {
		t.Fatalf("unwrapped %v/%v, want rawPC/%v", pc, addr, udpAddr)
	}

	// cnc.Connection (TCP-based fake UDP)
	_, uw := pipe.New()
	dr, dw := pipe.New()
	cc := cnc.NewConnection(
		cnc.ConnectionInputMulti(uw),
		cnc.ConnectionOutputMulti(dr),
		cnc.ConnectionOnClose(common.ChainedClosable{uw, dw}),
	)
	pc, addr, err = internet.UnwrapPacketConn(cc)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := pc.(*internet.FakePacketConn); !ok {
		t.Fatalf("cnc unwrap got %T, want *internet.FakePacketConn", pc)
	}
	if addr == nil {
		t.Fatal("cnc unwrap addr is nil, want *net.UDPAddr")
	}

	// unsupported type: error, not panic, and the conn is closed
	client, server := net.Pipe()
	defer server.Close()
	_, _, err = internet.UnwrapPacketConn(client)
	if err == nil {
		t.Fatal("expected error for unsupported connection type")
	}
}

func TestWrapPacketConnDial(t *testing.T) {
	// no mask: passthrough
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	out, err := internet.WrapPacketConnDial(nil, client)
	if err != nil || out != client {
		t.Fatalf("nil mss: got %v/%v", out, err)
	}

	// with mask: dialed conn is unwrapped, masked, and re-wrapped
	udpAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1234}
	rawPC, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer rawPC.Close()
	mss := &internet.MemoryStreamConfig{
		UdpmaskManager: internet.NewUdpmaskManager([]internet.Udpmask{orderMask{}}),
	}
	out, err = internet.WrapPacketConnDial(mss, &internet.PacketConnWrapper{PacketConn: rawPC, Dest: udpAddr})
	if err != nil {
		t.Fatal(err)
	}
	wrapper, ok := out.(*internet.PacketConnWrapper)
	if !ok {
		t.Fatalf("got %T, want *internet.PacketConnWrapper", out)
	}
	if _, ok := wrapper.PacketConn.(*orderMaskPacketConn); !ok {
		t.Fatalf("masked conn = %T, want *orderMaskPacketConn", wrapper.PacketConn)
	}
	if wrapper.Dest != udpAddr {
		t.Fatalf("dest = %v, want %v", wrapper.Dest, udpAddr)
	}
}

func TestWrapPacketConnClientServerNoMask(t *testing.T) {
	rawPC, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer rawPC.Close()
	for _, mss := range []*internet.MemoryStreamConfig{nil, {}} {
		out, err := internet.WrapPacketConnClient(mss, rawPC)
		if err != nil || out != rawPC {
			t.Fatalf("client mss=%v: got %v/%v", mss, out, err)
		}
		out, err = internet.WrapPacketConnServer(mss, rawPC)
		if err != nil || out != rawPC {
			t.Fatalf("server mss=%v: got %v/%v", mss, out, err)
		}
	}

	// mask error closes the raw conn
	mss := &internet.MemoryStreamConfig{
		UdpmaskManager: internet.NewUdpmaskManager([]internet.Udpmask{failingMask{}}),
	}
	_, err = internet.WrapPacketConnClient(mss, rawPC)
	if err == nil || !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("err = %v, want mask err", err)
	}
}

type closedTrackingConn struct {
	net.Conn
	closed bool
}

func (c *closedTrackingConn) Close() error {
	c.closed = true
	return c.Conn.Close()
}
