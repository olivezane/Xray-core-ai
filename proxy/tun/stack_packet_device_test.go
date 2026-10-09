package tun

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

// recordingPacketTun is a device that carries packets as gVisor buffers, like
// the Windows, Darwin, and FreeBSD devices do.
type recordingPacketTun struct {
	fakeTun

	packet  []byte
	readErr error
	written [][]byte
	waits   int
	// blockWait makes Wait park until the channel is closed, the way the
	// Windows device parks on a read event
	blockWait chan struct{}
}

func (d *recordingPacketTun) WritePacket(packet *stack.PacketBuffer) tcpip.Error {
	var written []byte
	for _, slice := range packet.AsSlices() {
		written = append(written, slice...)
	}
	d.written = append(d.written, written)
	return nil
}

func (d *recordingPacketTun) ReadPacket() (byte, *stack.PacketBuffer, error) {
	if d.readErr != nil {
		return 0, nil, d.readErr
	}
	packet := stack.NewPacketBuffer(stack.PacketBufferOptions{
		Payload: buffer.MakeWithData(d.packet),
	})
	d.packet = nil
	return 4, packet, nil
}

func (d *recordingPacketTun) Wait() {
	d.waits++
	if d.blockWait != nil {
		<-d.blockWait
	}
}

func TestPacketDeviceRejectsADeviceWithoutPacketPath(t *testing.T) {
	_, err := newPacketDevice(&fakeTun{})
	if err == nil {
		t.Fatal("a device that carries no packets must not produce a packet device")
	}
}

func TestPacketDeviceAdaptsAGVisorDevice(t *testing.T) {
	device := &recordingPacketTun{packet: []byte{0x45, 0x00, 0x00, 0x14, 0xde, 0xad}}
	packets, err := newPacketDevice(device)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	buffers := [][]byte{make([]byte, 64), make([]byte, 64)}
	count, err := packets.ReadPackets(buffers)
	if err != nil {
		t.Fatalf("unexpected read error: %v", err)
	}
	if count != 1 {
		t.Fatalf("read %d packets, want 1", count)
	}
	if want := []byte{0x45, 0x00, 0x00, 0x14, 0xde, 0xad}; !bytes.Equal(buffers[0][:6], want) {
		t.Fatalf("read % x, want % x", buffers[0][:6], want)
	}

	if err := packets.WritePackets([][]byte{{0x60, 0x00}, {0x45, 0x01}}); err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}
	if len(device.written) != 2 || !bytes.Equal(device.written[0], []byte{0x60, 0x00}) || !bytes.Equal(device.written[1], []byte{0x45, 0x01}) {
		t.Fatalf("device received % x, want the two packets in order", device.written)
	}

	// the park must end on its own, so a closing caller is never stuck on a
	// device whose own wait has no timeout
	packets.Wait()
	if device.waits != 0 {
		t.Fatalf("the adapter waited on the device %d times, want its own bounded park", device.waits)
	}
}

func TestPacketDeviceWaitIsBounded(t *testing.T) {
	blocked := make(chan struct{})
	defer close(blocked)
	device := &recordingPacketTun{blockWait: blocked}
	packets, err := newPacketDevice(device)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	returned := make(chan struct{})
	go func() {
		packets.Wait()
		close(returned)
	}()

	select {
	case <-returned:
	case <-time.After(10 * time.Second):
		t.Fatal("the packet device parked without returning, so a closing caller could not stop")
	}
}

func TestPacketDeviceReportsAnEmptyGVisorQueue(t *testing.T) {
	device := &recordingPacketTun{readErr: ErrQueueEmpty}
	packets, err := newPacketDevice(device)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := packets.ReadPackets([][]byte{make([]byte, 64)}); !errors.Is(err, ErrQueueEmpty) {
		t.Fatalf("read error = %v, want %v", err, ErrQueueEmpty)
	}
}

func TestPacketDeviceReportsAGVisorDeviceFailure(t *testing.T) {
	failure := errors.New("device gone")
	device := &recordingPacketTun{readErr: failure}
	packets, err := newPacketDevice(device)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := packets.ReadPackets([][]byte{make([]byte, 64)}); !errors.Is(err, failure) {
		t.Fatalf("read error = %v, want the device failure", err)
	}
}
