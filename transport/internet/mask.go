package internet

import (
	"reflect"

	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/net/cnc"
)

// Mask wrap-chain builder.
//
// Every transport applies its masks through this builder, which fixes the
// order of the wrap chain in exactly one place: the mask wraps the raw
// connection BEFORE TLS or reality is layered on top. (A stateful mask such as
// XMC reads the raw stream during its login handshake, so masking after TLS
// would break it.)

// WrapConnClient applies the configured TCP mask chain to a client-side raw
// connection, then hands the result to wrapTLS (if non-nil) — the fixed
// mask-before-TLS order. The raw connection is returned unchanged when no mask
// is configured. On wrap error the raw connection is closed.
func WrapConnClient(mss *MemoryStreamConfig, raw net.Conn, wrapTLS func(net.Conn) (net.Conn, error)) (net.Conn, error) {
	if mss != nil && mss.TcpmaskManager != nil {
		newConn, err := mss.TcpmaskManager.WrapConnClient(raw)
		if err != nil {
			raw.Close()
			return nil, errors.New("mask err").Base(err)
		}
		raw = newConn
	}
	if wrapTLS != nil {
		return wrapTLS(raw)
	}
	return raw, nil
}

// WrapListener applies the configured TCP mask chain to a server-side
// listener. The listener is returned unchanged when no mask is configured.
func WrapListener(mss *MemoryStreamConfig, l net.Listener) (net.Listener, error) {
	if mss == nil || mss.TcpmaskManager == nil {
		return l, nil
	}
	return mss.TcpmaskManager.WrapListener(l)
}

// UnwrapPacketConn converts a connection dialed by DialSystem into its
// underlying PacketConn and remote address. On an unsupported connection type
// the connection is closed and an error is returned.
func UnwrapPacketConn(raw net.Conn) (net.PacketConn, *net.UDPAddr, error) {
	switch c := raw.(type) {
	case *PacketConnWrapper:
		return c.PacketConn, c.RemoteAddr().(*net.UDPAddr), nil
	case *cnc.Connection:
		return &FakePacketConn{Conn: c}, &net.UDPAddr{IP: c.RemoteAddr().(*net.TCPAddr).IP, Port: c.RemoteAddr().(*net.TCPAddr).Port}, nil
	default:
		raw.Close()
		return nil, nil, errors.New("mask: unsupported dialed connection type ", reflect.TypeOf(raw))
	}
}

// WrapPacketConnClient applies the UDP mask chain (client side) to a
// PacketConn. The conn is returned unchanged when no mask is configured; on
// wrap error the raw conn is closed.
func WrapPacketConnClient(mss *MemoryStreamConfig, raw net.PacketConn) (net.PacketConn, error) {
	if mss == nil || mss.UdpmaskManager == nil {
		return raw, nil
	}
	newConn, err := mss.UdpmaskManager.WrapPacketConnClient(raw)
	if err != nil {
		raw.Close()
		return nil, errors.New("mask err").Base(err)
	}
	return newConn, nil
}

// WrapPacketConnServer applies the UDP mask chain (server side) to a
// PacketConn. The conn is returned unchanged when no mask is configured; on
// wrap error the raw conn is closed.
func WrapPacketConnServer(mss *MemoryStreamConfig, raw net.PacketConn) (net.PacketConn, error) {
	if mss == nil || mss.UdpmaskManager == nil {
		return raw, nil
	}
	newConn, err := mss.UdpmaskManager.WrapPacketConnServer(raw)
	if err != nil {
		raw.Close()
		return nil, errors.New("mask err").Base(err)
	}
	return newConn, nil
}

// WrapPacketConnDial applies the UDP mask chain to a connection dialed by
// DialSystem, returning a net.Conn suitable for use as a stat.Connection. The
// dialed conn is returned unchanged when no mask is configured.
func WrapPacketConnDial(mss *MemoryStreamConfig, raw net.Conn) (net.Conn, error) {
	if mss == nil || mss.UdpmaskManager == nil {
		return raw, nil
	}
	pktConn, addr, err := UnwrapPacketConn(raw)
	if err != nil {
		return nil, err
	}
	wrapped, err := WrapPacketConnClient(mss, pktConn)
	if err != nil {
		return nil, err
	}
	return &PacketConnWrapper{PacketConn: wrapped, Dest: addr}, nil
}
