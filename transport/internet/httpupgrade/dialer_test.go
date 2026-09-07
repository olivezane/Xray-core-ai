package httpupgrade

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	cnet "github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/transport/internet"
)

// TestDialClosesConnOnUpgradeRejected: 服务器对 upgrade 请求回非 101
// 应答时,dial 已建立的 TCP 连接必须被关闭,不能泄漏给上层。
func TestDialClosesConnOnUpgradeRejected(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	serverDone := make(chan error, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			serverDone <- err
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
		br := bufio.NewReader(conn)
		if _, err := http.ReadRequest(br); err != nil {
			serverDone <- err
			return
		}
		if _, err := conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n")); err != nil {
			serverDone <- err
			return
		}
		// 客户端必须主动关闭:这里读到 EOF 才算关
		buf := make([]byte, 1)
		_, err = conn.Read(buf)
		serverDone <- err
	}()

	dest := cnet.TCPDestination(cnet.LocalHostIP, cnet.Port(ln.Addr().(*net.TCPAddr).Port))
	mss := &internet.MemoryStreamConfig{
		ProtocolSettings: &Config{Path: "ws"},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	conn, err := dialhttpUpgrade(ctx, dest, mss)
	if err == nil {
		if conn != nil {
			_ = conn.Close()
		}
		t.Fatal("expected dial error for non-upgrade reply")
	}

	select {
	case err := <-serverDone:
		if err != io.EOF {
			t.Fatalf("server read %v, want io.EOF (client must close the conn)", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server never saw EOF: client leaked the established conn")
	}
}
