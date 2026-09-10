package xdns

import (
	"net"
	"sync"
	"testing"
	"time"
)

// blockingPacketConn 吞掉全部写,读阻塞到 Close。
type blockingPacketConn struct {
	done chan struct{}
}

func (p *blockingPacketConn) ReadFrom(b []byte) (int, net.Addr, error) {
	<-p.done
	return 0, nil, net.ErrClosed
}

func (p *blockingPacketConn) WriteTo(b []byte, addr net.Addr) (int, error) {
	return len(b), nil
}

func (p *blockingPacketConn) Close() error {
	select {
	case <-p.done:
	default:
		close(p.done)
	}
	return nil
}

func (p *blockingPacketConn) LocalAddr() net.Addr                { return &net.UDPAddr{} }
func (p *blockingPacketConn) SetDeadline(t time.Time) error      { return nil }
func (p *blockingPacketConn) SetReadDeadline(t time.Time) error  { return nil }
func (p *blockingPacketConn) SetWriteDeadline(t time.Time) error { return nil }

// TestClientConcurrentWriteAndClose: Close() 无锁写 closed,而
// sendLoop 每处理一个包都在无锁读 closed 并改写 resolverIdx,
// WriteTo 又在锁内读 resolverIdx——并发灌包时构成数据竞争。
// 修复前 go test -race 必报。
func TestClientConcurrentWriteAndClose(t *testing.T) {
	raw := &blockingPacketConn{done: make(chan struct{})}
	cfg := &Config{Resolvers: []string{"xdns-test:txt+udp://8.8.8.8:53"}}
	conn, err := NewConnClient(cfg, raw)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	addr := &net.UDPAddr{IP: net.ParseIP("8.8.8.8"), Port: 53}
	stop := make(chan struct{})
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				_, _ = conn.WriteTo([]byte("payload"), addr)
			}
		}()
	}

	// 让 sendLoop 高频活动后并发关闭,制造 closed/resolverIdx 读写重叠
	time.Sleep(200 * time.Millisecond)
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	close(stop)
	wg.Wait()
}
