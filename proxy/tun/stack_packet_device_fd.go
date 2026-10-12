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

// fdPacketDevice is a packet device over the plain file descriptor a TUN
// device owns. The descriptor may be blocking or not: every read is preceded
// by a poll, so a read never parks the inbound pump.
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
			// a packet naming no address family this device carries is dropped
			if buffer == nil {
				continue
			}
		}

		// shortcut: writes are not polled, so on a blocking descriptor (Darwin,
		// FreeBSD) a full device queue parks the outbound pump until the kernel
		// drains it. Open the descriptor O_NONBLOCK if a device ever backs up
		// for long enough to stall a close.
		_, err := unix.Write(d.fd, buffer)
		// a device that cannot buffer the packet right now drops it, the way a
		// would-block write is treated
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

// packetDeviceOfFd builds the packet device of a device whose packets flow
// through one file descriptor, with the header the platform puts in front of
// every packet
func packetDeviceOfFd(fd int, headerRoom int) (*fdPacketDevice, error) {
	if fd < 0 {
		return nil, xerrors.New("the tun device exposes no packet file descriptor")
	}
	return &fdPacketDevice{fd: fd, headerRoom: headerRoom}, nil
}
