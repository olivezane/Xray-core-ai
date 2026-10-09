//go:build !linux

package tun

// platformPacketDevice returns the packet device a platform TUN device carries
// packets through by itself, or nothing when the platform has none of its own.
// These platforms carry packets through the gVisor methods they implement, so
// packetDevice adapts those.
func platformPacketDevice(Tun) (packetDevice, error) {
	return nil, nil
}
