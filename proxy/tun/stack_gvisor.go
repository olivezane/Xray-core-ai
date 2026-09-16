package tun

import (
	"context"
	"time"

	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv6"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/icmp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
	"gvisor.dev/gvisor/pkg/waiter"
)

const (
	defaultNIC tcpip.NICID = 1

	tcpRXBufMinSize = tcp.MinBufferSize
	tcpRXBufDefSize = tcp.DefaultSendBufferSize
	tcpRXBufMaxSize = 8 << 20 // 8MiB

	tcpTXBufMinSize = tcp.MinBufferSize
	tcpTXBufDefSize = tcp.DefaultReceiveBufferSize
	tcpTXBufMaxSize = 6 << 20 // 6MiB
)

var _ Stack = (*stackGVisor)(nil)

// stackGVisor is ip stack implemented by gVisor package
type stackGVisor struct {
	ctx         context.Context
	tun         Tun
	mtu         uint32
	idleTimeout time.Duration
	handler     ConnectionHandler
	stack       *stack.Stack
	endpoint    stack.LinkEndpoint
}

// newGVisorStack builds new ip stack (using gVisor)
func newGVisorStack(ctx context.Context, options StackOptions, handler ConnectionHandler) (*stackGVisor, error) {
	gStack := &stackGVisor{
		ctx:         ctx,
		tun:         options.Tun,
		mtu:         options.MTU,
		idleTimeout: options.IdleTimeout,
		handler:     handler,
	}

	// the counterpart of the MIPS Stack's own line, so a log tells which stack
	// is carrying the traffic
	errors.LogInfo(ctx, "[tun] ", StackGVisor, " network stack: mtu=", options.MTU)

	return gStack, nil
}

// Start is called by Handler to bring stack to life
func (t *stackGVisor) Start() error {
	linkEndpoint, err := newEndpoint(t.tun, t.mtu)
	if err != nil {
		return err
	}

	ipStack, err := newStack()
	if err != nil {
		return err
	}

	// the handlers close over the stack, and gVisor delivers nothing until a
	// NIC exists, so they are installed before the device is attached
	t.endpoint = linkEndpoint
	t.stack = ipStack
	t.installHandlers(ipStack)

	if err := attachNIC(ipStack, linkEndpoint); err != nil {
		return err
	}

	return nil
}

// installHandlers gives the stack the transport handlers of the TUN inbound. It
// has to run before the device is attached: gVisor documents
// SetTransportProtocolHandler as initialization-only, and the receive path is
// live as soon as the stack has a NIC.
func (t *stackGVisor) installHandlers(ipStack *stack.Stack) {
	tcpForwarder := tcp.NewForwarder(ipStack, 0, 65535, func(r *tcp.ForwarderRequest) {
		go func(r *tcp.ForwarderRequest) {
			var wq waiter.Queue
			id := r.ID()

			// Perform a TCP three-way handshake.
			ep, err := r.CreateEndpoint(&wq)
			if err != nil {
				errors.LogError(t.ctx, err.String())
				r.Complete(true)
				return
			}

			options := ep.SocketOptions()
			options.SetKeepAlive(false)
			options.SetReuseAddress(true)
			options.SetReusePort(true)

			t.handler.HandleConnection(
				gonet.NewTCPConn(&wq, ep),
				// local address on the gVisor side is connection destination
				net.TCPDestination(net.IPAddress(id.LocalAddress.AsSlice()), net.Port(id.LocalPort)),
			)

			// close the socket
			ep.Close()
			// send connection complete upstream
			r.Complete(false)
		}(r)
	})
	ipStack.SetTransportProtocolHandler(tcp.ProtocolNumber, tcpForwarder.HandlePacket)

	// Use custom UDP packet handler, instead of strict gVisor forwarder, for FullCone NAT support
	udpForwarder := newUdpConnectionHandler(t.handler.HandleConnection, t.writeRawUDPPacket)
	ipStack.SetTransportProtocolHandler(udp.ProtocolNumber, func(id stack.TransportEndpointID, pkt *stack.PacketBuffer) bool {
		data := pkt.Clone().Data().AsRange().ToSlice()
		// if len(data) == 0 {
		// 	return false
		// }
		// source/destination of the packet we process as incoming, on gVisor side are Remote/Local
		// in other terms, src is the side behind tun, dst is the side behind gVisor
		// this function handle packets passing from the tun to the gVisor, therefore the src/dst assignement
		srcIP := net.IPAddress(id.RemoteAddress.AsSlice())
		dstIP := net.IPAddress(id.LocalAddress.AsSlice())
		if srcIP == nil || dstIP == nil {
			panic(id)
		}
		src := net.UDPDestination(srcIP, net.Port(id.RemotePort))
		dst := net.UDPDestination(dstIP, net.Port(id.LocalPort))
		udpForwarder.HandlePacket(src, dst, data)
		return true
	})
	ipStack.SetTransportProtocolHandler(icmp.ProtocolNumber4, t.handleICMPv4Packet)
	ipStack.SetTransportProtocolHandler(icmp.ProtocolNumber6, t.handleICMPv6Packet)
}

func (t *stackGVisor) writeRawUDPPacket(payload []byte, src net.Destination, dst net.Destination) error {
	udpLen := header.UDPMinimumSize + len(payload)
	srcIP := tcpip.AddrFromSlice(src.Address.IP())
	dstIP := tcpip.AddrFromSlice(dst.Address.IP())

	// build packet with appropriate IP header size
	isIPv4 := dst.Address.Family().IsIPv4()
	ipHdrSize := header.IPv6MinimumSize
	ipProtocol := header.IPv6ProtocolNumber
	if isIPv4 {
		ipHdrSize = header.IPv4MinimumSize
		ipProtocol = header.IPv4ProtocolNumber
	}

	pkt := stack.NewPacketBuffer(stack.PacketBufferOptions{
		ReserveHeaderBytes: ipHdrSize + header.UDPMinimumSize,
		Payload:            buffer.MakeWithData(payload),
	})
	defer pkt.DecRef()

	// Build UDP header
	udpHdr := header.UDP(pkt.TransportHeader().Push(header.UDPMinimumSize))
	udpHdr.Encode(&header.UDPFields{
		SrcPort: uint16(src.Port),
		DstPort: uint16(dst.Port),
		Length:  uint16(udpLen),
	})

	// Calculate and set UDP checksum
	xsum := header.PseudoHeaderChecksum(header.UDPProtocolNumber, srcIP, dstIP, uint16(udpLen))
	udpHdr.SetChecksum(^udpHdr.CalculateChecksum(checksum.Checksum(payload, xsum)))

	// Build IP header
	if isIPv4 {
		ipHdr := header.IPv4(pkt.NetworkHeader().Push(header.IPv4MinimumSize))
		ipHdr.Encode(&header.IPv4Fields{
			TotalLength: uint16(header.IPv4MinimumSize + udpLen),
			TTL:         64,
			Protocol:    uint8(header.UDPProtocolNumber),
			SrcAddr:     srcIP,
			DstAddr:     dstIP,
		})
		ipHdr.SetChecksum(^ipHdr.CalculateChecksum())
	} else {
		ipHdr := header.IPv6(pkt.NetworkHeader().Push(header.IPv6MinimumSize))
		ipHdr.Encode(&header.IPv6Fields{
			PayloadLength:     uint16(udpLen),
			TransportProtocol: header.UDPProtocolNumber,
			HopLimit:          64,
			SrcAddr:           srcIP,
			DstAddr:           dstIP,
		})
	}

	// dispatch the packet
	err := t.stack.WriteRawPacket(defaultNIC, ipProtocol, buffer.MakeWithView(pkt.ToView()))
	if err != nil {
		return errors.New("failed to write raw udp packet back to stack", err)
	}

	return nil
}

// Close is called by Handler to shut down the stack
func (t *stackGVisor) Close() error {
	if t.stack == nil {
		return nil
	}
	t.endpoint.Attach(nil)
	t.stack.Close()
	for _, endpoint := range t.stack.CleanupEndpoints() {
		endpoint.Abort()
	}

	return nil
}

// newStack builds the gVisor Stack with its transport options. Nothing can be
// delivered to it yet: only a NIC makes the stack's receive path live.
func newStack() (*stack.Stack, error) {
	opts := stack.Options{
		NetworkProtocols:   []stack.NetworkProtocolFactory{ipv4.NewProtocol, ipv6.NewProtocol},
		TransportProtocols: []stack.TransportProtocolFactory{tcp.NewProtocol, udp.NewProtocol, icmp.NewProtocol4, icmp.NewProtocol6},
		HandleLocal:        false,
	}
	gStack := stack.New(opts)

	cOpt := tcpip.CongestionControlOption("cubic")
	gStack.SetTransportProtocolOption(tcp.ProtocolNumber, &cOpt)
	sOpt := tcpip.TCPSACKEnabled(true)
	gStack.SetTransportProtocolOption(tcp.ProtocolNumber, &sOpt)
	mOpt := tcpip.TCPModerateReceiveBufferOption(true)
	gStack.SetTransportProtocolOption(tcp.ProtocolNumber, &mOpt)

	// Disable RACK/TLP loss recovery to fix connection stalls under high load
	rOpt := tcpip.TCPRecovery(0)
	gStack.SetTransportProtocolOption(tcp.ProtocolNumber, &rOpt)

	tcpRXBufOpt := tcpip.TCPReceiveBufferSizeRangeOption{
		Min:     tcpRXBufMinSize,
		Default: tcpRXBufDefSize,
		Max:     tcpRXBufMaxSize,
	}
	err := gStack.SetTransportProtocolOption(tcp.ProtocolNumber, &tcpRXBufOpt)
	if err != nil {
		return nil, errors.New(err.String())
	}

	tcpTXBufOpt := tcpip.TCPSendBufferSizeRangeOption{
		Min:     tcpTXBufMinSize,
		Default: tcpTXBufDefSize,
		Max:     tcpTXBufMaxSize,
	}
	err = gStack.SetTransportProtocolOption(tcp.ProtocolNumber, &tcpTXBufOpt)
	if err != nil {
		return nil, errors.New(err.String())
	}

	return gStack, nil
}

// attachNIC gives the stack the device it delivers packets through. This is the
// moment the receive path goes live, so every handler the TUN inbound installs
// is registered before it.
func attachNIC(gStack *stack.Stack, ep stack.LinkEndpoint) error {
	if err := gStack.CreateNIC(defaultNIC, ep); err != nil {
		return errors.New(err.String())
	}

	gStack.SetRouteTable([]tcpip.Route{
		{Destination: header.IPv4EmptySubnet, NIC: defaultNIC},
		{Destination: header.IPv6EmptySubnet, NIC: defaultNIC},
	})

	if err := gStack.SetSpoofing(defaultNIC, true); err != nil {
		return errors.New(err.String())
	}
	if err := gStack.SetPromiscuousMode(defaultNIC, true); err != nil {
		return errors.New(err.String())
	}

	return nil
}
