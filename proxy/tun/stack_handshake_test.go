package tun

import (
	"fmt"
	"net/netip"
	"strconv"
	"testing"
	"time"

	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

// buildTCPSegment builds one IPv4 TCP segment of the client's half of a
// handshake with a destination a Network Stack intercepts
func buildTCPSegment(t *testing.T, client netip.AddrPort, server netip.AddrPort, flags header.TCPFlags, sequence, acknowledgement uint32) []byte {
	t.Helper()

	packet := make([]byte, header.IPv4MinimumSize+header.TCPMinimumSize)
	ip := header.IPv4(packet)
	ip.Encode(&header.IPv4Fields{
		TotalLength: uint16(len(packet)),
		TTL:         64,
		Protocol:    uint8(header.TCPProtocolNumber),
		SrcAddr:     tcpip.AddrFrom4(client.Addr().As4()),
		DstAddr:     tcpip.AddrFrom4(server.Addr().As4()),
	})
	ip.SetChecksum(^ip.CalculateChecksum())

	segment := header.TCP(packet[header.IPv4MinimumSize:])
	segment.Encode(&header.TCPFields{
		SrcPort:    client.Port(),
		DstPort:    server.Port(),
		SeqNum:     sequence,
		AckNum:     acknowledgement,
		DataOffset: header.TCPMinimumSize,
		Flags:      flags,
		WindowSize: 65535,
	})
	segment.SetChecksum(^segment.CalculateChecksum(header.PseudoHeaderChecksum(header.TCPProtocolNumber, ip.SourceAddress(), ip.DestinationAddress(), uint16(len(segment)))))

	return packet
}

// synAckFingerprint describes the answer a Network Stack puts on the wire for
// an intercepted handshake. Two stacks differ here, so a capture that matches
// one of these lines tells which stack answered it.
func synAckFingerprint(ip header.IPv4, segment header.TCP) string {
	options := header.ParseSynOptions(segment.Options(), true)
	wscale := "none"
	if options.WS >= 0 {
		wscale = strconv.Itoa(options.WS)
	}

	return fmt.Sprintf("ttl=%d window=%d wscale=%s sack=%t timestamps=%t mss=%d",
		ip.TTL(), segment.WindowSize(), wscale, options.SACKPermitted, options.TS, options.MSS)
}

// parseSynAck checks that the packet is the answer to the given handshake and
// returns it decoded
func parseSynAck(t *testing.T, packet []byte, client, server netip.AddrPort) (header.IPv4, header.TCP) {
	t.Helper()

	if len(packet) < header.IPv4MinimumSize+header.TCPMinimumSize {
		t.Fatalf("the stack answered %d bytes, too short to be a tcp segment", len(packet))
	}
	ip := header.IPv4(packet)
	if ip.Protocol() != uint8(header.TCPProtocolNumber) {
		t.Fatalf("the stack answered with ip protocol %d, want tcp", ip.Protocol())
	}
	if !ip.IsChecksumValid() {
		t.Fatal("the stack answered with an invalid ip checksum")
	}
	if ip.SourceAddress() != tcpip.AddrFrom4(server.Addr().As4()) || ip.DestinationAddress() != tcpip.AddrFrom4(client.Addr().As4()) {
		t.Fatalf("the answer came from %s to %s, want %s to %s", ip.SourceAddress(), ip.DestinationAddress(), server.Addr(), client.Addr())
	}

	segment := header.TCP(ip.Payload())
	flags := segment.Flags()
	if flags&header.TCPFlagSyn == 0 || flags&header.TCPFlagAck == 0 {
		t.Fatalf("the answer flags are %b, want syn+ack", flags)
	}
	if segment.SourcePort() != server.Port() || segment.DestinationPort() != client.Port() {
		t.Fatalf("the answer ports are %d -> %d, want %d -> %d", segment.SourcePort(), segment.DestinationPort(), server.Port(), client.Port())
	}
	if !segment.IsChecksumValid(ip.SourceAddress(), ip.DestinationAddress(), 0, 0) {
		t.Fatal("the stack answered with an invalid tcp checksum")
	}

	return ip, segment
}

// handshakeMTU is the MTU both stacks are built with when their handshake
// answers are compared
const handshakeMTU = 1500

// mtuMSS is the segment size an answer to a handshake of the given MTU offers
func mtuMSS(mtu uint32) uint16 {
	return uint16(mtu - header.IPv4MinimumSize - header.TCPMinimumSize)
}

// waitForPacketUntil gives up on a packet the test never receives
const waitForPacketUntil = 10 * time.Second
