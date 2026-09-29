//go:build linux

package dokodemo

import (
	"fmt"
	"net"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

func FakeUDP(addr *net.UDPAddr, mark int) (net.PacketConn, error) {
	var af int
	var sockaddr syscall.Sockaddr

	if len(addr.IP) == 4 {
		af = syscall.AF_INET
		sockaddr = &syscall.SockaddrInet4{Port: addr.Port}
		//nolint:forcetypeassert // the sockaddr was built by this function as AF_INET
		copy(sockaddr.(*syscall.SockaddrInet4).Addr[:], addr.IP)
	} else {
		af = syscall.AF_INET6
		sockaddr = &syscall.SockaddrInet6{Port: addr.Port}
		//nolint:forcetypeassert // the sockaddr was built by this function as AF_INET6
		copy(sockaddr.(*syscall.SockaddrInet6).Addr[:], addr.IP)
	}

	var fd int
	var err error

	if fd, err = syscall.Socket(af, syscall.SOCK_DGRAM, 0); err != nil {
		return nil, &net.OpError{Op: "fake", Err: fmt.Errorf("socket open: %w", err)}
	}

	if mark != 0 {
		if err = syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_MARK, mark); err != nil {
			_ = syscall.Close(fd)
			return nil, &net.OpError{Op: "fake", Err: fmt.Errorf("set socket option: SO_MARK: %w", err)}
		}
	}

	if err = syscall.SetsockoptInt(fd, syscall.SOL_IP, syscall.IP_TRANSPARENT, 1); err != nil {
		_ = syscall.Close(fd)
		return nil, &net.OpError{Op: "fake", Err: fmt.Errorf("set socket option: IP_TRANSPARENT: %w", err)}
	}

	_ = syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)

	_ = syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, unix.SO_REUSEPORT, 1)

	if err = syscall.Bind(fd, sockaddr); err != nil {
		_ = syscall.Close(fd)
		return nil, &net.OpError{Op: "fake", Err: fmt.Errorf("socket bind: %w", err)}
	}

	fdFile := os.NewFile(uintptr(fd), "net-udp-fake-"+addr.String())
	defer fdFile.Close()

	packetConn, err := net.FilePacketConn(fdFile)
	if err != nil {
		_ = syscall.Close(fd)
		return nil, &net.OpError{Op: "fake", Err: fmt.Errorf("convert file descriptor to connection: %w", err)}
	}

	return packetConn, nil
}
