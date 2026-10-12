//go:build windows

package tun

import (
	go_errors "errors"

	"github.com/xtls/xray-core/common/errors"
	"golang.org/x/sys/windows"
)

// windowsPacketDeviceWaitMillis bounds one Wait: the read event the device
// parks on is not signaled when the session ends, so an unbounded wait would
// keep a closing caller parked until the next packet arrived
const windowsPacketDeviceWaitMillis = 1000

// windowsPacketDevice is a packet device over the Wintun ring buffer the
// Windows TUN device owns. The ring hands out the packets it received, and the
// room for the packets it sends, as plain slices, so nothing has to be wrapped
// in a packet buffer on the way through. Its Wait parks on the same read event
// the device exposes, but with a bound, since that event is not signalled when
// the session ends.
type windowsPacketDevice struct {
	device *WindowsTun
}

// packetDevice implements tunPacketDevice: the MIPS Stack reads and writes the
// packets of this device through the Wintun ring buffer it already owns
func (t *WindowsTun) packetDevice() (packetDevice, error) {
	return &windowsPacketDevice{device: t}, nil
}

func (d *windowsPacketDevice) ReadPackets(packets [][]byte) (int, error) {
	// the session may be ended by a concurrent Close, which takes the write
	// lock, so hold the read lock for as long as the ring is in use
	d.device.RLock()
	defer d.device.RUnlock()
	if d.device.closed {
		return 0, errors.New("tun device is closed")
	}

	var count int
	for count < len(packets) {
		packet, err := d.device.session.ReceivePacket()
		if go_errors.Is(err, windows.ERROR_NO_MORE_ITEMS) {
			break
		}
		if err != nil {
			return count, err
		}
		// a packet longer than its buffer is truncated, as it would be by the
		// device itself
		copy(packets[count], packet)
		count++
		d.device.session.ReleaseReceivePacket(packet)
	}

	if count == 0 {
		return 0, ErrQueueEmpty
	}

	return count, nil
}

func (d *windowsPacketDevice) WritePackets(packets [][]byte) error {
	d.device.RLock()
	defer d.device.RUnlock()
	if d.device.closed {
		return errors.New("tun device is closed")
	}

	for _, packet := range packets {
		buffer, err := d.device.session.AllocateSendPacket(len(packet))
		if go_errors.Is(err, windows.ERROR_BUFFER_OVERFLOW) {
			// the ring is full: drop the packet
			continue
		}
		if err != nil {
			return err
		}
		copy(buffer, packet)
		d.device.session.SendPacket(buffer)
	}

	return nil
}

func (d *windowsPacketDevice) Wait() {
	// the read event is torn down when the session ends, so hold the read lock
	// for as long as it is waited on, the same way the ring is
	d.device.RLock()
	defer d.device.RUnlock()
	if d.device.closed {
		return
	}
	_, _ = windows.WaitForSingleObject(d.device.readWait, windowsPacketDeviceWaitMillis)
}
