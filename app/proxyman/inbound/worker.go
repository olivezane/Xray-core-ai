package inbound

import (
	"context"
	"time"

	"github.com/xtls/xray-core/common"
	c "github.com/xtls/xray-core/common/ctx"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/session"
	"github.com/xtls/xray-core/features/routing"
	"github.com/xtls/xray-core/features/stats"
	"github.com/xtls/xray-core/proxy"
	hysteria_proxy "github.com/xtls/xray-core/proxy/hysteria"
	"github.com/xtls/xray-core/transport/internet"
	"github.com/xtls/xray-core/transport/internet/hub"
	"github.com/xtls/xray-core/transport/internet/hysteria"
	"github.com/xtls/xray-core/transport/internet/stat"
	"github.com/xtls/xray-core/transport/internet/tcp"
)

type worker interface {
	Start() error
	Close() error
	Port() net.Port
	Proxy() proxy.Inbound
}

type tcpWorker struct {
	address         net.Address
	port            net.Port
	proxy           proxy.Inbound
	stream          *internet.MemoryStreamConfig
	recvOrigDest    bool
	tag             string
	dispatcher      routing.Dispatcher
	sniffingRequest session.SniffingRequest
	uplinkCounter   stats.Counter
	downlinkCounter stats.Counter

	// hub is the accept loop of the transport, which owns the listener and the
	// context every connection of this worker descends from.
	hub *hub.Listener

	ctx context.Context
	// sessions tracks the connections handed to the proxy. It belongs to the
	// inbound handler and is shared by all of its workers.
	sessions *connRegistry
}

func getTProxyType(s *internet.MemoryStreamConfig) internet.SocketConfig_TProxyMode {
	if s == nil || s.SocketSettings == nil {
		return internet.SocketConfig_Off
	}
	return s.SocketSettings.Tproxy
}

func (w *tcpWorker) callback(connCtx context.Context, conn stat.Connection) {
	if connCtx == nil {
		connCtx = w.ctx
	}
	// The transport context is the parent of the connection context: values set
	// by the transport (e.g. the HYSTERIA validator) reach the proxy, and the
	// connection is cancelled when the transport or the worker goes away.
	ctx, cancel := context.WithCancel(connCtx)
	defer cancel()
	sid := session.NewID()
	ctx = c.ContextWithID(ctx, sid)

	outbounds := []*session.Outbound{{}}
	if w.recvOrigDest {
		var dest net.Destination
		switch getTProxyType(w.stream) {
		case internet.SocketConfig_Redirect:
			d, err := tcp.GetOriginalDestination(conn)
			if err != nil {
				errors.LogInfoInner(ctx, err, "failed to get original destination")
			} else {
				dest = d
			}
		case internet.SocketConfig_TProxy:
			dest = net.DestinationFromAddr(conn.LocalAddr())
		}

		if dest.IsValid() {
			// Check if try to connect to this inbound itself (can cause loopback)
			var isLoopBack bool
			if w.address == net.AnyIP || w.address == net.AnyIPv6 {
				if dest.Port.Value() == w.port.Value() && IsLocal(dest.Address.IP()) {
					isLoopBack = true
				}
			} else {
				if w.hub.Addr().String() == dest.NetAddr() {
					isLoopBack = true
				}
			}
			if isLoopBack {
				cancel()
				conn.Close()
				errors.LogError(ctx, errors.New("loopback connection detected"))
				return
			}
			outbounds[0].Target = dest
		}
	}
	ctx = session.ContextWithOutbounds(ctx, outbounds)

	if w.uplinkCounter != nil || w.downlinkCounter != nil {
		conn = &stat.CounterConnection{
			Connection:   conn,
			ReadCounter:  w.uplinkCounter,
			WriteCounter: w.downlinkCounter,
		}
	}
	inbound := &session.Inbound{
		Source:  net.DestinationFromAddr(conn.RemoteAddr()),
		Local:   net.DestinationFromAddr(conn.LocalAddr()),
		Gateway: net.TCPDestination(w.address, w.port),
		Tag:     w.tag,
		Conn:    conn,
	}
	ctx = session.ContextWithInbound(ctx, inbound)
	entry, ok := w.sessions.register(&connection{Inbound: inbound, Network: net.Network_TCP, Start: time.Now(), Conn: conn, Cancel: cancel})
	if !ok {
		common.CloseIfExists(conn)
		return
	}
	defer w.sessions.unregister(entry)

	content := new(session.Content)
	content.SniffingRequest = w.sniffingRequest
	ctx = session.ContextWithContent(ctx, content)

	if err := w.proxy.Process(ctx, net.Network_TCP, conn, w.dispatcher); err != nil {
		errors.LogInfoInner(ctx, err, "connection ends")
	}
	common.CloseIfExists(conn)
}

func (w *tcpWorker) Proxy() proxy.Inbound {
	return w.proxy
}

func (w *tcpWorker) Start() error {
	var options hub.Options
	if v, ok := w.proxy.(*hysteria_proxy.Server); ok {
		// The validator is transport state of HYSTERIA, so the hub installs it in
		// the context the listener and its connections start from.
		options.PrepareStreamContext = func(ctx context.Context) context.Context {
			return hysteria.ContextWithValidator(ctx, v.HysteriaInboundValidator())
		}
	}

	listener, err := hub.ListenTCP(w.ctx, w.address, w.port, w.stream, w.callback, options)
	if err != nil {
		return errors.New("failed to listen TCP on ", w.port).Base(err)
	}
	w.hub = listener
	return nil
}

func (w *tcpWorker) Close() error {
	if w.hub == nil {
		return nil
	}
	// The hub closes the listener and cancels the connections; the proxy is closed
	// here because it belongs to this worker, not to the transport.
	return hub.CloseAll(w.hub, w.proxy)
}

func (w *tcpWorker) Port() net.Port {
	return w.hub.Port()
}

// udpWorker serves one UDP port. The socket, the sessions of the peers sending to
// it and their idle cleanup belong to the transport and live in hub; this worker
// only says what the sessions mean to the inbound: their context, their
// metadata, and the proxy that processes them.
type udpWorker struct {
	proxy           proxy.Inbound
	hub             *hub.PacketListener
	address         net.Address
	port            net.Port
	tag             string
	stream          *internet.MemoryStreamConfig
	dispatcher      routing.Dispatcher
	sniffingRequest session.SniffingRequest
	uplinkCounter   stats.Counter
	downlinkCounter stats.Counter

	ctx context.Context

	// sessions tracks the udpConn sessions handed to the proxy. It belongs to
	// the inbound handler and is shared by all of its workers.
	sessions *connRegistry
}

// serve is what the hub calls once per peer session.
func (w *udpWorker) serve(ctx context.Context, conn *hub.PacketConn, source net.Destination, originalDest net.Destination) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	conn.SetCancel(cancel)
	sid := session.NewID()
	ctx = c.ContextWithID(ctx, sid)

	outbounds := []*session.Outbound{{}}
	if originalDest.IsValid() {
		outbounds[0].Target = originalDest
	}
	ctx = session.ContextWithOutbounds(ctx, outbounds)

	local := net.DestinationFromAddr(w.hub.Addr())
	if local.Address == net.AnyIP || local.Address == net.AnyIPv6 {
		if source.Address.Family().IsIPv4() {
			local.Address = net.AnyIP
		} else {
			local.Address = net.AnyIPv6
		}
	}

	inbound := &session.Inbound{
		Source:  source,
		Local:   local, // Due to some limitations, in UDP connections, localIP is always equal to listen interface IP
		Gateway: net.UDPDestination(w.address, w.port),
		Tag:     w.tag,
	}
	ctx = session.ContextWithInbound(ctx, inbound)
	content := new(session.Content)
	content.SniffingRequest = w.sniffingRequest
	ctx = session.ContextWithContent(ctx, content)

	entry, ok := w.sessions.register(&connection{Inbound: inbound, Network: net.Network_UDP, Start: time.Now(), Conn: conn, Cancel: cancel})
	if ok {
		if err := w.proxy.Process(ctx, net.Network_UDP, conn, w.dispatcher); err != nil {
			errors.LogInfoInner(ctx, err, "connection ends")
		}
		w.sessions.unregister(entry)
	}
	conn.Close()
	w.hub.RemoveConn(source)
}

func (w *udpWorker) Start() error {
	listener, err := hub.ListenPacket(w.ctx, w.address, w.port, w.stream, w.serve, hub.PacketOptions{
		Uplink:   w.uplinkCounter,
		Downlink: w.downlinkCounter,
	})
	if err != nil {
		return err
	}
	w.hub = listener
	return nil
}

func (w *udpWorker) Close() error {
	if w.hub == nil {
		return nil
	}
	return hub.CloseAll(w.hub, w.proxy)
}

func (w *udpWorker) Port() net.Port {
	return w.port
}

func (w *udpWorker) Proxy() proxy.Inbound {
	return w.proxy
}

type dsWorker struct {
	address         net.Address
	proxy           proxy.Inbound
	stream          *internet.MemoryStreamConfig
	tag             string
	dispatcher      routing.Dispatcher
	sniffingRequest session.SniffingRequest
	uplinkCounter   stats.Counter
	downlinkCounter stats.Counter

	hub *hub.Listener

	ctx context.Context

	sessions *connRegistry
}

func (w *dsWorker) callback(connCtx context.Context, conn stat.Connection) {
	if connCtx == nil {
		connCtx = w.ctx
	}
	ctx, cancel := context.WithCancel(connCtx)
	defer cancel()
	sid := session.NewID()
	ctx = c.ContextWithID(ctx, sid)

	if w.uplinkCounter != nil || w.downlinkCounter != nil {
		conn = &stat.CounterConnection{
			Connection:   conn,
			ReadCounter:  w.uplinkCounter,
			WriteCounter: w.downlinkCounter,
		}
	}
	inbound := &session.Inbound{
		Source:  net.DestinationFromAddr(conn.RemoteAddr()),
		Local:   net.DestinationFromAddr(conn.LocalAddr()),
		Gateway: net.UnixDestination(w.address),
		Tag:     w.tag,
		Conn:    conn,
	}
	ctx = session.ContextWithInbound(ctx, inbound)
	entry, ok := w.sessions.register(&connection{Inbound: inbound, Network: net.Network_UNIX, Start: time.Now(), Conn: conn, Cancel: cancel})
	if !ok {
		common.CloseIfExists(conn)
		return
	}
	defer w.sessions.unregister(entry)

	content := new(session.Content)
	content.SniffingRequest = w.sniffingRequest
	ctx = session.ContextWithContent(ctx, content)

	if err := w.proxy.Process(ctx, net.Network_UNIX, conn, w.dispatcher); err != nil {
		errors.LogInfoInner(ctx, err, "connection ends")
	}
	if err := conn.Close(); err != nil {
		errors.LogInfoInner(ctx, err, "failed to close connection")
	}
}

func (w *dsWorker) Proxy() proxy.Inbound {
	return w.proxy
}

func (w *dsWorker) Port() net.Port {
	return net.Port(0)
}

func (w *dsWorker) Start() error {
	listener, err := hub.ListenUnix(w.ctx, w.address, w.stream, w.callback, hub.Options{})
	if err != nil {
		return errors.New("failed to listen Unix Domain Socket on ", w.address).Base(err)
	}
	w.hub = listener
	return nil
}

func (w *dsWorker) Close() error {
	if w.hub == nil {
		return nil
	}
	return hub.CloseAll(w.hub, w.proxy)
}

func IsLocal(ip net.IP) bool {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return false
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok {
			if ipnet.IP.Equal(ip) {
				return true
			}
		}
	}
	return false
}
