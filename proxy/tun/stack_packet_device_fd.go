//go:build linux || darwin || freebsd

package tun

import (
	"errors"
	"time"

	xerrors "github.com/xtls/xray-core/common/errors"
	"golang.org/x/sys/unix"
)

// waitTimeout bounds one Wait, so a closing caller is not parked on the device
const waitTimeout = time.Second

// fdEndpointDevice is implemented by devices whose packets flow through a
// plain file descriptor (Linux, Android, Darwin, FreeBSD)
type fdEndpointDevice interface {
	FDs() []int
}

// platformPacketDevice returns the packet device a platform TUN device carries
// packets through by itself, or nothing when the platform has none of its own
func platformPacketDevice(device Tun) (packetDevice, error) {
	fdDevice, ok := device.(fdEndpointDevice)
	if !ok {
		return nil, nil
	}
	fds := fdDevice.FDs()
	if len(fds) != 1 {
		return nil, xerrors.New("tun device exposes ", len(fds), " packet file descriptors, the stack needs exactly one")
	}
	// the poll-then-read loop below needs a non-blocking descriptor. FreeBSD
	// opens the device blocking, and Darwin hands out a blocking descriptor too:
	// File.Fd puts one back into blocking mode before returning it
	if err := unix.SetNonblock(fds[0], true); err != nil {
		return nil, err
	}
	return &fdPacketDevice{fd: fds[0], headerRoom: packetDeviceHeaderRoom}, nil
}

// fdPacketDevice is a packet device over the plain file descriptor a platform
// TUN device owns.
//
// headerRoom is how many bytes the device puts in front of every packet and
// counts as part of the read: zero for Linux and Android, which carry bare IP
// packets, and the Unix 4-byte address family header for Darwin and FreeBSD. A
// write reserves those bytes and names the family in the last one, a read
// skips them.
type fdPacketDevice struct {
	fd         int
	headerRoom int

	// readScratch belongs to the inbound pump, which reads a framed packet out
	// of a buffer the device header also fits in
	readScratch []byte

	// writeScratch belongs to the outbound pump, which frames one packet per
	// iteration
	writeScratch []byte
}

func (d *fdPacketDevice) ReadPackets(packets [][]byte) (int, error) {
	var count int
	for count < len(packets) {
		if !d.readable(0) {
			break
		}
		buffer := packets[count]
		if d.headerRoom > 0 {
			buffer = d.readScratchFor(len(packets[count]))
		}
		n, err := unix.Read(d.fd, buffer)
		// a descriptor that became empty or was interrupted between the poll
		// and the read ends this batch
		if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EINTR) {
			break
		}
		if err != nil {
			return count, err
		}
		// an empty read, or a device header with no packet behind it, is not a
		// packet
		if n <= d.headerRoom {
			break
		}
		if d.headerRoom > 0 {
			copy(packets[count], buffer[d.headerRoom:n])
		}
		count++
	}

	if count == 0 {
		return 0, ErrQueueEmpty
	}

	return count, nil
}

func (d *fdPacketDevice) WritePackets(packets [][]byte) error {
	for _, packet := range packets {
		buffer := packet
		if d.headerRoom > 0 {
			buffer = d.frame(packet)
			// a packet naming no address family this device carries is dropped,
			// the way the gVisor device rejects it
			if buffer == nil {
				continue
			}
		}

		_, err := unix.Write(d.fd, buffer)
		// a device that cannot buffer the packet right now drops it, the same
		// way the gVisor Stack treats a would-block write
		if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return err
		}
	}

	return nil
}

// readScratchFor returns the buffer a framed device reads into: it has to hold
// the device header in front of the packet, which the caller's buffer does not
func (d *fdPacketDevice) readScratchFor(packetLen int) []byte {
	if len(d.readScratch) < packetLen+d.headerRoom {
		d.readScratch = make([]byte, packetLen+d.headerRoom)
	}
	return d.readScratch
}

// frame prepends the Unix packet header to one packet, or reports nil for a
// packet that names no address family this device carries
func (d *fdPacketDevice) frame(packet []byte) []byte {
	// a packet with nothing behind the header names no address family
	if len(packet) == 0 {
		return nil
	}
	if len(d.writeScratch) < len(packet)+d.headerRoom {
		d.writeScratch = make([]byte, len(packet)+d.headerRoom)
	}
	framed := d.writeScratch[:len(packet)+d.headerRoom]
	clear(framed[:d.headerRoom])
	copy(framed[d.headerRoom:], packet)

	var family byte
	switch framed[d.headerRoom] >> 4 {
	case 4:
		family = unix.AF_INET
	case 6:
		family = unix.AF_INET6
	default:
		return nil
	}
	framed[d.headerRoom-1] = family

	return framed
}

func (d *fdPacketDevice) Wait() {
	d.readable(waitTimeout)
}

// readable reports whether the device has a packet to read, waiting at most
// for the given timeout
func (d *fdPacketDevice) readable(timeout time.Duration) bool {
	fds := []unix.PollFd{{Fd: int32(d.fd), Events: unix.POLLIN}}
	for {
		n, err := unix.Poll(fds, int(timeout.Milliseconds()))
		if errors.Is(err, unix.EINTR) {
			continue
		}
		// an unusable descriptor reports POLLNVAL: report readable so the next
		// read surfaces the real error instead of parking forever
		return err != nil || n > 0 || fds[0].Revents&unix.POLLNVAL != 0
	}
}
