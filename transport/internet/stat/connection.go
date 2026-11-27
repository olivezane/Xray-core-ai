package stat

import (
	"net"

	"github.com/xtls/xray-core/features/stats"
)

type Connection interface {
	net.Conn
}

type CounterConnection struct {
	Connection
	ReadCounter  stats.Counter
	WriteCounter stats.Counter
}

func (c *CounterConnection) Read(b []byte) (int, error) {
	nBytes, err := c.Connection.Read(b)
	if c.ReadCounter != nil {
		c.ReadCounter.Add(int64(nBytes))
	}

	return nBytes, err
}

func (c *CounterConnection) Write(b []byte) (int, error) {
	nBytes, err := c.Connection.Write(b)
	if c.WriteCounter != nil {
		c.WriteCounter.Add(int64(nBytes))
	}
	return nBytes, err
}

// Unwrapper is implemented by a wrapper that hides the connection it wraps, e.g.
// the connection tracker. Unwrapping keeps type checks (a *net.PacketConnWrapper,
// for instance) working through a wrapped connection.
type Unwrapper interface {
	Unwrap() net.Conn
}

// TryUnwrapStatsConn returns the connection behind the wrappers Xray puts around
// a dialed or accepted connection, so that callers can see what it really is.
func TryUnwrapStatsConn(conn net.Conn) net.Conn {
	for conn != nil {
		switch wrapper := conn.(type) {
		case *CounterConnection:
			conn = wrapper.Connection
		case Unwrapper:
			conn = wrapper.Unwrap()
		default:
			return conn
		}
	}
	return conn
}
