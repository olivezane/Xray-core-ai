package tcp

import (
	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/net"
)

// PickPort returns an unused TCP port in the system. The port returned is highly likely to be unused, but not guaranteed.
func PickPort() net.Port {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	common.Must(err)
	defer listener.Close()

	//nolint:forcetypeassert // the socket was created by this package as a TCP socket
	addr := listener.Addr().(*net.TCPAddr)
	return net.Port(addr.Port) //nolint:gosec // Port of a net.Addr is always 0..65535
}
