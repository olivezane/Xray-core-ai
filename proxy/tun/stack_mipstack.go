package tun

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/metacubex/mipstack"
	"github.com/xtls/xray-core/common"
	xerrors "github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
)

const (
	// mipstackBatchSize bounds how many packets one pump iteration moves
	mipstackBatchSize = 64
	// mipstackDefaultMTU is the MTU assumed when the configuration names none
	mipstackDefaultMTU = 1500
	// mipstackMaxResponderSweepInterval bounds how often idle UDP Responders are
	// released
	mipstackMaxResponderSweepInterval = 30 * time.Second
	// mipstackMinResponderSweepInterval keeps a short idle policy enforced promptly
	// without sweeping in a busy loop
	mipstackMinResponderSweepInterval = 10 * time.Millisecond
	// mipstackFallbackResponderIdle bounds a UDP Responder's life when the idle
	// policy of the user level is disabled
	mipstackFallbackResponderIdle = 5 * time.Minute
)

// mipstackUDPFlow identifies one intercepted UDP flow: the client endpoint and the
// remote endpoint the client addressed
type mipstackUDPFlow struct {
	client netip.AddrPort
	remote netip.AddrPort
}

// mipstackUDPResponder is the retained reply capability of one intercepted UDP
// flow, with the moment it was last used
type mipstackUDPResponder struct {
	responder  *mipstack.UDPForwarderResponder
	lastActive atomic.Int64
}

func (r *mipstackUDPResponder) touch() {
	r.lastActive.Store(time.Now().UnixNano())
}

func (r *mipstackUDPResponder) idleFor(now time.Time) time.Duration {
	return now.Sub(time.Unix(0, r.lastActive.Load()))
}

var _ Stack = (*stackMipstack)(nil)

// stackMipstack is the MIPS Stack: it carries the packets of a TUN device between
// the device and the mihomo user-space IP stack.
type stackMipstack struct {
	ctx         context.Context
	device      packetDevice
	stack       *mipstack.Stack
	handler     ConnectionHandler
	mtu         uint32
	idleTimeout time.Duration

	tcpForwarder  *mipstack.TCPForwarder
	udpForwarder  *mipstack.UDPForwarder
	icmpForwarder *mipstack.ICMPForwarder
	udpSessions   *udpConnectionHandler

	udpMu      sync.Mutex
	responders map[mipstackUDPFlow]*mipstackUDPResponder

	sweepStop chan struct{}
	closed    atomic.Bool
	closeOnce sync.Once
	pumps     sync.WaitGroup
}

// newMipstack builds the MIPS Stack for one TUN inbound
// newMipstack builds a MIPS Stack from options that SelectStack has already
// validated
func newMipstack(ctx context.Context, options StackOptions, handler ConnectionHandler) (Stack, error) {
	device, err := newPacketDevice(options.Tun)
	if err != nil {
		return nil, err
	}

	mtu := options.MTU
	if mtu == 0 {
		mtu = mipstackDefaultMTU
	}

	mips, err := mipstack.New(mipstack.Config{
		// the inbound intercepts whatever destination the device hands it, so
		// the stack admits nonlocal destinations and needs no Local Address of
		// its own beyond the ones the device configures
		LocalAddresses: localAddressesFromGateways(options.Gateway),
		Promiscuous:    true,
		MTU:            mtu,
		TCP: mipstack.TCPSocketDefaults{
			CongestionControl: options.TCPCongestion,
			IdleTimeout:       options.IdleTimeout,
		},
	})
	if err != nil {
		return nil, xerrors.New("failed to create the ", StackMipstack, " network stack").Base(err)
	}

	stack := &stackMipstack{
		ctx:         ctx,
		device:      device,
		stack:       mips,
		handler:     handler,
		mtu:         mtu,
		idleTimeout: options.IdleTimeout,
		responders:  make(map[mipstackUDPFlow]*mipstackUDPResponder),
		sweepStop:   make(chan struct{}),
	}

	// the intercepted client sessions are the ones the gVisor Stack already
	// uses: one session per client, replying from the address the client
	// addressed
	stack.udpSessions = newUdpConnectionHandler(handler.HandleConnection, stack.writeUDPReply)
	stack.udpSessions.SetOnFinished(stack.releaseResponders)

	forwarder, err := mipstack.NewTCPForwarder(mips, mipstack.TCPForwarderOptions{}, stack.handleTCP)
	if err != nil {
		_ = mips.Close()
		return nil, xerrors.New("failed to create the ", StackMipstack, " tcp interceptor").Base(err)
	}
	stack.tcpForwarder = forwarder

	udpForwarder, err := mipstack.NewUDPForwarder(mips, mipstack.UDPForwarderOptions{}, stack.handleUDP)
	if err != nil {
		_ = forwarder.Close()
		_ = mips.Close()
		return nil, xerrors.New("failed to create the ", StackMipstack, " udp interceptor").Base(err)
	}
	stack.udpForwarder = udpForwarder

	icmpForwarder, err := mipstack.NewICMPForwarder(mips, mipstack.ICMPForwarderOptions{}, stack.handleICMP)
	if err != nil {
		_ = udpForwarder.Close()
		_ = forwarder.Close()
		_ = mips.Close()
		return nil, xerrors.New("failed to create the ", StackMipstack, " icmp interceptor").Base(err)
	}
	stack.icmpForwarder = icmpForwarder

	// name the stack and the settings it is running with, so an operator can
	// tell from the log which stack carries their traffic
	congestion := options.TCPCongestion
	if congestion == "" {
		// mipstack reads an empty name as its default algorithm
		congestion = mipstack.CongestionControlCUBIC + " (default)"
	}
	xerrors.LogInfo(ctx, "[tun] ", StackMipstack, " network stack: mtu=", mtu,
		" tcp congestion=", congestion,
		" tcp idle timeout=", options.IdleTimeout,
		" gateway=", options.Gateway)

	return stack, nil
}

// Start brings the MIPS Stack up and starts moving packets
func (t *stackMipstack) Start() error {
	if err := t.stack.Start(); err != nil {
		return xerrors.New("failed to start the ", StackMipstack, " network stack").Base(err)
	}

	t.pumps.Add(3)
	go t.inboundPump()
	go t.outboundPump()
	go t.sweepResponders()

	return nil
}

// Close shuts the MIPS Stack down and stops moving packets
func (t *stackMipstack) Close() error {
	var err error
	t.closeOnce.Do(func() {
		t.closed.Store(true)
		close(t.sweepStop)
		err = xerrors.Combine(
			common.CloseIfExists(t.tcpForwarder),
			common.CloseIfExists(t.udpForwarder),
			common.CloseIfExists(t.icmpForwarder),
			common.CloseIfExists(t.stack),
		)
		t.pumps.Wait()

		t.udpMu.Lock()
		t.responders = make(map[mipstackUDPFlow]*mipstackUDPResponder)
		t.udpMu.Unlock()
	})
	return err
}

// handleTCP hands one intercepted TCP connection to the inbound
func (t *stackMipstack) handleTCP(request *mipstack.TCPForwarderRequest) {
	if t.closed.Load() {
		_ = request.Drop()
		return
	}

	flow := request.Flow()
	connection, err := request.Accept(t.ctx)
	if err != nil {
		// the peer gave up, or the stack is shutting down
		xerrors.LogDebug(t.ctx, "[tun] ", StackMipstack, " dropped an intercepted tcp connection: ", err)
		return
	}

	destination := net.TCPDestination(
		net.IPAddress(flow.Destination.Addr().Unmap().AsSlice()),
		net.Port(flow.Destination.Port()),
	)
	go t.handler.HandleConnection(connection, destination)
}

// handleUDP hands one intercepted datagram to the client session it belongs to
func (t *stackMipstack) handleUDP(request *mipstack.UDPForwarderRequest) {
	if t.closed.Load() {
		_ = request.Drop()
		return
	}

	flow := request.Flow()
	client, ok := mipstackAddrPort(flow.Source)
	if !ok {
		_ = request.Drop()
		return
	}
	remote, ok := mipstackAddrPort(flow.Destination)
	if !ok {
		_ = request.Drop()
		return
	}

	key := mipstackUDPFlow{client: client, remote: remote}
	t.udpMu.Lock()
	entry, known := t.responders[key]
	t.udpMu.Unlock()

	if known {
		entry.touch()
	} else {
		// a UDP Responder answers one flow, so a client addressing several
		// remotes keeps one responder per flow
		responder, err := request.DetachForReplies()
		if err != nil {
			xerrors.LogDebug(t.ctx, "[tun] ", StackMipstack, " dropped an intercepted udp datagram: ", err)
			return
		}

		t.udpMu.Lock()
		if existing, ok := t.responders[key]; ok {
			entry = existing
		} else {
			entry = &mipstackUDPResponder{responder: responder}
			entry.touch()
			t.responders[key] = entry
		}
		t.udpMu.Unlock()
	}

	// the payload of the interception call is only valid until the handler
	// returns, while the session reads it later
	payload := append([]byte(nil), request.Payload()...)
	t.udpSessions.HandlePacket(mipstackUDPDestination(client), mipstackUDPDestination(remote), payload)
}

// handleICMP answers the intercepted echo requests, as the gVisor Stack does,
// and consumes every other message
func (t *stackMipstack) handleICMP(request *mipstack.ICMPForwarderRequest) {
	if t.closed.Load() {
		_ = request.Drop()
		return
	}

	message, err := request.Message().ICMPMessage()
	if err != nil {
		_ = request.Drop()
		return
	}
	if !message.IsEchoRequest() {
		_ = request.Drop()
		return
	}

	if err := request.ReplyEcho(); err != nil {
		xerrors.LogInfoInner(t.ctx, err, "[tun] failed to write local icmp echo reply")
	}
}

// writeUDPReply sends one reply datagram back to the client from the address
// the client addressed
func (t *stackMipstack) writeUDPReply(payload []byte, source net.Destination, client net.Destination) error {
	remote, ok := mipstackAddrPortOfDestination(source)
	if !ok {
		return xerrors.New("invalid udp reply source ", source.NetAddr())
	}
	clientAddress, ok := mipstackAddrPortOfDestination(client)
	if !ok {
		return xerrors.New("invalid udp reply destination ", client.NetAddr())
	}

	t.udpMu.Lock()
	entry := t.responders[mipstackUDPFlow{client: clientAddress, remote: remote}]
	t.udpMu.Unlock()
	if entry == nil {
		// the responder is released once the client session ends or the flow
		// outlives Connection Idle, and a reply that arrives after that leaves
		// no mapping for this datagram: the client sees it dropped, while the
		// session lives on to serve the datagrams the client sends next
		xerrors.LogDebug(t.ctx, "[tun] ", StackMipstack, " dropped a udp reply for the released flow ", source.NetAddr(), " -> ", client.NetAddr())
		return nil
	}
	entry.touch()

	if _, err := entry.responder.ReplyFrom(payload, remote); err != nil {
		return xerrors.New("failed to reply to the udp flow ", source.NetAddr(), " -> ", client.NetAddr()).Base(err)
	}

	return nil
}

// releaseResponders drops the UDP Responders of a client session that ended
func (t *stackMipstack) releaseResponders(client net.Destination) {
	address, ok := mipstackAddrPortOfDestination(client)
	if !ok {
		return
	}

	t.udpMu.Lock()
	for flow := range t.responders {
		if flow.client == address {
			delete(t.responders, flow)
		}
	}
	t.udpMu.Unlock()
}

// sweepResponders releases the UDP Responders that outlived the idle policy of
// their user level
func (t *stackMipstack) sweepResponders() {
	defer t.pumps.Done()

	ticker := time.NewTicker(t.responderSweepInterval())
	defer ticker.Stop()

	for {
		select {
		case <-t.sweepStop:
			return
		case now := <-ticker.C:
			timeout := t.responderIdleTimeout()
			t.udpMu.Lock()
			for flow, entry := range t.responders {
				if entry.idleFor(now) > timeout {
					delete(t.responders, flow)
				}
			}
			t.udpMu.Unlock()
		}
	}
}

// responderIdleTimeout is how long an unused UDP Responder is kept
func (t *stackMipstack) responderIdleTimeout() time.Duration {
	if t.idleTimeout > 0 {
		return t.idleTimeout
	}
	return mipstackFallbackResponderIdle
}

// responderSweepInterval is how often idle UDP Responders are released: half
// the idle bound, so a short policy is enforced promptly and a long one is not
// swept needlessly often
func (t *stackMipstack) responderSweepInterval() time.Duration {
	interval := t.responderIdleTimeout() / 2
	if interval > mipstackMaxResponderSweepInterval {
		return mipstackMaxResponderSweepInterval
	}
	if interval < mipstackMinResponderSweepInterval {
		return mipstackMinResponderSweepInterval
	}
	return interval
}

// inboundPump moves packets from the device into the stack
func (t *stackMipstack) inboundPump() {
	defer t.pumps.Done()

	packets := newPacketBatch(t.mtu)
	for !t.closed.Load() {
		count, err := t.device.ReadPackets(packets)
		if errors.Is(err, ErrQueueEmpty) {
			t.device.Wait()
			continue
		}
		if err != nil {
			if !t.closed.Load() {
				xerrors.LogInfoInner(t.ctx, err, "[tun] "+StackMipstack+" stopped reading packets")
			}
			return
		}

		// the stack parses complete packets of its own framing, so the byte
		// length the device reported is all it needs
		if _, err := t.stack.Write(packets[:count], 0); err != nil {
			if !t.closed.Load() {
				xerrors.LogInfoInner(t.ctx, err, "[tun] "+StackMipstack+" refused packets")
			}
			return
		}
	}
}

// outboundPump moves packets from the stack back to the device
func (t *stackMipstack) outboundPump() {
	defer t.pumps.Done()

	packets := newPacketBatch(t.mtu)
	sizes := make([]int, len(packets))
	batch := make([][]byte, 0, len(packets))

	for !t.closed.Load() {
		count, err := t.stack.Read(packets, sizes, 0)
		if err != nil {
			if !t.closed.Load() {
				xerrors.LogInfoInner(t.ctx, err, "[tun] "+StackMipstack+" stopped producing packets")
			}
			return
		}

		batch = batch[:0]
		for i := 0; i < count; i++ {
			batch = append(batch, packets[i][:sizes[i]])
		}
		if err := t.device.WritePackets(batch); err != nil {
			if !t.closed.Load() {
				xerrors.LogInfoInner(t.ctx, err, "[tun] "+StackMipstack+" could not write packets")
			}
			return
		}
	}
}

// newPacketBatch allocates the per-packet buffers of one pump iteration
func newPacketBatch(mtu uint32) [][]byte {
	packets := make([][]byte, mipstackBatchSize)
	for i := range packets {
		packets[i] = make([]byte, mtu)
	}
	return packets
}

// mipstackAddrPort normalizes one endpoint of an intercepted flow
func mipstackAddrPort(address netip.AddrPort) (netip.AddrPort, bool) {
	addr := address.Addr().Unmap()
	if !addr.IsValid() {
		return netip.AddrPort{}, false
	}
	return netip.AddrPortFrom(addr, address.Port()), true
}

// mipstackAddrPortOfDestination converts one endpoint of a reply back into a
// comparable address
func mipstackAddrPortOfDestination(destination net.Destination) (netip.AddrPort, bool) {
	address := destination.Address
	if address == nil || address.Family().IsDomain() {
		return netip.AddrPort{}, false
	}
	addr, ok := netip.AddrFromSlice(address.IP())
	if !ok {
		return netip.AddrPort{}, false
	}
	return netip.AddrPortFrom(addr.Unmap(), uint16(destination.Port)), true
}

// mipstackUDPDestination converts an intercepted endpoint into an inbound
// destination
func mipstackUDPDestination(address netip.AddrPort) net.Destination {
	return net.UDPDestination(net.IPAddress(address.Addr().AsSlice()), net.Port(address.Port()))
}

// localAddressesFromGateways turns the configured gateway prefixes into the
// Local Addresses of the MIPS Stack. Gateways the device itself rejects are
// skipped, and no Local Address is invented.
func localAddressesFromGateways(gateways []string) []netip.Prefix {
	var addresses []netip.Prefix
	for _, gateway := range gateways {
		gateway = strings.TrimSpace(gateway)
		if gateway == "" {
			continue
		}
		if prefix, err := netip.ParsePrefix(gateway); err == nil {
			addresses = append(addresses, prefix.Masked())
			continue
		}
		if address, err := netip.ParseAddr(gateway); err == nil {
			addresses = append(addresses, netip.PrefixFrom(address, address.BitLen()))
		}
	}
	return addresses
}

// normalizeTCPCongestion maps a configured congestion control name onto the
// name the MIPS Stack expects
func normalizeTCPCongestion(congestion string) (string, error) {
	switch name := strings.ToLower(strings.TrimSpace(congestion)); name {
	case "", mipstack.CongestionControlCUBIC:
		return mipstack.CongestionControlCUBIC, nil
	case mipstack.CongestionControlBBR, mipstack.CongestionControlBBR3, mipstack.CongestionControlReno:
		return name, nil
	default:
		return "", xerrors.New("unknown tcp congestion control: ", congestion)
	}
}
