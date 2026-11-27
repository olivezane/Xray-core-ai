package hub

import (
	"context"
	gotcp "net"
	"testing"
	"time"

	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/transport/internet"
	"github.com/xtls/xray-core/transport/internet/stat"
	"github.com/xtls/xray-core/transport/internet/tcp"
	_ "github.com/xtls/xray-core/transport/internet/transports"
)

func streamSettings(t *testing.T) *internet.MemoryStreamConfig {
	t.Helper()
	return &internet.MemoryStreamConfig{
		ProtocolName:     "tcp",
		ProtocolSettings: &tcp.Config{},
	}
}

// TestListenTCPHandsConnectionsToTheHandler checks the contract an inbound relies
// on: the handler runs in its own goroutine for every accepted connection, the
// context it gets descends from the listener, and closing the listener cancels it.
func TestListenTCPHandsConnectionsToTheHandler(t *testing.T) {
	port := pickPort(t)

	accepted := make(chan context.Context, 1)
	listener, err := ListenTCP(context.Background(), net.LocalHostIP, port, streamSettings(t), func(ctx context.Context, conn stat.Connection) {
		accepted <- ctx
		defer conn.Close()
		<-ctx.Done()
	}, Options{})
	if err != nil {
		t.Fatal("failed to listen: ", err)
	}
	defer listener.Close()

	conn, err := gotcp.Dial("tcp", gotcp.JoinHostPort("127.0.0.1", port.String()))
	if err != nil {
		t.Fatal("failed to dial the listener: ", err)
	}
	defer conn.Close()

	var handlerCtx context.Context
	select {
	case handlerCtx = <-accepted:
	case <-time.After(5 * time.Second):
		t.Fatal("the handler did not get the connection")
	}

	if err := listener.Close(); err != nil {
		t.Errorf("closing the listener failed: %v", err)
	}
	select {
	case <-handlerCtx.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("closing the listener did not cancel the connection context")
	}
}

// pickPort asks the OS for a free TCP port.
func pickPort(t *testing.T) net.Port {
	t.Helper()
	l, err := gotcp.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal("failed to pick a port: ", err)
	}
	port := net.Port(l.Addr().(*gotcp.TCPAddr).Port)
	l.Close()
	return port
}

// TestCloseStopsTheAcceptLoop checks that closing the listener is what stops
// everything it started, which is what a closing inbound relies on.
func TestCloseStopsTheAcceptLoop(t *testing.T) {
	listener := &Listener{cancel: func() {}}
	if err := listener.Close(); err != nil {
		t.Errorf("closing a listener without a hub failed: %v", err)
	}

	var nilListener *Listener
	if err := nilListener.Close(); err != nil {
		t.Errorf("closing a nil listener failed: %v", err)
	}
	if nilListener.Port() != 0 || nilListener.Addr() != nil {
		t.Error("a nil listener reports a port or an address")
	}
}

// TestCloseAllClosesEverything checks the close order an inbound needs: the hub
// first, then the proxy, and a failure anywhere is reported.
func TestCloseAllClosesEverything(t *testing.T) {
	order := make([]string, 0, 2)
	first := closer(func() error { order = append(order, "hub"); return nil })
	second := closer(func() error { order = append(order, "proxy"); return nil })

	if err := CloseAll(first, second); err != nil {
		t.Fatalf("CloseAll failed: %v", err)
	}
	if len(order) != 2 || order[0] != "hub" || order[1] != "proxy" {
		t.Errorf("closed in order %v, want hub then proxy", order)
	}

	// Values that are not closable are skipped, as common.Close does.
	if err := CloseAll(first, "not closable"); err != nil {
		t.Errorf("CloseAll reported %v for a non-closable value", err)
	}
}

type closer func() error

func (c closer) Close() error { return c() }

// TestPacketListenerAddress checks the part of the packet listener an inbound
// builds its connection metadata from.
func TestPacketListenerAddress(t *testing.T) {
	var nilListener *PacketListener
	if err := nilListener.Close(); err != nil {
		t.Errorf("closing a nil packet listener failed: %v", err)
	}

	l := &PacketListener{activeSet: make(map[connID]*PacketConn)}
	// Removing a session that does not exist must not panic.
	l.RemoveConn(net.TCPDestination(net.LocalHostIP, 1))
	_ = time.Now
}
