//go:build linux

package tun

// This test drives real packets through the gVisor Stack the moment it starts,
// which is what caught the startup race this change fixed: the device used to be
// attached before the transport handlers were registered. It is also how the
// marker below was measured.

import (
	"context"
	"errors"
	"net/netip"
	"testing"
	"time"

	"golang.org/x/sys/unix"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

// readFromFDUntil reads one packet from the test's end of the socket pair the
// device carries packets through
func readFromFDUntil(t *testing.T, fd int, timeout time.Duration) []byte {
	t.Helper()

	if err := unix.SetNonblock(fd, true); err != nil {
		t.Fatalf("make the test's end of the socket pair non-blocking: %v", err)
	}
	deadline := time.Now().Add(timeout)
	packet := make([]byte, 2048)
	for {
		n, err := unix.Read(fd, packet)
		if err == nil {
			return packet[:n]
		}
		if !errors.Is(err, unix.EAGAIN) {
			t.Fatalf("read the written packet: %v", err)
		}
		if time.Now().After(deadline) {
			t.Fatal("the stack wrote no packet")
		}
		time.Sleep(time.Millisecond)
	}
}

// TestGVisorStackAnswersSYNWithSYNACK is the gVisor Stack's counterpart of
// TestMipstackAnswersSYNWithSYNACK: it reports the marker the other Network
// Stack puts on the wire for the same handshake, so the two can be told apart
// in a real capture.
func TestGVisorStackAnswersSYNWithSYNACK(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	pair := newPacketSocket(t)
	// the device's end is non-blocking, like the descriptor a real device hands
	// to the stack
	if err := unix.SetNonblock(pair[1], true); err != nil {
		t.Fatalf("make the device's end of the socket pair non-blocking: %v", err)
	}
	device := &fakeFDTun{fds: []int{pair[1]}}
	handler := newRecordingConnectionHandler()

	stack, err := newGVisorStack(ctx, StackOptions{Tun: device, MTU: handshakeMTU, IdleTimeout: time.Minute}, handler)
	if err != nil {
		t.Fatalf("create the gVisor Stack: %v", err)
	}
	if err := stack.Start(); err != nil {
		t.Fatalf("start the gVisor Stack: %v", err)
	}
	defer stack.Close()

	client := netip.MustParseAddrPort("10.0.0.2:40000")
	server := netip.MustParseAddrPort("192.0.2.10:80")
	if _, err := unix.Write(pair[0], buildTCPSegment(t, client, server, header.TCPFlagSyn, 1, 0)); err != nil {
		t.Fatalf("queue the client's syn: %v", err)
	}

	ip, segment := parseSynAck(t, readFromFDUntil(t, pair[0], waitForPacketUntil), client, server)
	t.Logf("gvisor syn-ack: %s", synAckFingerprint(ip, segment))

	if _, err := unix.Write(pair[0], buildTCPSegment(t, client, server, header.TCPFlagAck, segment.AckNumber(), segment.SequenceNumber()+1)); err != nil {
		t.Fatalf("queue the client's acknowledgement: %v", err)
	}

	accepted := handler.next(t, 10*time.Second)
	if accepted.destination.NetAddr() != server.String() {
		t.Fatalf("the intercepted connection reports %s, want %s", accepted.destination.NetAddr(), server)
	}
	_ = accepted.connection.Close()
}
