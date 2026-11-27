package outbound

import (
	"context"
	"crypto/rand"
	goerrors "errors"
	"io"
	"math/big"
	"time"

	"github.com/xtls/xray-core/common/dice"

	"github.com/xtls/xray-core/app/proxyman"
	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/buf"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/serial"
	"github.com/xtls/xray-core/common/session"
	"github.com/xtls/xray-core/common/tracker"
	"github.com/xtls/xray-core/core"
	finbound "github.com/xtls/xray-core/features/inbound"
	"github.com/xtls/xray-core/features/outbound"
	"github.com/xtls/xray-core/features/policy"
	"github.com/xtls/xray-core/features/stats"
	"github.com/xtls/xray-core/proxy"
	"github.com/xtls/xray-core/transport"
	"github.com/xtls/xray-core/transport/internet"
	"github.com/xtls/xray-core/transport/internet/stat"
	"google.golang.org/protobuf/proto"
)

func getStatCounter(v *core.Instance, tag string) (stats.Counter, stats.Counter) {
	var uplinkCounter stats.Counter
	var downlinkCounter stats.Counter

	policy := v.GetFeature(policy.ManagerType()).(policy.Manager)
	if len(tag) > 0 && policy.ForSystem().Stats.OutboundUplink {
		statsManager := v.GetFeature(stats.ManagerType()).(stats.Manager)
		name := "outbound>>>" + tag + ">>>traffic>>>uplink"
		c, _ := statsManager.GetOrRegisterCounter(name)
		if c != nil {
			uplinkCounter = c
		}
	}
	if len(tag) > 0 && policy.ForSystem().Stats.OutboundDownlink {
		statsManager := v.GetFeature(stats.ManagerType()).(stats.Manager)
		name := "outbound>>>" + tag + ">>>traffic>>>downlink"
		c, _ := statsManager.GetOrRegisterCounter(name)
		if c != nil {
			downlinkCounter = c
		}
	}

	return uplinkCounter, downlinkCounter
}

// Handler implements outbound.Handler.
type Handler struct {
	// conns is the connection table of the instance, where every connection this
	// handler dials is registered until it is closed. It is nil in a bare handler
	// without an instance.
	conns           *tracker.Registry
	tag             string
	senderSettings  *proxyman.SenderConfig
	streamSettings  *internet.MemoryStreamConfig
	proxyConfig     proto.Message
	proxy           proxy.Outbound
	uplinkCounter   stats.Counter
	downlinkCounter stats.Counter
}

// NewHandler creates a new Handler based on the given configuration.
func NewHandler(ctx context.Context, config *core.OutboundHandlerConfig) (outbound.Handler, error) {
	v := core.MustFromContext(ctx)
	uplinkCounter, downlinkCounter := getStatCounter(v, config.Tag)
	h := &Handler{
		tag:             config.Tag,
		uplinkCounter:   uplinkCounter,
		downlinkCounter: downlinkCounter,
	}

	// The instance's connection table, where this handler registers every
	// connection it dials.
	if manager, ok := v.GetFeature(finbound.ManagerType()).(interface {
		Connections() *tracker.Registry
	}); ok {
		h.conns = manager.Connections()
	}

	if config.SenderSettings != nil {
		senderSettings, err := config.SenderSettings.GetInstance()
		if err != nil {
			return nil, err
		}
		switch s := senderSettings.(type) {
		case *proxyman.SenderConfig:
			h.senderSettings = s
			mss, err := internet.ToMemoryStreamConfig(s.StreamSettings)
			if err != nil {
				return nil, errors.New("failed to parse stream settings").Base(err)
			}
			h.streamSettings = mss
		default:
			return nil, errors.New("settings is not SenderConfig")
		}
	}

	proxyConfig, err := config.ProxySettings.GetInstance()
	if err != nil {
		return nil, err
	}
	h.proxyConfig = proxyConfig

	ctx = session.ContextWithFullHandler(ctx, h)

	if h.streamSettings != nil {
		ctx = session.ContextWithStreamSettings(ctx, h.streamSettings)
	}

	rawProxyHandler, err := common.CreateObject(ctx, proxyConfig)
	if err != nil {
		return nil, err
	}

	proxyHandler, ok := rawProxyHandler.(proxy.Outbound)
	if !ok {
		return nil, errors.New("not an outbound handler")
	}

	h.proxy = proxyHandler
	return h, nil
}

// Tag implements outbound.Handler.
func (h *Handler) Tag() string {
	return h.tag
}

// Dispatch implements proxy.Outbound.Dispatch.
func (h *Handler) Dispatch(ctx context.Context, link *transport.Link) {
	outbounds := session.OutboundsFromContext(ctx)
	ob := outbounds[len(outbounds)-1]
	if h.senderSettings != nil && h.senderSettings.TargetStrategy.HasStrategy() && ob.Target.Address.Family().IsDomain() {
		if session.SkipDNSResolveFromContext(ctx) {
			errors.LogInfo(ctx, "bypassing targetStrategy ", h.senderSettings.TargetStrategy, " for ", ob.Target, "; preserving target domain")
		} else {
			strategy := h.senderSettings.TargetStrategy
			if ob.Target.Network == net.Network_UDP && ob.OriginalTarget.Address != nil {
				strategy = strategy.GetDynamicStrategy(ob.OriginalTarget.Address.Family())
			}
			ips, err := internet.LookupForIP(ob.Target.Address.Domain(), strategy, nil)
			if err != nil {
				errors.LogInfoInner(ctx, err, "failed to resolve ip for target ", ob.Target.Address.Domain())
				if h.senderSettings.TargetStrategy.ForceIP() {
					err := errors.New("failed to resolve ip for target ", ob.Target.Address.Domain()).Base(err)
					session.SubmitOutboundErrorToOriginator(ctx, err)
					common.Interrupt(link.Writer)
					common.Interrupt(link.Reader)
					return
				}
			} else {
				unchangedDomain := ob.Target.Address.Domain()
				ob.Target.Address = net.IPAddress(ips[dice.Roll(len(ips))])
				errors.LogInfo(ctx, "target: ", unchangedDomain, " resolved to: ", ob.Target.Address.String())
			}
		}
	}
	reader, writer := link.Reader, link.Writer
	if ob.Target.Network == net.Network_UDP && ob.OriginalTarget.Address != nil && ob.OriginalTarget.Address != ob.Target.Address {
		link.Reader = &buf.EndpointOverrideReader{Reader: link.Reader, Dest: ob.Target.Address, OriginalDest: ob.OriginalTarget.Address}
		link.Writer = &buf.EndpointOverrideWriter{Writer: link.Writer, Dest: ob.Target.Address, OriginalDest: ob.OriginalTarget.Address}
	}

	err := h.proxy.Process(ctx, link, h)
	var completion interface{ DeferWriterClose() bool }
	if goerrors.As(err, &completion) && completion.DeferWriterClose() {
		go common.Close(writer)
		common.Interrupt(reader)
		return
	}
	var errC error
	if err != nil {
		errC = errors.Cause(err)
		if goerrors.Is(errC, io.EOF) || goerrors.Is(errC, io.ErrClosedPipe) || goerrors.Is(errC, context.Canceled) {
			err = nil
		}
	}
	if err != nil {
		// Ensure outbound ray is properly closed.
		err := errors.New("failed to process outbound traffic").Base(err)
		session.SubmitOutboundErrorToOriginator(ctx, err)
		errors.LogInfo(ctx, err.Error())
		common.Interrupt(writer)
	} else {
		if errC != nil && goerrors.Is(errC, io.ErrClosedPipe) {
			common.Interrupt(writer)
		} else {
			common.Close(writer)
		}
	}
	common.Interrupt(reader)
}

func (h *Handler) DestIpAddress() net.IP {
	return internet.DestIpAddress()
}

// Dial implements internet.Dialer.
func (h *Handler) Dial(ctx context.Context, dest net.Destination) (stat.Connection, error) {
	if h.senderSettings != nil && h.senderSettings.Via != nil {
		outbounds := session.OutboundsFromContext(ctx)
		ob := outbounds[len(outbounds)-1]
		h.SetOutboundGateway(ctx, ob)
	}

	conn, err := internet.Dial(ctx, dest, h.streamSettings)
	// The statistics wrapper stays outermost, so that a caller can still see it;
	// the tracking wrapper goes inside it and removes the entry when the
	// connection is closed.
	conn = h.trackConnection(conn, dest)
	return h.getStatCouterConnection(conn), err
}

func (h *Handler) SetOutboundGateway(ctx context.Context, ob *session.Outbound) {
	if ob.Gateway == nil && h.senderSettings != nil && h.senderSettings.Via != nil &&
		(h.streamSettings.SocketSettings == nil || len(h.streamSettings.SocketSettings.DialerProxy) == 0) {
		var domain string
		addr := h.senderSettings.Via.AsAddress()
		domain = h.senderSettings.Via.GetDomain()
		switch {
		case h.senderSettings.ViaCidr != "":
			ob.Gateway = ParseRandomIP(addr, h.senderSettings.ViaCidr)
		case domain == "origin":
			if inbound := session.InboundFromContext(ctx); inbound != nil {
				if inbound.Local.IsValid() && inbound.Local.Address.Family().IsIP() {
					ob.Gateway = inbound.Local.Address
					errors.LogDebug(ctx, "use inbound local ip as sendthrough: ", inbound.Local.Address.String())
				}
			}
		case domain == "srcip":
			if inbound := session.InboundFromContext(ctx); inbound != nil {
				if inbound.Source.IsValid() && inbound.Source.Address.Family().IsIP() {
					ob.Gateway = inbound.Source.Address
					errors.LogDebug(ctx, "use inbound source ip as sendthrough: ", inbound.Source.Address.String())
				}
			}
		default: // case addr.Family().IsDomain():
			ob.Gateway = addr
		}
	}
}

// trackConnection registers a dialed connection in the connection table of the
// instance, so that it can be reported and closed with the connections of the
// inbounds it serves. The table entry goes away when the connection is closed.
func (h *Handler) trackConnection(conn stat.Connection, dest net.Destination) stat.Connection {
	if h.conns == nil || conn == nil {
		return conn
	}
	entry := &tracker.Entry{
		Owner:       h,
		Direction:   tracker.Outbound,
		OutboundTag: h.tag,
		Destination: dest,
		Network:     dest.Network,
		Start:       time.Now(),
		Conn:        conn,
	}
	if !h.conns.Register(entry) {
		// The instance is closing: do not hand out a connection nobody will close.
		common.CloseIfExists(conn)
		return conn
	}
	return &trackedConn{Connection: conn, registry: h.conns}
}

// trackedConn removes itself from the connection table when it is closed, so a
// finished connection does not stay in a snapshot.
type trackedConn struct {
	stat.Connection
	registry *tracker.Registry
}

// Unwrap exposes the connection behind the tracking wrapper, so that code which
// inspects the raw connection still sees it.
func (c *trackedConn) Unwrap() net.Conn {
	return c.Connection
}

func (c *trackedConn) Close() error {
	c.registry.Unregister(c.Connection)
	return c.Connection.Close()
}

func (h *Handler) getStatCouterConnection(conn stat.Connection) stat.Connection {
	if h.uplinkCounter != nil || h.downlinkCounter != nil {
		return &stat.CounterConnection{
			Connection:   conn,
			ReadCounter:  h.downlinkCounter,
			WriteCounter: h.uplinkCounter,
		}
	}
	return conn
}

// GetOutbound implements proxy.GetOutbound.
func (h *Handler) GetOutbound() proxy.Outbound {
	return h.proxy
}

// Start implements common.Runnable.
func (h *Handler) Start() error {
	return nil
}

// Close implements common.Closable.
func (h *Handler) Close() error {
	common.Close(h.proxy)
	return nil
}

// SenderSettings implements outbound.Handler.
func (h *Handler) SenderSettings() *serial.TypedMessage {
	return serial.ToTypedMessage(h.senderSettings)
}

// ProxySettings implements outbound.Handler.
func (h *Handler) ProxySettings() *serial.TypedMessage {
	return serial.ToTypedMessage(h.proxyConfig)
}

func ParseRandomIP(addr net.Address, prefix string) net.Address {
	_, ipnet, _ := net.ParseCIDR(addr.IP().String() + "/" + prefix)

	ones, bits := ipnet.Mask.Size()
	subnetSize := new(big.Int).Lsh(big.NewInt(1), uint(bits-ones))

	rnd, _ := rand.Int(rand.Reader, subnetSize)

	startInt := new(big.Int).SetBytes(ipnet.IP)
	rndInt := new(big.Int).Add(startInt, rnd)

	rndBytes := rndInt.Bytes()
	padded := make([]byte, len(ipnet.IP))
	copy(padded[len(padded)-len(rndBytes):], rndBytes)

	return net.ParseAddress(net.IP(padded).String())
}
