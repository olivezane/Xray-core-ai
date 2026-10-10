//go:build linux

package tun

// packetDeviceHeaderRoom is zero: the Linux and Android devices carry bare IP
// packets, with no address family header in front of them.
const packetDeviceHeaderRoom = 0
