package tun

import (
	"io"
	"net"
	"sync/atomic"
	"testing"

	"github.com/xtls/xray-core/common/buf"
	xnet "github.com/xtls/xray-core/common/net"
)

func newUDPTestHandler(t *testing.T) (*udpConnectionHandler, chan *udpConn, *int32, *[]udpWrite) {
	t.Helper()
	connCh := make(chan *udpConn, 8)
	var dials int32
	var writes []udpWrite
	handler := newUdpConnectionHandler(
		func(conn net.Conn, _ xnet.Destination) {
			atomic.AddInt32(&dials, 1)
			connCh <- conn.(*udpConn)
		},
		func(data []byte, src, dst xnet.Destination) error {
			writes = append(writes, udpWrite{data: append([]byte(nil), data...), src: src, dst: dst})
			return nil
		},
	)
	return handler, connCh, &dials, &writes
}

type udpWrite struct {
	data []byte
	src  xnet.Destination
	dst  xnet.Destination
}

func TestUDPFullConeAllocationReuseAndEgress(t *testing.T) {
	client := xnet.UDPDestination(xnet.IPAddress([]byte{10, 0, 0, 1}), 12345)
	serverA := xnet.UDPDestination(xnet.IPAddress([]byte{1, 2, 3, 4}), 53)
	serverB := xnet.UDPDestination(xnet.IPAddress([]byte{5, 6, 7, 8}), 443)

	handler, connCh, dials, _ := newUDPTestHandler(t)

	// first packet from the client allocates a connection
	handler.HandlePacket(client, serverA, []byte("ping-a"))
	conn := <-connCh
	if atomic.LoadInt32(dials) != 1 {
		t.Fatalf("expected exactly 1 dial after the first packet, got %d", atomic.LoadInt32(dials))
	}

	// FullCone: a packet from the same source to a different server reuses
	// the same connection instead of dialing again
	handler.HandlePacket(client, serverB, []byte("ping-b"))
	if atomic.LoadInt32(dials) != 1 {
		t.Fatalf("expected no new dial for an existing source, got %d", atomic.LoadInt32(dials))
	}

	// egress packets are delivered in order on the same connection
	p := make([]byte, 16)
	n, err := conn.Read(p)
	if err != nil || string(p[:n]) != "ping-a" {
		t.Fatalf("expected 'ping-a', got %q (err=%v)", p[:n], err)
	}
	n, err = conn.Read(p)
	if err != nil || string(p[:n]) != "ping-b" {
		t.Fatalf("expected 'ping-b', got %q (err=%v)", p[:n], err)
	}
}

func TestUDPFullConeEgressWritesBackToSource(t *testing.T) {
	client := xnet.UDPDestination(xnet.IPAddress([]byte{10, 0, 0, 1}), 12345)
	serverA := xnet.UDPDestination(xnet.IPAddress([]byte{1, 2, 3, 4}), 53)

	handler, connCh, _, writes := newUDPTestHandler(t)
	handler.HandlePacket(client, serverA, []byte("ping"))
	conn := <-connCh

	// a reply packet addressed to the client goes back to the client's
	// original source address:port
	b := buf.FromBytes([]byte("pong"))
	b.UDP = &serverA
	if err := conn.WriteMultiBuffer(buf.MultiBuffer{b}); err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}

	if len(*writes) != 1 {
		t.Fatalf("expected 1 write back, got %d", len(*writes))
	}
	w := (*writes)[0]
	if string(w.data) != "pong" {
		t.Errorf("expected payload 'pong', got %q", w.data)
	}
	if w.src != serverA {
		t.Errorf("expected reply source %v, got %v", serverA, w.src)
	}
	if w.dst != client {
		t.Errorf("expected reply destination %v, got %v", client, w.dst)
	}
}

func TestUDPFullConeCloseFreesBinding(t *testing.T) {
	client := xnet.UDPDestination(xnet.IPAddress([]byte{10, 0, 0, 1}), 12345)
	serverA := xnet.UDPDestination(xnet.IPAddress([]byte{1, 2, 3, 4}), 53)

	handler, connCh, dials, _ := newUDPTestHandler(t)
	handler.HandlePacket(client, serverA, []byte("first"))
	conn := <-connCh

	// drain the buffered packet, then close: the NAT binding is freed
	p := make([]byte, 16)
	if n, err := conn.Read(p); err != nil || string(p[:n]) != "first" {
		t.Fatalf("expected 'first', got %q (err=%v)", p[:n], err)
	}
	_ = conn.Close()
	if _, err := conn.Read(make([]byte, 4)); err != io.EOF {
		t.Fatalf("expected io.EOF after Close, got %v", err)
	}

	// a new packet from the same source dials a fresh connection
	handler.HandlePacket(client, serverA, []byte("second"))
	conn2 := <-connCh
	if atomic.LoadInt32(dials) != 2 {
		t.Fatalf("expected a new dial after Close, got %d", atomic.LoadInt32(dials))
	}

	n, err := conn2.Read(p)
	if err != nil || string(p[:n]) != "second" {
		t.Fatalf("expected 'second' on the fresh connection, got %q (err=%v)", p[:n], err)
	}
}
