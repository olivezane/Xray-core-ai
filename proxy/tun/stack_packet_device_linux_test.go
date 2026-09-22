//go:build linux

package tun

import (
	"bytes"
	"errors"
	"testing"

	"golang.org/x/sys/unix"
	"gvisor.dev/gvisor/pkg/tcpip/link/fdbased"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

// fakeFDTun is a device whose packets flow through a plain file descriptor,
// like the Linux and Android devices.
type fakeFDTun struct {
	fakeTun
	fds []int
	mtu uint32
}

func (f *fakeFDTun) FDs() []int { return f.fds }

func (f *fakeFDTun) newEndpoint() (stack.LinkEndpoint, error) {
	return fdbased.New(&fdbased.Options{
		FDs:               f.fds,
		MTU:               f.mtu,
		RXChecksumOffload: true,
	})
}

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
		_ = unix.Close(pair[0])
		_ = unix.Close(pair[1])
	})
	return pair
}

func TestFDPacketDeviceRejectsMultipleDescriptors(t *testing.T) {
	pair := newPacketSocket(t)

	if _, err := newPacketDevice(&fakeFDTun{fds: []int{pair[1], pair[1]}}); err == nil {
		t.Fatal("a device with several packet descriptors must be rejected instead of silently using one")
	}
}

func TestFDPacketDeviceDrainsABatchAndWritesPackets(t *testing.T) {
	pair := newPacketSocket(t)
	device, err := newPacketDevice(&fakeFDTun{fds: []int{pair[1]}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	buffers := [][]byte{make([]byte, 64), make([]byte, 64)}
	if _, err := device.ReadPackets(buffers); !errors.Is(err, ErrQueueEmpty) {
		t.Fatalf("read error = %v, want %v while the device is empty", err, ErrQueueEmpty)
	}

	for _, packet := range [][]byte{[]byte("first"), []byte("second")} {
		if _, err := unix.Write(pair[0], packet); err != nil {
			t.Fatalf("queue packet: %v", err)
		}
	}
	count, err := device.ReadPackets(buffers)
	if err != nil {
		t.Fatalf("unexpected read error: %v", err)
	}
	if count != 2 {
		t.Fatalf("read %d packets, want the %d queued packets", count, 2)
	}
	if !bytes.Equal(buffers[0][:5], []byte("first")) || !bytes.Equal(buffers[1][:6], []byte("second")) {
		t.Fatalf("read %q and %q, want the queued packets in order", buffers[0][:5], buffers[1][:6])
	}

	if err := device.WritePackets([][]byte{[]byte("outbound")}); err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}
	out := make([]byte, 64)
	n, err := unix.Read(pair[0], out)
	if err != nil {
		t.Fatalf("read written packet: %v", err)
	}
	if !bytes.Equal(out[:n], []byte("outbound")) {
		t.Fatalf("device wrote %q, want %q", out[:n], "outbound")
	}
}
