package inbound

import (
	"context"
	"testing"
	"time"

	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/session"
	"github.com/xtls/xray-core/common/tracker"
	finbound "github.com/xtls/xray-core/features/inbound"
	"github.com/xtls/xray-core/transport/internet/stat"
)

// TestInstanceTableHoldsInboundAndOutbound checks what makes the table global:
// the connections an inbound handler serves and the connections an outbound
// handler dialed are in the same table of the instance, so one snapshot and one
// close reach both.
func TestInstanceTableHoldsInboundAndOutbound(t *testing.T) {
	manager := &Manager{conns: tracker.NewRegistry()}
	registry := manager.Connections()

	// An inbound handler writes into the instance table, under itself as owner.
	handler := &AlwaysOnInboundHandler{tag: "in"}
	handler.sessions = sharedBy(registry, handler)

	served := &fakeSession{}
	if _, ok := handler.sessions.register(&connection{
		Inbound: &session.Inbound{Tag: "in", Source: net.TCPDestination(net.LocalHostIP, 1000)},
		Network: net.Network_TCP,
		Start:   time.Now(),
		Conn:    served,
	}); !ok {
		t.Fatal("the instance table refused an inbound connection")
	}

	// An outbound handler writes into the same table, under itself.
	outboundOwner := &struct{ tag string }{"proxy"}
	if !registry.Register(&tracker.Entry{
		Owner:       outboundOwner,
		Direction:   tracker.Outbound,
		OutboundTag: "proxy",
		Destination: net.TCPDestination(net.DomainAddress("example.com"), 443),
		Network:     net.Network_TCP,
		Start:       time.Now(),
		Conn:        &fakeSession{},
	}) {
		t.Fatal("the instance table refused an outbound connection")
	}

	// The instance snapshot holds both, and each handler only sees its own.
	sessions := manager.Sessions(context.Background())
	if len(sessions) != 2 {
		t.Fatalf("instance snapshot holds %d connections, want 2", len(sessions))
	}
	var inboundSeen, outboundSeen bool
	for _, s := range sessions {
		switch s.Direction {
		case finbound.DirectionInbound:
			inboundSeen = true
		case finbound.DirectionOutbound:
			outboundSeen = true
		}
	}
	if !inboundSeen || !outboundSeen {
		t.Errorf("instance snapshot does not report both directions: %+v", sessions)
	}
	if got := handler.Sessions(context.Background()); len(got) != 1 || got[0].InboundTag != "in" {
		t.Errorf("the handler sees %+v, want only its own connection", got)
	}

	// Closing through the instance reaches the outbound connection too, which is
	// the point of having one table: a removed user's dialed connection must not
	// outlive the removal.
	closed := manager.CloseSessions(context.Background(), func(s finbound.Session) bool {
		return s.Direction == finbound.DirectionOutbound
	})
	if closed != 1 {
		t.Errorf("instance closed %d connections, want 1", closed)
	}
	if served.isClosed() {
		t.Error("closing outbound sessions closed an inbound connection")
	}
}

// TestInstanceCloseStopsTheTable checks that closing the instance refuses new
// connections and closes the tracked ones, including those of outbound handlers,
// which no inbound handler closes.
func TestInstanceCloseStopsTheTable(t *testing.T) {
	manager := &Manager{
		taggedHandlers: make(map[string]finbound.Handler),
		conns:          tracker.NewRegistry(),
	}
	conn := &fakeSession{}
	manager.Connections().Register(&tracker.Entry{
		Owner:     "outbound",
		Direction: tracker.Outbound,
		Network:   net.Network_TCP,
		Conn:      conn,
	})

	if err := manager.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
	if !conn.isClosed() {
		t.Error("closing the instance did not close a dialed connection")
	}
	if manager.Connections().Register(&tracker.Entry{Owner: "late", Network: net.Network_TCP, Conn: &fakeSession{}}) {
		t.Error("a closed instance accepted a new connection")
	}
}

// fakeSession is a connection that records whether it was closed.
type fakeSession struct {
	stat.Connection
	closed bool
}

func (c *fakeSession) Close() error {
	c.closed = true
	return nil
}

func (c *fakeSession) isClosed() bool {
	return c.closed
}
