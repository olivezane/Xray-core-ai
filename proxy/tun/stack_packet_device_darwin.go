//go:build darwin

package tun

import "github.com/xtls/xray-core/common/errors"

// packetDeviceHeaderRoom is the Unix address family header a utun device puts
// in front of every packet.
const packetDeviceHeaderRoom = utunHeaderSize

// packetDevice implements tunPacketDevice: the MIPS Stack reads and writes the
// packets of this device through its file descriptor. The descriptor is taken
// from the device rather than from File.Fd, which puts it back into blocking
// mode before handing it out.
func (t *DarwinTun) packetDevice() (packetDevice, error) {
	raw, err := t.tunFile.SyscallConn()
	if err != nil {
		return nil, errors.New("failed to reach the tun device of this platform").Base(err)
	}
	fd := -1
	if err := raw.Control(func(descriptor uintptr) { fd = int(descriptor) }); err != nil {
		return nil, errors.New("failed to reach the tun device of this platform").Base(err)
	}
	return packetDeviceOfFd(fd, packetDeviceHeaderRoom)
}
