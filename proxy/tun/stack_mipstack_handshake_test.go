package tun

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"gvisor.dev/gvisor/pkg/tcpip/header"
)

// readWrittenPacket takes one packet a stack wrote to the test's bridge device
func readWrittenPacket(t *testing.T, device *bridgeDevice, timeout time.Duration) []byte {
	t.Helper()

	packets := make(chan []byte, 1)
	go func() {
		if packet, ok := device.drain(); ok {
			packets <- packet
		}
	}()

	select {
	case packet := <-packets:
		return packet
	case <-time.After(timeout):
		t.Fatal("the stack wrote no packet")
		return nil
	}
}

// TestMipstackAnswersSYNWithSYNACK pins the answer the stack puts on the wire
// for an intercepted TCP handshake, and reports the marker an operator can
// match a real capture against to tell which Network Stack is running.
func TestMipstackAnswersSYNWithSYNACK(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	device := newBridgeDevice()
	handler := newRecordingConnectionHandler()
	stack, err := newMipstack(ctx, StackOptions{
		Tun:         device,
		MTU:         handshakeMTU,
		IdleTimeout: time.Minute,
	}, handler)
	if err != nil {
		t.Fatalf("create the MIPS Stack: %v", err)
	}
	if err := stack.Start(); err != nil {
		t.Fatalf("start the MIPS Stack: %v", err)
	}
	defer device.close()
	defer stack.Close()

	client := netip.MustParseAddrPort("10.0.0.2:40000")
	server := netip.MustParseAddrPort("192.0.2.10:80")
	if !device.inject(buildTCPSegment(t, client, server, header.TCPFlagSyn, 1, 0)) {
		t.Fatal("the device is closed")
	}

	ip, segment := parseSynAck(t, readWrittenPacket(t, device, waitForPacketUntil), client, server)
	// the segment size is the one the configured MTU allows, not a constant
	if got, want := header.ParseSynOptions(segment.Options(), true).MSS, mtuMSS(handshakeMTU); got != want {
		t.Fatalf("the answer offers mss %d, want %d", got, want)
	}
	t.Logf("mipstack syn-ack: %s", synAckFingerprint(ip, segment))

	// the client acknowledges the answer, which is what turns the handshake
	// into the connection the inbound intercepts
	if !device.inject(buildTCPSegment(t, client, server, header.TCPFlagAck, segment.AckNumber(), segment.SequenceNumber()+1)) {
		t.Fatal("the device is closed")
	}

	accepted := handler.next(t, 10*time.Second)
	if accepted.destination.NetAddr() != server.String() {
		t.Fatalf("the intercepted connection reports %s, want %s", accepted.destination.NetAddr(), server)
	}
	_ = accepted.connection.Close()
}
