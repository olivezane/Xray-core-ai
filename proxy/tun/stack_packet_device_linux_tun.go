//go:build linux && !android

package tun

// packetDevice implements tunPacketDevice: the MIPS Stack reads and writes the
// packets of this device through its file descriptor
func (t *LinuxTun) packetDevice() (packetDevice, error) {
	return packetDeviceOfFd(t.fd, packetDeviceHeaderRoom)
}
