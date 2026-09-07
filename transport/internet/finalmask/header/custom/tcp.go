package custom

import (
	"bytes"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/xtls/xray-core/common/crypto"
	"github.com/xtls/xray-core/common/errors"
)

type tcpCustomClient struct {
	clients []*TCPSequence
	servers []*TCPSequence
	state   *stateStore
}

type tcpCustomClientConn struct {
	net.Conn
	header *tcpCustomClient

	auth atomic.Bool
	wg   sync.WaitGroup
	once sync.Once
}

func NewConnClientTCP(c *TCPConfig, raw net.Conn) (net.Conn, error) {
	conn := &tcpCustomClientConn{
		Conn: raw,
		header: &tcpCustomClient{
			clients: c.Clients,
			servers: c.Servers,
			state:   newStateStore(5 * time.Second),
		},
	}

	conn.wg.Add(1)

	return conn, nil
}

func (c *tcpCustomClientConn) TcpMaskConn() {}

func (c *tcpCustomClientConn) RawConn() net.Conn {
	// c.wg.Wait()

	return c.Conn
}

// Splice 在握手完成前返回 false:relay 的 peel 逻辑(UnwrapTcpMask)
// 只看 Splice() 就决定是否剥层,若握手未完成即放行,header 序列
// 将永远不会上线,伪装被整体旁路。握手完成后流是纯数据,此时
// RawConn 才可安全交给 splice 拷贝。
func (c *tcpCustomClientConn) Splice() bool {
	return c.auth.Load()
}

func (c *tcpCustomClientConn) Read(p []byte) (n int, err error) {
	c.wg.Wait()

	if !c.auth.Load() {
		return 0, errors.New("header auth failed")
	}

	return c.Conn.Read(p)
}

func (c *tcpCustomClientConn) Write(p []byte) (n int, err error) {
	c.once.Do(func() {
		ctx := newEvalContextWithAddrs(c.LocalAddr(), c.RemoteAddr())
		if vars, ok := c.header.state.get(tcpStateKey(c.LocalAddr(), c.RemoteAddr())); ok {
			ctx.vars = cloneVars(vars)
		}
		i := 0
		j := 0
		for i = range c.header.clients {
			if !writeSequenceWithContext(c.Conn, c.header.clients[i], ctx) {
				c.wg.Done()
				return
			}

			if j < len(c.header.servers) {
				if !readSequenceWithContext(c.Conn, c.header.servers[j], ctx) {
					c.wg.Done()
					return
				}
				j++
			}
		}

		for j < len(c.header.servers) {
			if !readSequenceWithContext(c.Conn, c.header.servers[j], ctx) {
				c.wg.Done()
				return
			}
			j++
		}

		c.header.state.set(tcpStateKey(c.LocalAddr(), c.RemoteAddr()), ctx.vars)
		c.auth.Store(true)
		c.wg.Done()
	})

	c.wg.Wait()

	if !c.auth.Load() {
		return 0, errors.New("header auth failed")
	}

	return c.Conn.Write(p)
}

type tcpCustomServer struct {
	clients []*TCPSequence
	servers []*TCPSequence
	errors  []*TCPSequence
	state   *stateStore
}

type tcpCustomServerConn struct {
	net.Conn
	header *tcpCustomServer

	auth atomic.Bool
	wg   sync.WaitGroup
	once sync.Once
}

func NewConnServerTCP(c *TCPConfig, raw net.Conn) (net.Conn, error) {
	conn := &tcpCustomServerConn{
		Conn: raw,
		header: &tcpCustomServer{
			clients: c.Clients,
			servers: c.Servers,
			errors:  c.Errors,
			state:   newStateStore(5 * time.Second),
		},
	}

	conn.wg.Add(1)

	return conn, nil
}

func (c *tcpCustomServerConn) TcpMaskConn() {}

func (c *tcpCustomServerConn) RawConn() net.Conn {
	// c.wg.Wait()

	return c.Conn
}

// Splice 在握手完成前返回 false,原因同客户端:peel 只认 Splice(),
// 放行过早会让服务端的 header 校验序列根本没机会执行。
func (c *tcpCustomServerConn) Splice() bool {
	return c.auth.Load()
}

func (c *tcpCustomServerConn) Read(p []byte) (n int, err error) {
	c.once.Do(func() {
		ctx := newEvalContextWithAddrs(c.LocalAddr(), c.RemoteAddr())
		if vars, ok := c.header.state.get(tcpStateKey(c.LocalAddr(), c.RemoteAddr())); ok {
			ctx.vars = cloneVars(vars)
		}
		i := 0
		j := 0
		for i = range c.header.clients {
			if !readSequenceWithContext(c.Conn, c.header.clients[i], ctx) {
				if i < len(c.header.errors) {
					writeSequenceWithContext(c.Conn, c.header.errors[i], ctx)
				}
				c.wg.Done()
				return
			}

			if j < len(c.header.servers) {
				if !writeSequenceWithContext(c.Conn, c.header.servers[j], ctx) {
					c.wg.Done()
					return
				}
				j++
			}
		}

		for j < len(c.header.servers) {
			if !writeSequenceWithContext(c.Conn, c.header.servers[j], ctx) {
				c.wg.Done()
				return
			}
			j++
		}

		c.header.state.set(tcpStateKey(c.LocalAddr(), c.RemoteAddr()), ctx.vars)
		c.auth.Store(true)
		c.wg.Done()
	})

	c.wg.Wait()

	if !c.auth.Load() {
		return 0, errors.New("header auth failed")
	}

	return c.Conn.Read(p)
}

func (c *tcpCustomServerConn) Write(p []byte) (n int, err error) {
	c.wg.Wait()

	if !c.auth.Load() {
		return 0, errors.New("header auth failed")
	}

	return c.Conn.Write(p)
}

func readSequenceWithContext(r io.Reader, sequence *TCPSequence, ctx *evalContext) bool {
	for _, item := range sequence.Sequence {
		length, err := measureItem(item.Rand, item.Packet, item.Save, item.Var, item.Expr, sizeMapFromEvalContext(ctx))
		if err != nil {
			return false
		}
		buf := make([]byte, length)
		n, err := io.ReadFull(r, buf)
		if err != nil {
			return false
		}
		if n != length {
			return false
		}
		switch {
		case item.Rand > 0:
		case len(item.Packet) > 0:
			if !bytes.Equal(item.Packet, buf[:n]) {
				return false
			}
		case item.Var != "":
			saved, ok := ctx.vars[item.Var]
			if !ok || !bytes.Equal(saved, buf[:n]) {
				return false
			}
		case item.Expr != nil:
			evaluated, err := evaluateExpr(item.Expr, ctx)
			if err != nil {
				return false
			}
			expected, err := evaluated.asBytes()
			if err != nil || !bytes.Equal(expected, buf[:n]) {
				return false
			}
		}
		if item.Save != "" {
			ctx.vars[item.Save] = append([]byte(nil), buf[:n]...)
		}
	}
	return true
}

func writeSequenceWithContext(w io.Writer, sequence *TCPSequence, ctx *evalContext) bool {
	var merged []byte
	for _, item := range sequence.Sequence {
		if item.DelayMax > 0 {
			if len(merged) > 0 {
				_, err := w.Write(merged)
				if err != nil {
					return false
				}
				merged = nil
			}
			time.Sleep(time.Duration(crypto.RandBetween(item.DelayMin, item.DelayMax)) * time.Millisecond)
		}
		evaluated, err := evaluateItem(item.Rand, item.RandMin, item.RandMax, item.Packet, item.Save, item.Var, item.Expr, ctx)
		if err != nil {
			return false
		}
		merged = append(merged, evaluated...)
	}
	if len(merged) > 0 {
		_, err := w.Write(merged)
		if err != nil {
			return false
		}
		merged = nil
	}
	return true
}

func tcpStateKey(local, remote net.Addr) string {
	localKey := ""
	if local != nil {
		localKey = local.String()
	}
	remoteKey := ""
	if remote != nil {
		remoteKey = remote.String()
	}
	return localKey + "|" + remoteKey
}
