package tun

import (
	"time"

	"github.com/xtls/xray-core/common/errors"
	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

// packetDevice moves complete IP packets between a TUN device and a Network
// Stack, without exposing any stack's own packet types.
type packetDevice interface {
	// ReadPackets fills up to len(packets) buffers with one complete packet
	// each and reports how many were read. A packet longer than its buffer is
	// truncated, as it would be by the device itself. It never blocks: it
	// returns ErrQueueEmpty when the device has nothing to read.
	ReadPackets(packets [][]byte) (int, error)
	// WritePackets writes every packet of the batch, in order. A packet the
	// device cannot buffer right now is dropped; the first hard error aborts
	// the batch.
	WritePackets(packets [][]byte) error
	// Wait parks the caller until the device may have a packet to read. It
	// always returns within a bounded time, so a caller that is closing can
	// observe it and stop.
	Wait()
}

// packetDevice returns the Packet Device of a TUN device: either the surface
// the device implements itself, or one adapted from the platform transport it
// already carries packets with.
func newPacketDevice(device Tun) (packetDevice, error) {
	if own, ok := device.(packetDevice); ok {
		return own, nil
	}
	if platform, err := platformPacketDevice(device); platform != nil || err != nil {
		return platform, err
	}

	// devices that carry packets as gVisor buffers need no packet device of
	// their own
	if gvDevice, ok := device.(GVisorDevice); ok {
		return newGvisorPacketDevice(gvDevice), nil
	}

	return nil, errors.New("tun device supports no packet device path")
}

// gvisorPacketDeviceWait is how long the gVisor device adapter parks between
// reads, matching the fallback park of the platform devices it stands in for
const gvisorPacketDeviceWait = time.Millisecond

// gvisorPacketDevice adapts the packet methods a TUN device already implements
// for the gVisor Stack into a Packet Device, so devices that carry packets as
// gVisor buffers need no changes of their own.
type gvisorPacketDevice struct {
	device GVisorDevice
}

// newGvisorPacketDevice adapts one gVisor-capable device
func newGvisorPacketDevice(device GVisorDevice) *gvisorPacketDevice {
	return &gvisorPacketDevice{device: device}
}

func (d *gvisorPacketDevice) ReadPackets(packets [][]byte) (int, error) {
	if len(packets) == 0 {
		return 0, nil
	}

	_, packet, err := d.device.ReadPacket()
	if err != nil {
		return 0, err
	}
	if packet == nil {
		return 0, ErrQueueEmpty
	}
	defer packet.DecRef()

	copyPacket(packets[0], packet)

	return 1, nil
}

func (d *gvisorPacketDevice) WritePackets(packets [][]byte) error {
	for _, packet := range packets {
		packetBuffer := stack.NewPacketBuffer(stack.PacketBufferOptions{
			Payload: buffer.MakeWithData(packet),
		})
		err := d.device.WritePacket(packetBuffer)
		packetBuffer.DecRef()
		if err != nil {
			return errors.New("failed to write packet to tun device: ", err.String())
		}
	}

	return nil
}

func (d *gvisorPacketDevice) Wait() {
	// the park is bounded here instead of handed to the device: the Windows
	// device waits for a read event without a timeout, so a caller parked there
	// would not return until a packet arrived, and could never stop
	time.Sleep(gvisorPacketDeviceWait)
}

// copyPacket copies the bytes of a gVisor packet buffer into dst, and reports
// how many bytes were copied
func copyPacket(dst []byte, packet *stack.PacketBuffer) int {
	var written int
	for _, slice := range packet.AsSlices() {
		if written == len(dst) {
			break
		}
		written += copy(dst[written:], slice)
	}
	return written
}
