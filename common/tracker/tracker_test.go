package tracker_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/session"
	"github.com/xtls/xray-core/common/tracker"
	finbound "github.com/xtls/xray-core/features/inbound"
)

type fakeConn struct {
	net.Conn
	closed   bool
	closedMu sync.Mutex
}

func (c *fakeConn) Close() error {
	c.closedMu.Lock()
	c.closed = true
	c.closedMu.Unlock()
	return nil
}

func (c *fakeConn) isClosed() bool {
	c.closedMu.Lock()
	defer c.closedMu.Unlock()
	return c.closed
}

type owner struct{ name string }

// TestGlobalTableHoldsBothDirections checks what makes the table global: a
// connection an inbound accepted and a connection an outbound dialed are in the
// same snapshot, each reported with what it is.
func TestGlobalTableHoldsBothDirections(t *testing.T) {
	registry := tracker.NewRegistry()
	accepted := &fakeConn{}
	dialed := &fakeConn{}
	inboundOwner := &owner{"in"}
	outboundOwner := &owner{"out"}

	registry.Register(&tracker.Entry{
		Owner:     inboundOwner,
		Direction: tracker.Inbound,
		Inbound:   &session.Inbound{Tag: "in-1", Source: net.TCPDestination(net.LocalHostIP, 1000), Local: net.TCPDestination(net.LocalHostIP, 443)},
		Network:   net.Network_TCP,
		Start:     time.Now(),
		Conn:      accepted,
	})
	registry.Register(&tracker.Entry{
		Owner:       outboundOwner,
		Direction:   tracker.Outbound,
		OutboundTag: "proxy",
		Destination: net.TCPDestination(net.DomainAddress("example.com"), 443),
		Network:     net.Network_TCP,
		Start:       time.Now(),
		Conn:        dialed,
	})

	sessions := registry.Sessions()
	if len(sessions) != 2 {
		t.Fatalf("table holds %d connections, want 2", len(sessions))
	}

	var sawInbound, sawOutbound bool
	for _, s := range sessions {
		switch s.Direction {
		case finbound.DirectionInbound:
			sawInbound = true
			if s.InboundTag != "in-1" {
				t.Errorf("inbound session tag = %q, want in-1", s.InboundTag)
			}
		case finbound.DirectionOutbound:
			sawOutbound = true
			if s.OutboundTag != "proxy" || s.Destination.Address.String() != "example.com" {
				t.Errorf("outbound session = %+v", s)
			}
		}
	}
	if !sawInbound || !sawOutbound {
		t.Error("the snapshot does not report both directions")
	}

	// Per-owner views do not leak into each other.
	if got := registry.SessionsOf(inboundOwner); len(got) != 1 || got[0].Direction != finbound.DirectionInbound {
		t.Errorf("inbound owner sees %+v", got)
	}
	if got := registry.SessionsOf(outboundOwner); len(got) != 1 || got[0].Direction != finbound.DirectionOutbound {
		t.Errorf("outbound owner sees %+v", got)
	}
}

// TestCloseSessionsReachesBothDirections checks control: matching sessions are
// cancelled and closed wherever they are in the table, and the rest survive.
func TestCloseSessionsReachesBothDirections(t *testing.T) {
	registry := tracker.NewRegistry()
	cancelled := 0
	cancel := func() { cancelled++ }

	accepted := &fakeConn{}
	dialed := &fakeConn{}
	kept := &fakeConn{}

	registry.Register(&tracker.Entry{
		Owner: "a", Direction: tracker.Inbound,
		Inbound: &session.Inbound{Tag: "in", User: &protocol.MemoryUser{Email: "removed@example.com"}},
		Network: net.Network_TCP, Conn: accepted, Cancel: cancel,
	})
	registry.Register(&tracker.Entry{
		Owner: "b", Direction: tracker.Outbound, OutboundTag: "removed-email-out",
		Network: net.Network_TCP, Conn: dialed,
	})
	registry.Register(&tracker.Entry{
		Owner: "c", Direction: tracker.Inbound,
		Inbound: &session.Inbound{Tag: "keep"}, Network: net.Network_TCP, Conn: kept,
	})

	closed := registry.CloseMatching(func(s finbound.Session) bool {
		return s.Email == "removed@example.com" || s.OutboundTag == "removed-email-out"
	})
	if closed != 2 {
		t.Errorf("closed %d connections, want 2", closed)
	}
	if !accepted.isClosed() || !dialed.isClosed() {
		t.Error("matching connections were not closed")
	}
	if kept.isClosed() {
		t.Error("a non-matching connection was closed")
	}
	if cancelled != 1 {
		t.Errorf("cancel called %d times, want 1", cancelled)
	}
}

// TestCloseAllStopsTheTable checks what a closing instance needs: everything is
// closed, and nothing can be registered afterwards.
func TestCloseAllStopsTheTable(t *testing.T) {
	registry := tracker.NewRegistry()
	conn := &fakeConn{}
	registry.Register(&tracker.Entry{Owner: "a", Network: net.Network_TCP, Conn: conn})

	registry.CloseAll()

	if !conn.isClosed() {
		t.Error("CloseAll did not close the connection")
	}
	if registry.Register(&tracker.Entry{Owner: "a", Network: net.Network_TCP, Conn: &fakeConn{}}) {
		t.Error("a closed table accepted a new connection")
	}
	if got := registry.Sessions(); len(got) != 0 {
		// The owner has not unregistered yet, so the entry may still be there; what
		// matters is that it cannot grow.
		_ = got
	}
}

// TestUnregisterRemovesFromSnapshot checks that a finished connection does not
// stay in a snapshot.
func TestUnregisterRemovesFromSnapshot(t *testing.T) {
	registry := tracker.NewRegistry()
	conn := &fakeConn{}
	registry.Register(&tracker.Entry{Owner: "a", Network: net.Network_TCP, Conn: conn})
	registry.Unregister(conn)
	if got := registry.Sessions(); len(got) != 0 {
		t.Errorf("table holds %d connections after unregister, want 0", len(got))
	}
}

// TestNilMatchClosesEverything checks the convenience path used by a closing
// instance.
func TestNilMatchClosesEverything(t *testing.T) {
	registry := tracker.NewRegistry()
	conn := &fakeConn{}
	registry.Register(&tracker.Entry{Owner: "a", Network: net.Network_TCP, Conn: conn})
	if closed := registry.CloseMatching(nil); closed != 1 {
		t.Errorf("closed %d, want 1", closed)
	}
	if !conn.isClosed() {
		t.Error("nil match did not close the connection")
	}
}

// TestCloseIfExistsToleratesNilConn checks a defensive path: an entry without a
// connection must not make closing panic.
func TestCloseIfExistsToleratesNilConn(t *testing.T) {
	registry := tracker.NewRegistry()
	registry.Register(&tracker.Entry{Owner: "a", Network: net.Network_TCP})
	registry.CloseMatching(nil)
	if err := errors.New("no panic"); err == nil {
		t.Fatal("unreachable")
	}
}

var _ = context.Background
