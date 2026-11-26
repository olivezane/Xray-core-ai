package inbound

import (
	"context"
	"sync"
	"testing"
	"time"

	gonet "net"

	"github.com/xtls/xray-core/common"
	c "github.com/xtls/xray-core/common/ctx"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/session"
	finbound "github.com/xtls/xray-core/features/inbound"
	"github.com/xtls/xray-core/features/routing"
	"github.com/xtls/xray-core/proxy"
	"github.com/xtls/xray-core/transport/internet"
	"github.com/xtls/xray-core/transport/internet/stat"
	"github.com/xtls/xray-core/transport/internet/tcp"
)

// ctxRecorder records what the worker hands to the proxy and blocks like a proxy
// serving a connection until the context is cancelled.
type ctxRecorder struct {
	ctxs chan context.Context
	// start, when not nil, runs in Process before it blocks: it stands in for a
	// proxy authenticating the connection.
	start func(context.Context)
	// ready is closed after start ran, so tests can observe the registry then.
	ready chan struct{}
}

func (p *ctxRecorder) Network() []net.Network {
	return []net.Network{net.Network_TCP, net.Network_UNIX}
}

func (p *ctxRecorder) Process(ctx context.Context, _ net.Network, _ stat.Connection, _ routing.Dispatcher) error {
	select {
	case p.ctxs <- ctx:
	default:
	}
	if p.start != nil {
		p.start(ctx)
	}
	if p.ready != nil {
		close(p.ready)
	}
	<-ctx.Done()
	return nil
}

var _ proxy.Inbound = (*ctxRecorder)(nil)

type ctxKey struct{}

func newTCPWorkerForTest(t *testing.T, parent context.Context, recorder proxy.Inbound) *tcpWorker {
	t.Helper()
	w := &tcpWorker{
		address: net.LocalHostIP,
		port:    pickPort(t),
		proxy:   recorder,
		stream: &internet.MemoryStreamConfig{
			ProtocolName:     "tcp",
			ProtocolSettings: &tcp.Config{},
		},
		sessions: newConnRegistry(),
		ctx:      parent,
	}
	if err := w.Start(); err != nil {
		t.Fatal("failed to start worker: ", err)
	}
	return w
}

func dialWorker(t *testing.T, w *tcpWorker) gonet.Conn {
	t.Helper()
	conn, err := gonet.Dial("tcp", gonet.JoinHostPort("127.0.0.1", w.port.String()))
	if err != nil {
		t.Fatal("failed to dial worker: ", err)
	}
	return conn
}

// TestTCPWorkerTransportContext verifies that the transport context reaches the
// proxy: the context given to Process carries the listening context's values and
// is cancelled when the worker is closed.
func TestTCPWorkerTransportContext(t *testing.T) {
	recorder := &ctxRecorder{ctxs: make(chan context.Context, 4)}

	parent, cancelParent := context.WithCancel(context.WithValue(context.Background(), ctxKey{}, "transport-value"))
	defer cancelParent()

	w := newTCPWorkerForTest(t, parent, recorder)
	conn := dialWorker(t, w)
	defer conn.Close()

	var procCtx context.Context
	select {
	case procCtx = <-recorder.ctxs:
	case <-time.After(5 * time.Second):
		t.Fatal("proxy did not receive the connection")
	}

	if v, _ := procCtx.Value(ctxKey{}).(string); v != "transport-value" {
		t.Error("transport context value did not reach the proxy, got: ", procCtx.Value(ctxKey{}))
	}
	if c.IDFromContext(procCtx) == 0 {
		t.Error("connection context has no session id")
	}
	if inbound := session.InboundFromContext(procCtx); inbound == nil || inbound.Conn == nil {
		t.Errorf("unexpected inbound metadata: %+v", inbound)
	}

	if err := w.Close(); err != nil {
		t.Error("failed to close worker: ", err)
	}
	select {
	case <-procCtx.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("connection context was not cancelled by worker Close")
	}
}

// TestWorkerReportsSessions verifies that a served connection shows up in the
// registry with the user the proxy authenticated it as, and that closing the
// sessions of that user closes the connection.
func TestWorkerReportsSessions(t *testing.T) {
	recorder := &ctxRecorder{
		ctxs:  make(chan context.Context, 4),
		ready: make(chan struct{}),
		start: func(ctx context.Context) {
			// What a protocol handler does once it authenticated the connection.
			session.InboundFromContext(ctx).User = &protocol.MemoryUser{Email: "user@example.com", Level: 0}
		},
	}

	w := newTCPWorkerForTest(t, context.Background(), recorder)
	conn := dialWorker(t, w)
	defer conn.Close()

	select {
	case <-recorder.ready:
	case <-time.After(5 * time.Second):
		t.Fatal("proxy did not receive the connection")
	}

	sessions := w.sessions.sessions()
	if len(sessions) != 1 {
		t.Fatalf("expected 1 tracked session, got %d: %+v", len(sessions), sessions)
	}
	s := sessions[0]
	if s.Email != "user@example.com" {
		t.Error("session is missing the authenticated user: ", s.Email)
	}
	if s.Network != net.Network_TCP || s.InboundTag != "" || !s.Source.IsValid() {
		t.Errorf("unexpected session metadata: %+v", s)
	}

	// Another user's connection must be left alone.
	if closed := w.sessions.closeMatching(func(finbound.Session) bool { return false }); closed != 0 {
		t.Errorf("closed %d sessions that did not match", closed)
	}
	if closed := w.sessions.closeMatching(func(s finbound.Session) bool { return s.Email == "user@example.com" }); closed != 1 {
		t.Fatalf("expected to close 1 session of the user, closed %d", closed)
	}

	// The closed connection is unregistered once its worker is done with it.
	deadline := time.Now().Add(5 * time.Second)
	for len(w.sessions.sessions()) != 0 {
		if time.Now().After(deadline) {
			t.Fatal("session stayed registered after being closed")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := w.Close(); err != nil {
		t.Error("failed to close worker: ", err)
	}
}

// TestConnRegistry verifies the ownership rules: closeAll closes the tracked
// connections, and a closed registry refuses new ones.
func TestConnRegistry(t *testing.T) {
	r := newConnRegistry()

	first := &stubConn{done: make(chan struct{})}
	second := &stubConn{done: make(chan struct{})}
	if !r.register(&connection{Conn: first, Network: net.Network_TCP, Start: time.Now()}) {
		t.Fatal("registry refused a connection while open")
	}
	if !r.register(&connection{Conn: second, Network: net.Network_TCP, Start: time.Now()}) {
		t.Fatal("registry refused a connection while open")
	}
	r.unregister(second)

	if sessions := r.sessions(); len(sessions) != 1 {
		t.Fatalf("expected 1 tracked session, got %d", len(sessions))
	}

	r.closeAll()

	select {
	case <-first.done:
	default:
		t.Error("closeAll did not close the tracked connection")
	}
	select {
	case <-second.done:
		t.Error("closeAll closed a connection that was unregistered")
	default:
	}

	if r.register(&connection{Conn: &stubConn{done: make(chan struct{})}}) {
		t.Error("a closed registry accepted a connection")
	}
}

func pickPort(t *testing.T) net.Port {
	t.Helper()
	l, err := gonet.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal("failed to pick a port: ", err)
	}
	port := net.Port(l.Addr().(*gonet.TCPAddr).Port)
	common.CloseIfExists(l)
	return port
}

type stubConn struct {
	done     chan struct{}
	doneOnce sync.Once
}

func (c *stubConn) Read([]byte) (int, error)         { return 0, gonet.ErrClosed }
func (c *stubConn) Write(b []byte) (int, error)      { return len(b), nil }
func (c *stubConn) Close() error                     { c.doneOnce.Do(func() { close(c.done) }); return nil }
func (c *stubConn) LocalAddr() gonet.Addr            { return nil }
func (c *stubConn) RemoteAddr() gonet.Addr           { return nil }
func (c *stubConn) SetDeadline(time.Time) error      { return nil }
func (c *stubConn) SetReadDeadline(time.Time) error  { return nil }
func (c *stubConn) SetWriteDeadline(time.Time) error { return nil }
