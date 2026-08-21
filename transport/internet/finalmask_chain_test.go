package internet

import (
	"net"
	"testing"
)

// recordingTcpMask records each wrap call's name into a shared slice and
// produces a recordingMaskConn, letting tests assert the exact wrap order of
// the mask chain through the public builders.
type recordingTcpMask struct {
	name   string
	splice bool
	rec    *[]string
	last   net.Conn
}

func (m *recordingTcpMask) TCP() {}

func (m *recordingTcpMask) wrap(raw net.Conn) net.Conn {
	*m.rec = append(*m.rec, m.name)
	m.last = &recordingMaskConn{Conn: raw, splice: m.splice}
	return m.last
}

func (m *recordingTcpMask) WrapConnClient(raw net.Conn) (net.Conn, error) { return m.wrap(raw), nil }
func (m *recordingTcpMask) WrapConnServer(raw net.Conn) (net.Conn, error) { return m.wrap(raw), nil }

// recordingMaskConn implements TcpMaskConn — only definable inside package
// internet because TcpMaskConn has an unexported marker method.
type recordingMaskConn struct {
	net.Conn
	splice bool
}

func (*recordingMaskConn) TcpMaskConn()        {}
func (c *recordingMaskConn) RawConn() net.Conn { return c.Conn }
func (c *recordingMaskConn) Splice() bool      { return c.splice }

// chainRawConn is a minimal net.Conn fake that tracks Close.
type chainRawConn struct {
	net.Conn
	closed bool
}

func (c *chainRawConn) Close() error {
	c.closed = true
	return c.Conn.Close()
}

func TestWrapConnClientMaskOrderBeforeTLS(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()
	raw := &chainRawConn{Conn: client}
	defer func() {
		if !raw.closed {
			client.Close()
		}
	}()

	var order []string
	mss := &MemoryStreamConfig{
		TcpmaskManager: NewTcpmaskManager([]Tcpmask{&recordingTcpMask{name: "mask", rec: &order}}),
	}
	tlsConn := &recordingMaskConn{Conn: raw}
	result, err := WrapConnClient(mss, raw, func(conn net.Conn) (net.Conn, error) {
		*(&order) = append(order, "tls")
		return tlsConn, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(order) != 2 || order[0] != "mask" || order[1] != "tls" {
		t.Fatalf("wrap order = %v, want [mask tls]", order)
	}
	if result != net.Conn(tlsConn) {
		t.Fatalf("result is %T, want the TLS conn", result)
	}
	if raw.closed {
		t.Fatal("raw conn was closed on success")
	}
}

func TestMaskLevelOrder(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	var order []string
	mss := &MemoryStreamConfig{
		TcpmaskManager: NewTcpmaskManager([]Tcpmask{
			&recordingTcpMask{name: "m1", rec: &order},
			&recordingTcpMask{name: "m2", rec: &order},
			&recordingTcpMask{name: "m3", rec: &order},
		}),
	}
	result, err := WrapConnClient(mss, client, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Backward iteration: last config entry wraps first = innermost.
	if len(order) != 3 || order[0] != "m3" || order[1] != "m2" || order[2] != "m1" {
		t.Fatalf("wrap order = %v, want [m3 m2 m1]", order)
	}
	if result != mss.TcpmaskManager.tcpmasks[0].(*recordingTcpMask).last {
		t.Fatalf("outermost conn is %T, want m1's conn", result)
	}
}

func TestWrapListenerServerSideOrder(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	var order []string
	mss := &MemoryStreamConfig{
		TcpmaskManager: NewTcpmaskManager([]Tcpmask{
			&recordingTcpMask{name: "s1", rec: &order},
			&recordingTcpMask{name: "s2", rec: &order},
		}),
	}
	ln, err := WrapListener(mss, &chainListener{conn: server})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := ln.Accept()
	if err != nil {
		t.Fatal(err)
	}
	if len(order) != 2 || order[0] != "s2" || order[1] != "s1" {
		t.Fatalf("server wrap order = %v, want [s2 s1]", order)
	}
	if conn != mss.TcpmaskManager.tcpmasks[0].(*recordingTcpMask).last {
		t.Fatalf("accepted conn is %T, want s1's conn", conn)
	}
}

type chainListener struct {
	net.Listener
	conn net.Conn
}

func (l *chainListener) Accept() (net.Conn, error) { return l.conn, nil }

func TestUnwrapTcpMaskStopsAtNonSplice(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	var order []string
	mss := &MemoryStreamConfig{
		TcpmaskManager: NewTcpmaskManager([]Tcpmask{
			&recordingTcpMask{name: "A", splice: true, rec: &order},
			&recordingTcpMask{name: "B", splice: false, rec: &order},
			&recordingTcpMask{name: "C", splice: true, rec: &order},
		}),
	}
	outermost, err := WrapConnClient(mss, client, nil)
	if err != nil {
		t.Fatal(err)
	}
	masks := mss.TcpmaskManager.tcpmasks
	got := UnwrapTcpMask(outermost)
	// Peel A (splice=true), stop at B (splice=false) — must NOT reach C.
	if want := masks[1].(*recordingTcpMask).last; got != want {
		t.Fatalf("UnwrapTcpMask = %T, want maskB's conn %T", got, want)
	}
	if got == masks[2].(*recordingTcpMask).last {
		t.Fatal("UnwrapTcpMask drilled past the non-splice conn into maskC")
	}
}
