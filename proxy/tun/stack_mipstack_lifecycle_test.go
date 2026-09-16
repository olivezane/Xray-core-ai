package tun

import (
	"context"
	stdnet "net"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/xtls/xray-core/common/buf"
)

// closeWithin closes the stack and fails the test when it does not return
func closeWithin(t *testing.T, stack Stack, within time.Duration) {
	t.Helper()

	done := make(chan error, 1)
	go func() { done <- stack.Close() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("close the stack: %v", err)
		}
	case <-time.After(within):
		t.Fatalf("the stack did not close within %s", within)
	}
}

func TestHandlerLifecycleWithMipsStack(t *testing.T) {
	device := newBridgeDevice()
	handler := newTestHandler(device)
	handler.config.Stack = StackMipstack
	handler.config.MTU = mipstackDefaultMTU

	if err := handler.Start(); err != nil {
		t.Fatalf("start the handler: %v", err)
	}
	if handler.tun != device {
		t.Fatal("the handler must hold the device after a successful Start")
	}
	underTest, ok := handler.stack.(*stackMipstack)
	if !ok {
		t.Fatalf("the handler holds %T, want the MIPS Stack", handler.stack)
	}

	// a successful Close waits for the pumps of the stack, so it returns only
	// once no pump keeps reading the device
	closeWithin(t, handler, 10*time.Second)
	if !underTest.closed.Load() {
		t.Fatal("the handler must close the MIPS Stack")
	}
	select {
	case <-device.closed:
	default:
		t.Fatal("the handler must close the device")
	}

	// closing twice is safe
	closeWithin(t, handler, 10*time.Second)
}

func TestMipstackClosesUnderTraffic(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	device := newBridgeDevice()
	handler := newRecordingConnectionHandler()
	stack, err := newMipstack(ctx, StackOptions{
		Tun:         device,
		MTU:         mipstackDefaultMTU,
		IdleTimeout: time.Minute,
	}, handler)
	if err != nil {
		t.Fatalf("create the MIPS Stack: %v", err)
	}
	if err := stack.Start(); err != nil {
		t.Fatalf("start the MIPS Stack: %v", err)
	}
	underTest := stack.(*stackMipstack)

	peer := newBridgeMipstackPeer(t, device, netip.MustParsePrefix("10.0.0.2/24"))
	defer peer.Close()
	defer device.close()

	// a client that keeps sending while the stack goes down
	client, err := peer.stack.ListenUDP(ctx, "udp", netip.AddrPortFrom(netip.MustParseAddr("10.0.0.2"), 0))
	if err != nil {
		t.Fatalf("open the peer's udp socket: %v", err)
	}
	defer client.Close()

	server := stdnet.UDPAddrFromAddrPort(netip.MustParseAddrPort("192.0.2.10:10000"))
	if _, err := client.WriteTo([]byte("ping"), server); err != nil {
		t.Fatalf("send the first datagram: %v", err)
	}

	accepted := handler.next(t, 10*time.Second)
	reader, ok := accepted.connection.(buf.Reader)
	if !ok {
		t.Fatalf("the intercepted UDP session is no buffer reader: %T", accepted.connection)
	}
	readDatagrams(t, reader, 1)

	sending := make(chan struct{})
	var senders sync.WaitGroup
	senders.Add(1)
	go func() {
		defer senders.Done()
		defer close(sending)
		for i := 0; i < 200; i++ {
			_, _ = client.WriteTo([]byte("ping"), server)
			time.Sleep(time.Millisecond)
		}
	}()

	closeWithin(t, stack, 10*time.Second)
	senders.Wait()
	<-sending

	// nothing the closed stack held survives it
	if got := responderCount(underTest); got != 0 {
		t.Fatalf("the closed stack holds %d UDP Responders, want 0", got)
	}
	closeWithin(t, stack, 10*time.Second)
}
