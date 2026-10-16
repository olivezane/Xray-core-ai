package tun

import (
	"errors"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xtls/xray-core/common/buf"
	xnet "github.com/xtls/xray-core/common/net"
)

// BenchmarkUDPNewSession measures what one new UDP flow costs the
// inbound while it stays open: QUIC and DNS open many short flows, and each
// one lives until the connection idle timeout.
func BenchmarkUDPNewSession(b *testing.B) {
	u := newUdpConnectionHandler(func(conn xnet.Conn, dest xnet.Destination) {}, nil)
	dst := xnet.UDPDestination(xnet.ParseAddress("1.1.1.1"), 443)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		src := xnet.UDPDestination(xnet.IPAddress([]byte{10, byte(i >> 16), byte(i >> 8), byte(i)}), xnet.Port(1024+i%60000))
		u.HandlePacket(src, dst, &buf.Buffer{})
	}
}

func TestPacketQueueOrderAndClose(t *testing.T) {
	q := newPacketQueue(udpQueueLimit)
	for i := 0; i < 3; i++ {
		b := buf.New()
		b.WriteByte(byte(i))
		if !q.push(b) {
			t.Fatalf("push %d rejected", i)
		}
	}
	for i := 0; i < 3; i++ {
		b, ok := q.pop()
		if !ok || b.Byte(0) != byte(i) {
			t.Fatalf("pop %d: got %v, %v", i, b, ok)
		}
		b.Release()
	}
	q.close()
	if _, ok := q.pop(); ok {
		t.Fatal("pop after close returned a packet")
	}
	if q.push(&buf.Buffer{}) {
		t.Fatal("push after close accepted")
	}
}

func TestPacketQueueLimit(t *testing.T) {
	q := newPacketQueue(udpQueueLimit)
	for i := 0; i < udpQueueLimit; i++ {
		if !q.push(&buf.Buffer{}) {
			t.Fatalf("push %d rejected below the limit", i)
		}
	}
	if q.push(&buf.Buffer{}) {
		t.Fatal("push above the limit accepted")
	}
}

func TestPacketQueueCloseUnblocksReader(t *testing.T) {
	q := newPacketQueue(udpQueueLimit)
	done := make(chan bool)
	go func() {
		_, ok := q.pop()
		done <- ok
	}()
	q.close()
	select {
	case ok := <-done:
		if ok {
			t.Fatal("blocked pop returned a packet after close")
		}
	case <-time.After(time.Second):
		t.Fatal("close did not wake the reader")
	}
}

func TestPacketQueueDropsDrainedStorage(t *testing.T) {
	q := newPacketQueue(udpQueueLimit)
	for i := 0; i < 100; i++ {
		q.push(&buf.Buffer{})
	}
	for i := 0; i < 100; i++ {
		q.pop()
	}
	if q.items != nil {
		t.Fatalf("drained queue still holds %d slots", cap(q.items))
	}
}

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
	handler.HandlePacket(client, serverA, buf.FromBytes([]byte("ping-a")))
	conn := <-connCh
	if atomic.LoadInt32(dials) != 1 {
		t.Fatalf("expected exactly 1 dial after the first packet, got %d", atomic.LoadInt32(dials))
	}

	// FullCone: a packet from the same source to a different server reuses
	// the same connection instead of dialing again
	handler.HandlePacket(client, serverB, buf.FromBytes([]byte("ping-b")))
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
	handler.HandlePacket(client, serverA, buf.FromBytes([]byte("ping")))
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
	handler.HandlePacket(client, serverA, buf.FromBytes([]byte("first")))
	conn := <-connCh

	// drain the buffered packet, then close: the NAT binding is freed
	p := make([]byte, 16)
	if n, err := conn.Read(p); err != nil || string(p[:n]) != "first" {
		t.Fatalf("expected 'first', got %q (err=%v)", p[:n], err)
	}
	conn.Close()
	if _, err := conn.Read(make([]byte, 4)); !errors.Is(err, io.EOF) {
		t.Fatalf("expected io.EOF after Close, got %v", err)
	}

	// a new packet from the same source dials a fresh connection
	handler.HandlePacket(client, serverA, buf.FromBytes([]byte("second")))
	conn2 := <-connCh
	if atomic.LoadInt32(dials) != 2 {
		t.Fatalf("expected a new dial after Close, got %d", atomic.LoadInt32(dials))
	}

	n, err := conn2.Read(p)
	if err != nil || string(p[:n]) != "second" {
		t.Fatalf("expected 'second' on the fresh connection, got %q (err=%v)", p[:n], err)
	}
}
