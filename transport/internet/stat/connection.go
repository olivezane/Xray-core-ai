package stat

import (
	"net"

	"github.com/xtls/xray-core/features/stats"
)

// Unwrapper is implemented by connection wrappers that can peel off
// one layer to reveal the connection underneath.
type Unwrapper interface {
	Unwrap() net.Conn
}

type Connection interface {
	net.Conn
}

type CounterConnection struct {
	Connection
	ReadCounter  stats.Counter
	WriteCounter stats.Counter
}

var _ Unwrapper = (*CounterConnection)(nil)

func (c *CounterConnection) Unwrap() net.Conn {
	return c.Connection
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

func TryUnwrapStatsConn(conn net.Conn) net.Conn {
	if conn == nil {
		return conn
	}
	if conn, ok := conn.(*CounterConnection); ok {
		return conn.Connection
	}
	return conn
}
