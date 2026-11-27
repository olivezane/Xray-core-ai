package internet

import (
	"context"

	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
)

// ListenUnix is the UDS version of ListenTCP
func ListenUnix(ctx context.Context, address net.Address, settings *MemoryStreamConfig, handler ConnHandler) (Listener, error) {
	if settings == nil {
		s, err := ToMemoryStreamConfig(nil)
		if err != nil {
			return nil, errors.New("failed to create default unix stream settings").Base(err)
		}
		settings = s
	}

	listener, err := listenStream(ctx, settings.ProtocolName, address, net.Port(0), settings, handler)
	if err != nil {
		return nil, errors.New("failed to listen on unix address: ", address).Base(err)
	}
	return listener, nil
}

func ListenTCP(ctx context.Context, address net.Address, port net.Port, settings *MemoryStreamConfig, handler ConnHandler) (Listener, error) {
	if settings == nil {
		s, err := ToMemoryStreamConfig(nil)
		if err != nil {
			return nil, errors.New("failed to create default stream settings").Base(err)
		}
		settings = s
	}

	if address.Family().IsDomain() && address.Domain() == "localhost" {
		address = net.LocalHostIP
	}

	if address.Family().IsDomain() {
		return nil, errors.New("domain address is not allowed for listening: ", address.Domain())
	}
	if port == 0 {
		return nil, errors.New("port 0 is not allowed for listening on TCP")
	}

	listener, err := listenStream(ctx, settings.ProtocolName, address, port, settings, handler)
	if err != nil {
		return nil, errors.New("failed to listen on address: ", address, ":", port).Base(err)
	}
	return listener, nil
}

// listenStream hands the accept loop of a listener over to the transport named
// by the stream settings. It is the only way an inbound gets a listener, so
// transportFor is where the protocol is looked up and where a protocol that
// cannot listen is reported.
func listenStream(ctx context.Context, protocol string, address net.Address, port net.Port, settings *MemoryStreamConfig, handler ConnHandler) (Listener, error) {
	if listen := transportFor(protocol).Listen; listen != nil {
		return listen(ctx, address, port, settings, handler)
	}
	return nil, errors.New(protocol, " is not a stream transport that listens; the transports of this build are listed in transport/internet/transports")
}

// ListenSystem listens on a local address for incoming TCP connections.
//
// xray:api:beta
func ListenSystem(ctx context.Context, addr net.Addr, sockopt *SocketConfig) (net.Listener, error) {
	return effectiveListener.Listen(ctx, addr, sockopt)
}

// ListenSystemPacket listens on a local address for incoming UDP connections.
//
// xray:api:beta
func ListenSystemPacket(ctx context.Context, addr net.Addr, sockopt *SocketConfig) (net.PacketConn, error) {
	return effectiveListener.ListenPacket(ctx, addr, sockopt)
}
