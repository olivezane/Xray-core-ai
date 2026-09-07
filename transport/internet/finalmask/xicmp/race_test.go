package xicmp

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/xtls/xray-core/transport/internet"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

type stubPC struct{}

func (stubPC) ReadFrom(b []byte) (int, net.Addr, error)  { return 0, nil, net.ErrClosed }
func (stubPC) WriteTo(b []byte, a net.Addr) (int, error) { return len(b), nil }
func (stubPC) Close() error                              { return nil }
func (stubPC) LocalAddr() net.Addr                       { return &net.UDPAddr{} }
func (stubPC) SetDeadline(t time.Time) error             { return nil }
func (stubPC) SetReadDeadline(t time.Time) error         { return nil }
func (stubPC) SetWriteDeadline(t time.Time) error        { return nil }

// TestClientConcurrentWriteAndEchoReply: recv 循环无锁读 c.seq 做
// 窗口过滤,WriteTo 在锁内改写 c.seq——并发收发时构成数据竞争。
// 修复前 go test -race 必报。
func TestClientConcurrentWriteAndEchoReply(t *testing.T) {
	cfg := &Config{DGRAM: true} // udp4 模式,无需特权 icmp socket
	conn, err := NewConnClient(cfg, stubPC{})
	if err != nil {
		t.Skipf("cannot open icmp socket: %v", err)
	}
	defer conn.Close()

	client := conn.(*xicmpConnClient)
	local := client.icmp4.LocalAddr().(*net.UDPAddr)
	injector, err := net.DialUDP("udp4", nil, local)
	if err != nil {
		t.Fatal(err)
	}
	defer injector.Close()

	stop := make(chan struct{})
	// 消费者:让 recv4 命中后不阻塞在 readCh
	go func() {
		buf := make([]byte, internet.UDPSize)
		for {
			if _, _, err := conn.ReadFrom(buf); err != nil {
				return
			}
		}
	}()
	// 注入器:持续向自身 icmp4 socket 回灌 echo reply,触发 recv4 读 c.seq
	go func() {
		seq := 1
		for {
			select {
			case <-stop:
				return
			default:
			}
			reply, err := (&icmp.Message{
				Type: ipv4.ICMPTypeEchoReply,
				Code: 0,
				Body: &icmp.Echo{ID: 1, Seq: seq, Data: make([]byte, 8)},
			}).Marshal(nil)
			if err == nil {
				_, _ = injector.Write(reply)
			}
			seq++
			time.Sleep(200 * time.Microsecond)
		}
	}()

	// 并发写:锁内推进 c.seq
	addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1")}
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 500 {
				_, _ = conn.WriteTo([]byte("payload"), addr)
			}
		}()
	}
	wg.Wait()

	time.Sleep(100 * time.Millisecond)
	close(stop)
}
