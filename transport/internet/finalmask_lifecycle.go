package internet

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// The stateful-connection lifecycle: a mask connection whose handshake must
// complete before proxy data flows. The handshake is lazy (it runs at most
// once, on the first data read or write), serialized by a handshake lock, and
// bracketed by the HandshakeDeadlines arbitration so caller-set deadlines
// always win. The handshake script supplies the protocol I/O; the module owns
// everything about how the connection transitions from handshaking to ready.

// connState is the single state vocabulary for a StatefulConn.
type connState int

const (
	stateHandshaking connState = iota
	stateReady
)

// Stream is the reader/writer pair a handshake commits once it completes.
// The committed stream becomes the connection's data path; Stop is called
// when the connection closes while the stream is committed.
type Stream interface {
	io.Reader
	io.Writer
	Stop()
}

// Handshake is the surface a handshake script sees: the raw connection's
// reader and writer (which the script may swap mid-handshake, e.g. to wrap
// encryption), and Commit, which atomically swaps the data path to the
// completed stream and moves the connection to ready. Commit returns
// net.ErrClosed and stops the stream when the connection closed while the
// handshake was running.
type Handshake struct {
	conn   *StatefulConn
	Reader io.Reader
	Writer io.Writer
}

func (h *Handshake) Commit(stream Stream) error {
	c := h.conn
	c.lifecycleMu.Lock()
	defer c.lifecycleMu.Unlock()

	if c.closed {
		stream.Stop()
		return net.ErrClosed
	}
	c.stream = stream
	c.reader = stream
	c.writer = stream
	c.state = stateReady
	return nil
}

// StatefulConn is a net.Conn that runs a handshake lazily before data flows.
type StatefulConn struct {
	conn          net.Conn
	reader        io.Reader
	writer        io.Writer
	state         connState
	handshakeLock sync.Mutex
	lifecycleMu   sync.Mutex
	closed        bool
	stream        Stream
	deadlines     *HandshakeDeadlines
	script        func(*Handshake) error
}

// NewStatefulConn wraps c with a lazy handshake. r is the initial reader for
// the handshake script (typically a buffered wrapper of c); the script runs
// at most once, on the first Read or Write.
func NewStatefulConn(c net.Conn, r io.Reader, script func(*Handshake) error) *StatefulConn {
	return &StatefulConn{
		conn:      c,
		reader:    r,
		writer:    c,
		state:     stateHandshaking,
		deadlines: NewHandshakeDeadlines(c),
		script:    script,
	}
}

func (c *StatefulConn) handshake() error {
	c.handshakeLock.Lock()
	defer c.handshakeLock.Unlock()

	if c.state == stateReady {
		return nil
	}

	h := &Handshake{conn: c, Reader: c.reader, Writer: c.writer}
	if err := c.deadlines.BeginHandshake(); err != nil {
		return err
	}
	defer func() { _ = c.deadlines.EndHandshake() }()
	return c.script(h)
}

func (c *StatefulConn) Read(b []byte) (int, error) {
	if err := c.handshake(); err != nil {
		return 0, fmt.Errorf("handshake: %w", err)
	}
	return c.reader.Read(b)
}

func (c *StatefulConn) Write(b []byte) (int, error) {
	if err := c.handshake(); err != nil {
		return 0, fmt.Errorf("handshake: %w", err)
	}
	return c.writer.Write(b)
}

// Stream returns the committed stream, or nil while the connection is
// still handshaking.
func (c *StatefulConn) Stream() Stream {
	c.lifecycleMu.Lock()
	defer c.lifecycleMu.Unlock()
	return c.stream
}

func (c *StatefulConn) Close() error {
	c.lifecycleMu.Lock()
	c.closed = true
	stream := c.stream
	c.lifecycleMu.Unlock()
	if stream != nil {
		stream.Stop()
	}
	return c.conn.Close()
}

func (c *StatefulConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *StatefulConn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *StatefulConn) SetDeadline(t time.Time) error {
	return c.deadlines.SetDeadline(t)
}

func (c *StatefulConn) SetReadDeadline(t time.Time) error {
	return c.deadlines.SetReadDeadline(t)
}

func (c *StatefulConn) SetWriteDeadline(t time.Time) error {
	return c.deadlines.SetWriteDeadline(t)
}
