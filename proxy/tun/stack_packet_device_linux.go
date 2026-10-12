//go:build linux

package tun

// packetDeviceHeaderRoom is zero: the Linux and Android devices carry bare IP
// packets, with no address family header in front of them. A file descriptor
// handed in by the environment is refused when it carries a virtio_net_hdr, so
// there is none to strip here either.
const packetDeviceHeaderRoom = 0
