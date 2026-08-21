package rawconn

import (
	"net"
	"testing"

	"github.com/xtls/xray-core/transport/internet"
	"github.com/xtls/xray-core/transport/internet/stat"
)

// wrapper is an opaque Unwrapper, standing in for TLS/Reality/CommonConn-style
// wrappers that must NOT be treated as splice-transparent.
type wrapper struct{ net.Conn }

func (w wrapper) Unwrap() net.Conn { return w.Conn }

// tcpMaskStub implements finalmask.TcpMaskConn with a controllable Splice flag.
// It embeds net.Conn so it also satisfies stat.Connection for IsRAW.
type tcpMaskStub struct {
	net.Conn
	raw    net.Conn
	splice bool
}

func (tcpMaskStub) TcpMaskConn()        {}
func (s tcpMaskStub) RawConn() net.Conn { return s.raw }
func (s tcpMaskStub) Splice() bool      { return s.splice }

func TestPeelSpliceTransparent_StopsAtOpaque(t *testing.T) {
	core := &net.TCPConn{}
	conn := wrapper{Conn: core}
	got, _, _ := peelSpliceTransparent(conn, nil, nil)
	if got != conn {
		t.Fatalf("peeled through opaque wrapper: got %T, want the wrapper itself", got)
	}
}

func TestPeelSpliceTransparent_PeelsCounterAndMask(t *testing.T) {
	core := &net.TCPConn{}
	conn := &stat.CounterConnection{Connection: tcpMaskStub{raw: core, splice: true}}
	got, _, _ := peelSpliceTransparent(conn, nil, nil)
	if got != core {
		t.Fatalf("expected to peel counter+mask down to core, got %T", got)
	}
}

func TestIsRAW_GuardAgainstOpaqueWrapper(t *testing.T) {
	// Freedom's MITM guard: a TLS-style wrapper must never read as RAW.
	if IsRAW(wrapper{Conn: &net.TCPConn{}}) {
		t.Error("expected TLS-style wrapper to NOT be RAW")
	}
}

func TestIsRAW_TcpMaskNonSpliceStops(t *testing.T) {
	if IsRAW(tcpMaskStub{raw: &net.TCPConn{}, splice: false}) {
		t.Error("expected non-splicing TcpMask to NOT be RAW")
	}
}

func TestUnwrap_PeelsThroughOpaqueIntoMask(t *testing.T) {
	// Unlike peelSpliceTransparent, Unwrap deep-peels: wrapper -> mask -> core.
	core := &net.TCPConn{}
	conn := wrapper{Conn: tcpMaskStub{raw: core, splice: true}}
	got, _, _ := Unwrap(conn)
	if got != core {
		t.Fatalf("expected deep unwrap to reach core, got %T", got)
	}
}

func TestUnwrap_CounterInsideWrapper(t *testing.T) {
	// Mixed nesting: opaque wrapper on top of a counter — counters must still
	// be collected after the wrapper is peeled.
	core := &net.TCPConn{}
	counter := noopCounter{}
	conn := wrapper{Conn: &stat.CounterConnection{Connection: core, ReadCounter: counter}}
	got, readCounter, _ := Unwrap(conn)
	if got != core {
		t.Fatalf("expected to reach core, got %T", got)
	}
	if readCounter == nil {
		t.Error("expected read counter to be collected through the wrapper")
	}
}

type noopCounter struct{}

func (noopCounter) Add(int64) int64 { return 0 }
func (noopCounter) Set(int64) int64 { return 0 }
func (noopCounter) Value() int64    { return 0 }

var _ internet.TcpMaskConn = tcpMaskStub{}
