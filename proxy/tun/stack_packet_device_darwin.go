//go:build darwin

package tun

// packetDeviceHeaderRoom is the Unix address family header a utun device puts
// in front of every packet.
const packetDeviceHeaderRoom = utunHeaderSize

// FDs exposes the descriptor the stack reads and writes packets through.
func (t *DarwinTun) FDs() []int {
	return []int{t.tunFd}
}
