//go:build linux || darwin || freebsd

package tun

import (
	"bytes"
	"context"
	"errors"
	"net/netip"
	"testing"
	"time"

	"golang.org/x/sys/unix"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

// newPacketSocket returns a datagram socket pair, the closest stand-in for a
// TUN descriptor: both ends carry whole packets and both are readable and
// writable. The first element is the test's end, the second the device's.
func newPacketSocket(t *testing.T) [2]int {
	t.Helper()
	pair, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_DGRAM, 0)
	if err != nil {
		t.Fatalf("create socket pair: %v", err)
	}
	t.Cleanup(func() {
		unix.Close(pair[0])
		unix.Close(pair[1])
	})
	return pair
}

// framedFDTun is a Tun whose packets flow through a Packet Device the test
// builds by hand, so a stack can be driven through a framing the platform
// itself cannot be asked for on the machine running the test.
type framedFDTun struct {
	fakeTun
	device *fdPacketDevice
}

// packetDevice implements tunPacketDevice: the test builds the device by hand,
// so it hands that one over unchanged
func (f *framedFDTun) packetDevice() (packetDevice, error) { return f.device, nil }

// TestFDPacketDeviceStripsAndAddsTheUnixHeader covers the Darwin and FreeBSD
// framing, where the device puts a Unix address family header in front of every
// packet the stack neither wants on the way in nor knows to write on the way
// out. The device is built by hand with a header room, since the descriptor
// this test can open carries bare packets on Linux.
func TestFDPacketDeviceStripsAndAddsTheUnixHeader(t *testing.T) {
	pair := newPacketSocket(t)
	device := &fdPacketDevice{fd: pair[1], headerRoom: 4}

	ipv4 := []byte{0x45, 0x00, 0x00, 0x14, 0xde, 0xad, 0xbe, 0xef}

	// inbound: the device header has to be gone from the packet the stack sees
	framed := append([]byte{0, 0, 0, unix.AF_INET}, ipv4...)
	if _, err := unix.Write(pair[0], framed); err != nil {
		t.Fatalf("queue packet: %v", err)
	}
	buffers := [][]byte{make([]byte, 64)}
	count, err := device.ReadPackets(buffers)
	if err != nil {
		t.Fatalf("unexpected read error: %v", err)
	}
	if count != 1 {
		t.Fatalf("read %d packets, want 1", count)
	}
	if !bytes.Equal(buffers[0][:len(ipv4)], ipv4) {
		t.Fatalf("read % x, want the packet without its device header % x", buffers[0][:len(ipv4)], ipv4)
	}

	// a device header with no packet behind it is not a packet
	if _, err := unix.Write(pair[0], []byte{0, 0, 0, unix.AF_INET}); err != nil {
		t.Fatalf("queue header-only frame: %v", err)
	}
	if _, err := device.ReadPackets(buffers); !errors.Is(err, ErrQueueEmpty) {
		t.Fatalf("read error = %v, want %v for a header-only frame", err, ErrQueueEmpty)
	}

	// outbound: the device header has to be back in front of the packet, with
	// the address family the packet names in its last byte
	if err := device.WritePackets([][]byte{ipv4}); err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}
	out := make([]byte, 64)
	n, err := unix.Read(pair[0], out)
	if err != nil {
		t.Fatalf("read written packet: %v", err)
	}
	if n != len(ipv4)+4 {
		t.Fatalf("wrote %d bytes, want the %d byte packet behind a 4 byte header", n, len(ipv4))
	}
	if out[3] != unix.AF_INET {
		t.Fatalf("device header names address family %d, want AF_INET (%d)", out[3], unix.AF_INET)
	}
	if !bytes.Equal(out[4:n], ipv4) {
		t.Fatalf("wrote % x, want % x", out[4:n], ipv4)
	}

	// a packet naming no address family this device carries is dropped
	if err := device.WritePackets([][]byte{{0x00, 0x00, 0x00, 0x00}}); err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}
	// a packet with nothing behind its header is dropped too, rather than read
	// out of range while the family is looked for
	if err := device.WritePackets([][]byte{{}}); err != nil {
		t.Fatalf("unexpected write error for an empty packet: %v", err)
	}
	if err := unix.SetNonblock(pair[0], true); err != nil {
		t.Fatalf("make the test end non-blocking: %v", err)
	}
	if n, err := unix.Read(pair[0], out); !errors.Is(err, unix.EAGAIN) {
		t.Fatalf("read %d bytes (err %v) from a packet that names no address family, want an empty queue", n, err)
	}
}

// TestFDPacketDeviceDoesNotBlockOnABlockingDescriptor covers the FreeBSD and
// Darwin devices, which hand the stack a descriptor they left in blocking mode.
// Every read is preceded by a poll, so an empty device has to report an empty
// queue instead of parking the caller on the descriptor forever.
func TestFDPacketDeviceDoesNotBlockOnABlockingDescriptor(t *testing.T) {
	pair := newPacketSocket(t)
	// deliberately blocking, unlike the descriptor newPacketDevice used to force
	if err := unix.SetNonblock(pair[1], false); err != nil {
		t.Fatalf("make the device's end blocking: %v", err)
	}
	device := &fdPacketDevice{fd: pair[1]}

	done := make(chan error, 1)
	go func() {
		_, err := device.ReadPackets([][]byte{make([]byte, 64)})
		done <- err
	}()

	select {
	case err := <-done:
		if !errors.Is(err, ErrQueueEmpty) {
			t.Fatalf("read error = %v, want %v on an empty blocking device", err, ErrQueueEmpty)
		}
	case <-time.After(waitForPacketUntil):
		t.Fatal("reading an empty blocking device parked the caller instead of reporting an empty queue")
	}
}

// readFramedUntil takes one framed packet from the test's end of the socket
// pair, with the device header still in front of it
func readFramedUntil(t *testing.T, fd int, timeout time.Duration) []byte {
	t.Helper()

	if err := unix.SetNonblock(fd, true); err != nil {
		t.Fatalf("make the test end non-blocking: %v", err)
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

// TestMipstackReadsAndWritesFramedPacketsThroughTheFDDevice drives the real
// MIPS Stack through the fd Packet Device with a Unix address family header in
// front of every packet, the framing Darwin and FreeBSD devices use. It is the
// end-to-end counterpart of the framing test above: the stack has to see the
// bare packet, and the device has to put the header back on the answer.
func TestMipstackReadsAndWritesFramedPacketsThroughTheFDDevice(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	pair := newPacketSocket(t)
	device := &framedFDTun{device: &fdPacketDevice{fd: pair[1], headerRoom: 4}}
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
	defer stack.Close()

	client := netip.MustParseAddrPort("10.0.0.2:40000")
	server := netip.MustParseAddrPort("192.0.2.10:80")

	// the host queues its syn behind the device header, the way a utun device
	// hands it over
	syn := buildTCPSegment(t, client, server, header.TCPFlagSyn, 1, 0)
	if _, err := unix.Write(pair[0], append([]byte{0, 0, 0, unix.AF_INET}, syn...)); err != nil {
		t.Fatalf("queue the client's syn: %v", err)
	}

	answer := readFramedUntil(t, pair[0], waitForPacketUntil)
	if len(answer) < 4 {
		t.Fatalf("the stack wrote %d bytes, too short to carry the device header", len(answer))
	}
	if answer[3] != unix.AF_INET {
		t.Fatalf("the answer names address family %d, want AF_INET (%d)", answer[3], unix.AF_INET)
	}
	// the stack must have seen the bare packet, so its answer has to be a
	// well-formed syn-ack for that packet
	ip, segment := parseSynAck(t, answer[4:], client, server)
	t.Logf("mipstack through the framed fd device: %s", synAckFingerprint(ip, segment))

	// the client acknowledges the answer, which is what turns the handshake
	// into the connection the inbound intercepts
	ack := buildTCPSegment(t, client, server, header.TCPFlagAck, segment.AckNumber(), segment.SequenceNumber()+1)
	if _, err := unix.Write(pair[0], append([]byte{0, 0, 0, unix.AF_INET}, ack...)); err != nil {
		t.Fatalf("queue the client's acknowledgement: %v", err)
	}

	accepted := handler.next(t, 10*time.Second)
	if accepted.destination.NetAddr() != server.String() {
		t.Fatalf("the intercepted connection reports %s, want %s", accepted.destination.NetAddr(), server)
	}
	accepted.connection.Close()
}
