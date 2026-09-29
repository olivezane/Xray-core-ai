package udp

import (
	"fmt"

	"github.com/xtls/xray-core/common/net"
)

type Server struct {
	Port         net.Port
	MsgProcessor func(msg []byte) []byte
	accepting    bool
	conn         *net.UDPConn
}

func (server *Server) Start() (net.Destination, error) {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   []byte{127, 0, 0, 1},
		Port: int(server.Port),
		Zone: "",
	})
	if err != nil {
		return net.Destination{}, err
	}
	server.Port = net.Port(conn.LocalAddr().(*net.UDPAddr).Port) //nolint:gosec,forcetypeassert // G115: the port comes from a bound socket address, so it is 0..65535
	fmt.Println("UDP server started on port ", server.Port)

	server.conn = conn
	go server.handleConnection(conn)
	//nolint:forcetypeassert // the socket was created by this package as a UDP socket
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return net.UDPDestination(net.IPAddress(localAddr.IP), net.Port(localAddr.Port)), nil //nolint:gosec // Port of a net.Addr is always 0..65535
}

func (server *Server) handleConnection(conn *net.UDPConn) {
	server.accepting = true
	for server.accepting {
		buffer := make([]byte, 2*1024)
		nBytes, addr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			fmt.Printf("Failed to read from UDP: %v\n", err)
			continue
		}

		response := server.MsgProcessor(buffer[:nBytes])
		if _, err := conn.WriteToUDP(response, addr); err != nil {
			fmt.Println("Failed to write to UDP: ", err.Error())
		}
	}
}

func (server *Server) Close() error {
	server.accepting = false
	return server.conn.Close()
}
