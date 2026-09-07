package realm

import (
	"net"
	"strconv"
	"testing"
	"time"
)

// stubPacketConn 提供 NewConnServer 所需的 net.PacketConn,
// 该路径不会真正收发网络数据。
type stubPacketConn struct{}

func (stubPacketConn) ReadFrom(p []byte) (int, net.Addr, error) { return 0, nil, net.ErrClosed }
func (stubPacketConn) WriteTo(p []byte, a net.Addr) (int, error) { return 0, net.ErrClosed }
func (stubPacketConn) Close() error                              { return nil }
func (stubPacketConn) LocalAddr() net.Addr                       { return &net.UDPAddr{IP: net.IPv4zero} }
func (stubPacketConn) SetDeadline(t time.Time) error             { return nil }
func (stubPacketConn) SetReadDeadline(t time.Time) error         { return nil }
func (stubPacketConn) SetWriteDeadline(t time.Time) error        { return nil }

// TestConnServerCloseAfterRegisterFailure 覆盖 run() 的"注册失败 +
// ctx 取消"退出路径:修复前该路径手工 wg.Done() 与 wg.Go 的自动 Done
// 叠加成负计数,run goroutine 退出时以 panic 杀死整个进程。
func TestConnServerCloseAfterRegisterFailure(t *testing.T) {
	// 监听后立即关闭,得到一个必拒的本地端口作为 realm 服务器地址
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()

	cfg := &Config{
		Scheme: "http",
		Host:   "127.0.0.1",
		Port:   strconv.Itoa(port),
		ID:     "test-id",
	}

	conn, err := NewConnServer(cfg, stubPacketConn{})
	if err != nil {
		t.Fatal(err)
	}

	// 等 run() 的 Register 失败落地(连接拒绝,毫秒级)
	time.Sleep(50 * time.Millisecond)

	// Close 取消 ctx,run() 应从 backoff 等待中退出;
	// 修复前此处在 run goroutine 内触发 negative WaitGroup counter panic
	if err := conn.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// 给 run goroutine 留出退出时间,让潜在的 panic 暴露出来
	time.Sleep(200 * time.Millisecond)
}
