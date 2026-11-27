package tun

import (
	"context"
	"net/netip"
	"strings"
	"syscall"
	"time"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/buf"
	c "github.com/xtls/xray-core/common/ctx"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/log"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/session"
	"github.com/xtls/xray-core/common/tracker"
	"github.com/xtls/xray-core/core"
	finbound "github.com/xtls/xray-core/features/inbound"
	"github.com/xtls/xray-core/features/policy"
	"github.com/xtls/xray-core/features/routing"
	"github.com/xtls/xray-core/features/stats"
	"github.com/xtls/xray-core/transport"
	"github.com/xtls/xray-core/transport/internet"
	"github.com/xtls/xray-core/transport/internet/stat"
)

// Handler is managing object that tie together tun interface, ip stack and dispatch connections to the routing
type Handler struct {
	// ctx is detached from the instance's; cancel ends it, and with it every dispatch started from it,
	// when the handler closes, since nothing else would
	ctx             context.Context
	cancel          context.CancelFunc
	config          *Config
	stack           Stack
	tun             Tun
	policyManager   policy.Manager
	dispatcher      routing.Dispatcher
	tag             string
	sniffingRequest session.SniffingRequest
	uplinkCounter   stats.Counter
	downlinkCounter stats.Counter
	updater         *InterfaceUpdater // this inbound's own, the package-global one may belong to a newer instance

	// sessions tracks the flows the stack is serving, so the instance can report
	// and close them like the connections of a port-based inbound.
	sessions *tracker.Registry
}

type tunUDPStatsWriter struct {
	writer  buf.Writer
	counter stats.Counter
}

func (w *tunUDPStatsWriter) WriteMultiBuffer(mb buf.MultiBuffer) error {
	for len(mb) > 0 {
		remaining, packet := buf.SplitFirst(mb)
		packetSize := packet.Len()
		if err := w.writer.WriteMultiBuffer(buf.MultiBuffer{packet}); err != nil {
			buf.ReleaseMulti(remaining)
			return err
		}
		w.counter.Add(int64(packetSize))
		mb = remaining
	}
	return nil
}

// TCPWriter is the writing side of a TCP connection of the stack for the dispatcher. It has no Close: the outbound
// closes its writer when the other side has no more to send, and the system may still be sending
type TCPWriter struct {
	Writer buf.Writer
	// Counter counts what is written, if there is one
	Counter stats.Counter
}

func (w *TCPWriter) WriteMultiBuffer(mb buf.MultiBuffer) error {
	size := mb.Len()
	if err := w.Writer.WriteMultiBuffer(mb); err != nil {
		return err
	}
	if w.Counter != nil {
		w.Counter.Add(int64(size))
	}
	return nil
}

// ConnectionHandler interface with the only method that stack is going to push new connections to
type ConnectionHandler interface {
	HandleConnection(conn net.Conn, destination net.Destination)
}

// Handler implements ConnectionHandler
var _ ConnectionHandler = (*Handler)(nil)

// Handler implements common.Runnable
var _ common.Runnable = (*Handler)(nil)

// Init the Handler instance with necessary parameters
func (t *Handler) Init(ctx context.Context, pm policy.Manager, dispatcher routing.Dispatcher) error {
	// Retrieve tag and sniffing config from context (set by AlwaysOnInboundHandler)
	if inbound := session.InboundFromContext(ctx); inbound != nil {
		t.tag = inbound.Tag
	}
	if content := session.ContentFromContext(ctx); content != nil {
		t.sniffingRequest = content.SniffingRequest
	}

	t.ctx, t.cancel = context.WithCancel(core.ToBackgroundDetachedContext(ctx))
	t.policyManager = pm
	t.dispatcher = dispatcher
	// The flows of the stack go into the connection table of the instance, like
	// the connections of every other inbound, so that one snapshot and one close
	// reach them too. Without an instance (a bare handler in a unit test) the
	// handler keeps a table of its own.
	t.sessions = tracker.NewRegistry()
	if instance := core.FromContext(ctx); instance != nil {
		if manager, ok := instance.GetFeature(finbound.ManagerType()).(interface {
			Connections() *tracker.Registry
		}); ok {
			t.sessions = manager.Connections()
		}
	}

	if len(t.tag) > 0 && pm.ForSystem().Stats.InboundUplink {
		statsManager := core.MustFromContext(ctx).GetFeature(stats.ManagerType()).(stats.Manager)
		name := "inbound>>>" + t.tag + ">>>traffic>>>uplink"
		c, _ := statsManager.GetOrRegisterCounter(name)
		if c != nil {
			t.uplinkCounter = c
		}
	}
	if len(t.tag) > 0 && pm.ForSystem().Stats.InboundDownlink {
		statsManager := core.MustFromContext(ctx).GetFeature(stats.ManagerType()).(stats.Manager)
		name := "inbound>>>" + t.tag + ">>>traffic>>>downlink"
		c, _ := statsManager.GetOrRegisterCounter(name)
		if c != nil {
			t.downlinkCounter = c
		}
	}

	return nil
}

func (t *Handler) Start() error {
	tunName := t.config.Name
	tunInterface, err := NewTun(t.config)
	if err != nil {
		return err
	}

	if t.config.AutoOutboundsInterface != "" {
		tunIndex, err := tunInterface.Index()
		if err != nil {
			_ = tunInterface.Close()
			return err
		}
		if t.config.AutoOutboundsInterface == "auto" {
			t.config.AutoOutboundsInterface = ""
		}
		u := &InterfaceUpdater{tunIndex: tunIndex, fixedName: t.config.AutoOutboundsInterface}
		updater = u
		t.updater = u
		// A platform that monitors route changes must watch its own updater, not the
		// package one a second tun inbound may overwrite.
		if su, ok := tunInterface.(interface{ setUpdater(*InterfaceUpdater) }); ok {
			su.setUpdater(u)
		}
		u.Update()
		internet.RegisterDialerController(func(network, address string, c syscall.RawConn) error {
			if u.IsClosed() {
				return nil
			}
			iface := u.Get()
			if iface == nil {
				errors.LogInfo(context.Background(), "[tun] falied to set interface > iface == nil")
				return errors.New("iface not found")
			}
			var controlErr, sockErr error
			controlErr = c.Control(func(fd uintptr) {
				addrPort, _ := netip.ParseAddrPort(address)
				// skip loopback
				if addrPort.Addr().IsLoopback() || strings.HasPrefix(strings.ToLower(address), "localhost:") {
					return
				}
				err := setinterface(network, address, fd, iface)
				if err != nil {
					sockErr = errors.New("[tun] falied to set interface").Base(err)
				}
			})
			if sockErr != nil {
				return sockErr
			}
			return controlErr
		})
	}

	errors.LogInfo(t.ctx, tunName, " created")

	tunStackOptions := StackOptions{
		Tun:         tunInterface,
		MTU:         t.config.MTU,
		IdleTimeout: t.policyManager.ForLevel(t.config.UserLevel).Timeouts.ConnectionIdle,
	}
	tunStack, err := NewStack(t.ctx, tunStackOptions, t)
	if err != nil {
		_ = tunInterface.Close()
		return err
	}

	err = tunStack.Start()
	if err != nil {
		_ = tunStack.Close()
		_ = tunInterface.Close()
		return err
	}

	err = tunInterface.Start()
	if err != nil {
		_ = tunStack.Close()
		_ = tunInterface.Close()
		return err
	}

	// Platform-specific system DNS takeover, where the platform implements it.
	// Rather no TUN than one that the system DNS bypasses.
	if c, ok := tunInterface.(interface {
		ConfigureSystemDNS(context.Context, string) error
	}); ok {
		if err := c.ConfigureSystemDNS(t.ctx, t.tag); err != nil {
			_ = tunStack.Close()
			_ = tunInterface.Close()
			return errors.New("unable to set the system DNS (remove autoSystemDnsToGateway to run without)").Base(err)
		}
	}

	t.stack = tunStack
	t.tun = tunInterface

	errors.LogInfo(t.ctx, tunName, " up")
	return nil
}

// HandleConnection pass the connection coming from the ip stack to the routing dispatcher
func (t *Handler) HandleConnection(conn net.Conn, destination net.Destination) {
	// when handling is done with any outcome, always signal back to the incoming connection
	// to close, send completion packets back to the network, and cleanup
	defer conn.Close()

	ctx, cancel := context.WithCancel(t.ctx)
	defer cancel()
	ctx = c.ContextWithID(ctx, session.NewID())

	source := net.DestinationFromAddr(conn.RemoteAddr())

	inbound := session.Inbound{
		Name:          "tun",
		Tag:           t.tag,
		CanSpliceCopy: 3,
		Source:        source,
		User: &protocol.MemoryUser{
			Level: t.config.UserLevel,
		},
	}

	ctx = session.ContextWithInbound(ctx, &inbound)
	entry := &tracker.Entry{Owner: t, Inbound: &inbound, Network: destination.Network, Start: time.Now(), Conn: conn, Cancel: cancel}
	if !t.sessions.Register(entry) {
		// The handler closed while this flow was being accepted.
		return
	}
	defer t.sessions.Unregister(entry)

	ctx = session.ContextWithContent(ctx, &session.Content{
		SniffingRequest: t.sniffingRequest,
	})
	ctx = session.SubContextFromMuxInbound(ctx)

	ctx = log.ContextWithAccessMessage(ctx, &log.AccessMessage{
		From:   inbound.Source,
		To:     destination,
		Status: log.AccessAccepted,
		Reason: "",
	})
	errors.LogInfo(ctx, "processing from ", source, " to ", destination)

	// both kinds of connection are buf.Reader and buf.Writer, what they carry is counted on the way
	reader := &buf.TimeoutWrapperReader{Reader: buf.NewReader(conn), Counter: t.uplinkCounter}
	writer := buf.NewWriter(conn)
	if destination.Network != net.Network_UDP {
		writer = &TCPWriter{Writer: writer, Counter: t.downlinkCounter}
	} else if t.downlinkCounter != nil {
		writer = &tunUDPStatsWriter{writer: writer, counter: t.downlinkCounter}
	}

	link := &transport.Link{
		Reader: reader,
		Writer: writer,
	}
	if err := t.dispatcher.DispatchLink(ctx, destination, link); err != nil {
		errors.LogError(ctx, errors.New("connection closed").Base(err))
	}
}

// Close implements common.Closable.
func (t *Handler) Close() error {
	if t.updater != nil {
		t.updater.Close()
	}
	// cancel first so every dispatch in flight unwinds, then close the stack (resets TCP, ends UDP flows)
	// and the tun
	if t.cancel != nil {
		t.cancel()
	}
	if t.sessions != nil {
		// Only the flows of this handler: the table is shared with the rest of the
		// instance.
		t.sessions.CloseMatchingOf(t, nil)
	}
	return errors.Combine(common.CloseIfExists(t.stack), common.CloseIfExists(t.tun))
}

// Sessions implements inbound.SessionTracker.
func (t *Handler) Sessions(_ context.Context) []finbound.Session {
	return t.sessions.SessionsOf(t)
}

// CloseSessions implements inbound.SessionTracker.
func (t *Handler) CloseSessions(_ context.Context, match func(finbound.Session) bool) int {
	return t.sessions.CloseMatchingOf(t, match)
}

var _ finbound.SessionTracker = (*Handler)(nil)

// Network implements proxy.Inbound
// and exists only to comply to proxy interface, declaring it doesn't listen on any network,
// making the process not open any port for this inbound (input will be network interface)
func (t *Handler) Network() []net.Network {
	return []net.Network{}
}

// Process implements proxy.Inbound
// and exists only to comply to proxy interface, which should never get any inputs due to no listening ports
func (t *Handler) Process(ctx context.Context, network net.Network, conn stat.Connection, dispatcher routing.Dispatcher) error {
	return nil
}

// ConfigObjects is what this package creates from a configuration, for
// main/distro/all to install: nothing registers itself here, so this is the whole
// surface this package adds to the configuration table.
var ConfigObjects = []common.ConfigObject{
	{
		Config: (*Config)(nil),
		New: func(ctx context.Context, config interface{}) (interface{}, error) {
		t := &Handler{config: config.(*Config)}
		err := core.RequireFeatures(ctx, func(pm policy.Manager, dispatcher routing.Dispatcher) error {
			return t.Init(ctx, pm, dispatcher)
		})
		return t, err
	},
	},
}
