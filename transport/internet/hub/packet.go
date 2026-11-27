package hub

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/buf"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/serial"
	"github.com/xtls/xray-core/common/signal/done"
	"github.com/xtls/xray-core/common/task"
	"github.com/xtls/xray-core/features/stats"
	"github.com/xtls/xray-core/transport/internet"
	"github.com/xtls/xray-core/transport/internet/udp"
	"github.com/xtls/xray-core/transport/pipe"
)

// PacketConn is one UDP session: everything a peer sends to one source address
// goes through its pipe, and everything written to it goes back out through the
// socket of the listener.
type PacketConn struct {
	lastActivityTime int64 // in seconds
	reader           buf.Reader
	writer           buf.Writer
	output           func([]byte) (int, error)
	remote           net.Addr
	local            net.Addr
	done             *done.Instance
	uplink           stats.Counter
	downlink         stats.Counter
	inactive         bool
	cancel           context.CancelFunc
}

// SetCancel records the cancel function of the session, so that closing the
// connection stops whoever serves it.
func (c *PacketConn) SetCancel(cancel context.CancelFunc) {
	c.cancel = cancel
}

// Inactive reports whether the session was dropped, e.g. by the idle checker.
func (c *PacketConn) Inactive() bool {
	return c.inactive
}

func (c *PacketConn) setInactive() {
	c.inactive = true
}

func (c *PacketConn) updateActivity() {
	atomic.StoreInt64(&c.lastActivityTime, time.Now().Unix())
}

// ReadMultiBuffer implements buf.Reader.
func (c *PacketConn) ReadMultiBuffer() (buf.MultiBuffer, error) {
	mb, err := c.reader.ReadMultiBuffer()
	if err != nil {
		return nil, err
	}
	c.updateActivity()
	if c.uplink != nil {
		c.uplink.Add(int64(mb.Len()))
	}
	return mb, nil
}

func (c *PacketConn) Read([]byte) (int, error) {
	panic("not implemented")
}

// Write implements io.Writer.
func (c *PacketConn) Write(b []byte) (int, error) {
	n, err := c.output(b)
	if c.downlink != nil {
		c.downlink.Add(int64(n))
	}
	if err == nil {
		c.updateActivity()
	}
	return n, err
}

func (c *PacketConn) Close() error {
	if c.cancel != nil {
		c.cancel()
	}
	common.Must(c.done.Close())
	common.Must(common.Close(c.writer))
	return nil
}

func (c *PacketConn) RemoteAddr() net.Addr { return c.remote }
func (c *PacketConn) LocalAddr() net.Addr  { return c.local }

func (*PacketConn) SetDeadline(time.Time) error      { return nil }
func (*PacketConn) SetReadDeadline(time.Time) error  { return nil }
func (*PacketConn) SetWriteDeadline(time.Time) error { return nil }

// connID identifies a session by the peer address it comes from.
type connID struct {
	src net.Destination
}

// PacketHandler is what an inbound does with a session. It is called once per
// session, in its own goroutine, and must block until the session ends.
type PacketHandler func(ctx context.Context, conn *PacketConn, source net.Destination, originalDest net.Destination)

// PacketOptions are the transport-level extras of a packet listener.
type PacketOptions struct {
	Uplink   stats.Counter
	Downlink stats.Counter
	// Capacity is the number of packets the socket queue can hold.
	Capacity int
}

// PacketListener is a started UDP accept loop: it owns the socket, the sessions
// of the peers sending to it, and the goroutines serving them.
type PacketListener struct {
	sync.RWMutex

	socket    *udp.Hub
	address   net.Address
	port      net.Port
	handler   PacketHandler
	options   PacketOptions
	checker   *task.Periodic
	activeSet map[connID]*PacketConn

	ctx         context.Context
	connCtx     context.Context
	cancelConns context.CancelFunc
}

// ListenPacket starts accepting datagrams on address:port.
func ListenPacket(ctx context.Context, address net.Address, port net.Port, settings *internet.MemoryStreamConfig, handler PacketHandler, options PacketOptions) (*PacketListener, error) {
	connCtx, cancelConns := context.WithCancel(ctx)
	capacity := options.Capacity
	if capacity <= 0 {
		capacity = 256
	}
	socket, err := udp.ListenUDP(connCtx, address, port, settings, udp.HubCapacity(capacity))
	if err != nil {
		cancelConns()
		return nil, err
	}

	l := &PacketListener{
		socket:      socket,
		address:     address,
		port:        port,
		handler:     handler,
		options:     options,
		activeSet:   make(map[connID]*PacketConn, 16),
		ctx:         ctx,
		connCtx:     connCtx,
		cancelConns: cancelConns,
	}
	l.checker = &task.Periodic{
		Interval: time.Minute,
		Execute:  l.clean,
	}
	go l.handlePackets()
	return l, nil
}

// Addr is the address the listener accepts on.
func (l *PacketListener) Addr() net.Addr { return l.socket.Addr() }

// Port is the port the listener accepts on.
func (l *PacketListener) Port() net.Port { return l.port }

func (l *PacketListener) handlePackets() {
	receive := l.socket.Receive()
	for payload := range receive {
		l.dispatch(payload.Payload, payload.Source, payload.Target)
	}
}

// dispatch routes one datagram to the session of its peer, starting a session
// when the peer is new.
func (l *PacketListener) dispatch(b *buf.Buffer, source net.Destination, originalDest net.Destination) {
	id := connID{src: source}
	if originalDest.IsValid() {
		b.UDP = &originalDest
	}
	conn, existing := l.getConnection(id, originalDest)

	// The payload is discarded if the pipe is full.
	conn.writer.WriteMultiBuffer(buf.MultiBuffer{b})

	if existing {
		return
	}

	common.Must(l.checker.Start())

	go func() {
		ctx, cancel := context.WithCancel(l.connCtx)
		conn.cancel = cancel
		l.handler(ctx, conn, source, originalDest)
	}()
}

func (l *PacketListener) getConnection(id connID, originalDest net.Destination) (*PacketConn, bool) {
	l.Lock()
	defer l.Unlock()

	if conn, found := l.activeSet[id]; found && !conn.done.Done() {
		conn.updateActivity()
		return conn, true
	}

	pReader, pWriter := pipe.New(pipe.DiscardOverflow(), pipe.WithSizeLimit(16*1024))
	conn := &PacketConn{
		reader: pReader,
		writer: pWriter,
		output: func(b []byte) (int, error) {
			return l.socket.WriteTo(b, id.src)
		},
		remote: &net.UDPAddr{
			IP:   id.src.Address.IP(),
			Port: int(id.src.Port),
		},
		local: &net.UDPAddr{
			IP:   l.address.IP(),
			Port: int(l.port),
		},
		done:     done.New(),
		uplink:   l.options.Uplink,
		downlink: l.options.Downlink,
	}
	l.activeSet[id] = conn
	conn.updateActivity()
	return conn, false
}

// RemoveConn drops the session of id, e.g. after the inbound served it.
func (l *PacketListener) RemoveConn(source net.Destination) {
	l.Lock()
	delete(l.activeSet, connID{src: source})
	l.Unlock()
}

func (l *PacketListener) clean() error {
	nowSec := time.Now().Unix()
	l.Lock()
	defer l.Unlock()

	if len(l.activeSet) == 0 {
		return errors.New("no more connections. stopping...")
	}

	for addr, conn := range l.activeSet {
		if nowSec-atomic.LoadInt64(&conn.lastActivityTime) > 2*60 {
			if !conn.inactive {
				conn.setInactive()
				delete(l.activeSet, addr)
			}
			conn.Close()
		}
	}

	if len(l.activeSet) == 0 {
		l.activeSet = make(map[connID]*PacketConn, 16)
	}
	return nil
}

// Close stops the socket, the sessions and the idle checker.
func (l *PacketListener) Close() error {
	if l == nil {
		return nil
	}
	l.Lock()

	var errs []interface{}
	if l.cancelConns != nil {
		l.cancelConns()
	}
	for id, conn := range l.activeSet {
		if !conn.inactive {
			conn.setInactive()
		}
		delete(l.activeSet, id)
		if err := conn.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	l.Unlock()

	if l.socket != nil {
		if err := l.socket.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if l.checker != nil {
		if err := l.checker.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.New("failed to close the packet listener").Base(errors.New(serial.Concat(errs...)))
	}
	return nil
}
