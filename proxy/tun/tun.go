package tun

import "github.com/xtls/xray-core/common/errors"

// packetRoom is what a tun may need in front of a packet in the buffer that it is read into
const packetRoom = 16

// Tun interface implements tun interface interaction
type Tun interface {
	Start() error
	Close() error
	Name() (string, error)
	Index() (int, error)
	// ReadPacket waits for a packet, reads it into b and returns it
	ReadPacket(b []byte) ([]byte, error)
	// WritePacket writes a packet that is in two parts, the second of which may be empty
	WritePacket(header, payload []byte) error
}

// segmenter is a tun whose other side takes TCP segments that are larger than the MTU,
// splits them only if it has to send them on, and completes their checksum then
type segmenter interface {
	// Segments reports whether WriteSegment can be used
	Segments() bool
	// WriteSegment writes a packet with a TCP segment whose payload is in parts, to be sent on with mss bytes
	// of it at most in one. Its checksum is that of the pseudo-header only, not inverted
	WriteSegment(header []byte, payload [][]byte, mss int) error
}

// ErrQueueEmpty reports that a device had no packet to read
var ErrQueueEmpty = errors.New("queue is empty")

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

// tunPacketDevice is implemented by a device that can offer its packets
// through a packet device of its own. A Network Stack is built on that device,
// and a platform whose device cannot offer one carries no such Stack.
type tunPacketDevice interface {
	packetDevice() (packetDevice, error)
}

// mipstackPacketDevice returns the Packet Device of a TUN device, or the error
// naming why this platform cannot carry the MIPS Stack
func mipstackPacketDevice(device Tun) (packetDevice, error) {
	own, ok := device.(tunPacketDevice)
	if !ok {
		return nil, errors.New("the ", StackMipstack, " stack is not supported by the tun device of this platform")
	}
	return own.packetDevice()
}
