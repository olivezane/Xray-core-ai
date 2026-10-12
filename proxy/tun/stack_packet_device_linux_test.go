//go:build linux

package tun

import (
	"bytes"
	"errors"
	"testing"

	"golang.org/x/sys/unix"
)

// fakeFDTun is a device whose packets flow through a plain file descriptor,
// like the Linux and Android devices.
type fakeFDTun struct {
	fakeTun
	fds []int
}

func (f *fakeFDTun) packetDevice() (packetDevice, error) {
	if len(f.fds) == 0 {
		return nil, errors.New("the tun device exposes no packet descriptor")
	}
	return packetDeviceOfFd(f.fds[0], packetDeviceHeaderRoom)
}

func TestFDPacketDeviceRejectsADeviceWithoutAPacketDescriptor(t *testing.T) {
	if _, err := mipstackPacketDevice(&fakeFDTun{}); err == nil {
		t.Fatal("a device with no packet descriptor must be rejected instead of built on a descriptor that does not exist")
	}
}

func TestFDPacketDeviceDrainsABatchAndWritesPackets(t *testing.T) {
	pair := newPacketSocket(t)
	device, err := mipstackPacketDevice(&fakeFDTun{fds: []int{pair[1]}})
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
