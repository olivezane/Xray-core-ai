//go:build linux

package tun

import (
	"errors"
	"time"

	xerrors "github.com/xtls/xray-core/common/errors"
	"golang.org/x/sys/unix"
)

// fdPacketDevice is a packet device over the plain file descriptor a Linux or
// Android TUN device owns
type fdPacketDevice struct {
	fd int
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
	return &fdPacketDevice{fd: fds[0]}, nil
}

// waitTimeout bounds one Wait, so a closing caller is not parked on the device
const waitTimeout = time.Second

func (d *fdPacketDevice) ReadPackets(packets [][]byte) (int, error) {
	var count int
	for count < len(packets) {
		if !d.readable(0) {
			break
		}
		n, err := unix.Read(d.fd, packets[count])
		// a descriptor that became empty or was interrupted between the poll
		// and the read ends this batch
		if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EINTR) {
			break
		}
		if err != nil {
			return count, err
		}
		if n == 0 {
			break
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
		_, err := unix.Write(d.fd, packet)
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
