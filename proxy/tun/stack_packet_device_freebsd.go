//go:build freebsd

package tun

// packetDeviceHeaderRoom is the Unix address family header a tun device puts in
// front of every packet.
const packetDeviceHeaderRoom = tunHeaderSize

// FDs exposes the descriptor the stack reads and writes packets through. It is
// taken from the descriptor itself rather than from File.Fd, which puts the
// descriptor back into blocking mode before handing it out.
func (t *FreeBSDTun) FDs() []int {
	raw, err := t.device.File().SyscallConn()
	if err != nil {
		return nil
	}
	var fd int
	if err := raw.Control(func(descriptor uintptr) { fd = int(descriptor) }); err != nil {
		return nil
	}
	return []int{fd}
}
