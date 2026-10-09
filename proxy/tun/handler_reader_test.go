package tun

import (
	"context"
	"errors"
	"io"
	"slices"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/xtls/xray-core/common/buf"
	xnet "github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/transport/internet/stat"
)

// lazyTestConn offers its payload through the lazy buffer API of the stacks
// that have one, and records the events of a read so a test can tell that the
// buffer was taken once payload was on offer, not before.
type lazyTestConn struct {
	*testConn

	mu      sync.Mutex
	events  []string
	hints   []int
	pending []byte
}

func newLazyTestConn(payload []byte) *lazyTestConn {
	return &lazyTestConn{testConn: newTestConn(nil), pending: payload}
}

func (c *lazyTestConn) ReadWithBuffer(getBuffer func(sizeHint int) []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.events = append(c.events, "read")
	if len(c.pending) == 0 {
		return 0, io.EOF
	}

	c.hints = append(c.hints, len(c.pending))
	buffer := getBuffer(len(c.pending))
	c.events = append(c.events, "buffer")
	n := copy(buffer, c.pending)
	c.pending = c.pending[n:]
	return n, nil
}

func (c *lazyTestConn) recordedEvents() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.events)
}

func (c *lazyTestConn) recordedHints() []int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.hints)
}

func TestUplinkReaderPrefersTheLazyRead(t *testing.T) {
	if reader := newUplinkReader(newLazyTestConn(nil), nil); reader == nil {
		t.Fatal("no reader for a lazy connection")
	} else if _, ok := reader.(*lazyReader); !ok {
		t.Fatalf("lazy connection got %T, want a lazy reader", reader)
	}

	wrapped := &stat.CounterConnection{Connection: newLazyTestConn(nil), ReadCounter: new(testCounter)}
	if reader := newUplinkReader(wrapped, nil); reader == nil {
		t.Fatal("no reader for a counted lazy connection")
	} else if _, ok := reader.(*lazyReader); !ok {
		t.Fatalf("counted lazy connection got %T, want a lazy reader", reader)
	}

	if reader := newUplinkReader(newTestConn(nil), nil); reader == nil {
		t.Fatal("no reader for an eager connection")
	} else if _, ok := reader.(*lazyReader); ok {
		t.Fatal("a connection without the lazy read got a lazy reader")
	}
}

func TestLazyReaderTakesTheBufferWhenPayloadArrives(t *testing.T) {
	counter := new(testCounter)
	conn := newLazyTestConn([]byte("uplink"))
	reader := newUplinkReader(conn, counter)

	mb, err := reader.ReadMultiBuffer()
	if err != nil {
		t.Fatalf("read the uplink payload: %v", err)
	}
	if got := mb.Len(); got != int32(len("uplink")) {
		t.Fatalf("read payload length = %d, want %d", got, len("uplink"))
	}
	if got := string(mb[0].Bytes()); got != "uplink" {
		t.Fatalf("read payload = %q, want %q", got, "uplink")
	}
	buf.ReleaseMulti(mb)

	if got := counter.Value(); got != int64(len("uplink")) {
		t.Fatalf("uplink counter = %d, want %d", got, len("uplink"))
	}
	if got := conn.recordedHints(); !slices.Equal(got, []int{len("uplink")}) {
		t.Fatalf("buffer hints = %v, want %v", got, []int{len("uplink")})
	}
	if got := conn.recordedEvents(); !slices.Equal(got, []string{"read", "buffer"}) {
		t.Fatalf("read events = %v, want the buffer taken after payload arrived", got)
	}
}

func TestLazyReaderTakesNoBufferForATerminalState(t *testing.T) {
	counter := new(testCounter)
	conn := newLazyTestConn(nil)
	reader := newUplinkReader(conn, counter)

	mb, err := reader.ReadMultiBuffer()
	if !errors.Is(err, io.EOF) {
		t.Fatalf("read of a finished connection = %v, want EOF", err)
	}
	if mb != nil {
		t.Fatalf("read of a finished connection = %v, want no buffer", mb)
	}
	if got := counter.Value(); got != 0 {
		t.Fatalf("uplink counter = %d, want 0", got)
	}
	if got := conn.recordedEvents(); !slices.Equal(got, []string{"read"}) {
		t.Fatalf("read events = %v, want no buffer taken", got)
	}
}

func TestHandlerCountsLazyTunConnectionTraffic(t *testing.T) {
	uplinkCounter := new(testCounter)
	downlinkCounter := new(testCounter)
	dispatcher := &testDispatcher{writePayload: []byte("downlink")}
	conn := newLazyTestConn([]byte("uplink"))

	handler := &Handler{
		ctx:             context.Background(),
		config:          &Config{},
		dispatcher:      dispatcher,
		uplinkCounter:   uplinkCounter,
		downlinkCounter: downlinkCounter,
	}
	handler.HandleConnection(conn, xnet.TCPDestination(xnet.LocalHostIP, 443))

	if got := uplinkCounter.Value(); got != int64(len("uplink")) {
		t.Fatalf("unexpected uplink counter: got %d, want %d", got, len("uplink"))
	}
	if got := downlinkCounter.Value(); got != int64(len("downlink")) {
		t.Fatalf("unexpected downlink counter: got %d, want %d", got, len("downlink"))
	}
	if got := int(atomic.LoadInt32(&dispatcher.readBytes)); got != len("uplink") {
		t.Fatalf("dispatcher read unexpected bytes: got %d, want %d", got, len("uplink"))
	}
}
