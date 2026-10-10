//go:build linux

package tun

import (
	"bytes"
	"errors"
	"testing"

	"golang.org/x/sys/unix"
)

// TestFDPacketDeviceStripsAndAddsTheUnixHeader covers the Darwin and FreeBSD
// path, where the device frames every packet with a Unix address family header
// the stack neither wants on the way in nor knows to write on the way out. The
// device is built by hand with a header room, since the Linux descriptor this
// test can open carries bare packets.
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
