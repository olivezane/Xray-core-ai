package multiplex

import (
	"testing"

	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/session"
	"github.com/xtls/xray-core/common/tracker"
)

// TestStreamRowIsItsOwnEntry checks what makes a multiplexed connection readable
// in a snapshot: each mux session is its own row, carrying the stream id and the
// target, so it can be told apart from the carrier connection and from the other
// streams sharing it.
func TestStreamRowIsItsOwnEntry(t *testing.T) {
	registry := tracker.NewRegistry()

	carrier := &tracker.Entry{
		Owner:       "carrier",
		Direction:   tracker.Outbound,
		OutboundTag: "proxy",
		Destination: net.TCPDestination(net.DomainAddress("proxy.example"), 443),
		Network:     net.Network_TCP,
	}
	if !registry.Register(carrier) {
		t.Fatal("table refused the carrier connection")
	}

	const streamID = 7
	stream := &tracker.Entry{
		Owner:       "stream",
		Direction:   tracker.Outbound,
		Destination: net.TCPDestination(net.DomainAddress("example.com"), 80),
		Network:     net.Network_TCP,
		Stream:      streamID,
		Inbound:     &session.Inbound{Tag: "in"},
	}
	if !trackStreamIn(registry, stream) {
		t.Fatal("table refused the stream row")
	}

	sessions := registry.Sessions()
	if len(sessions) != 2 {
		t.Fatalf("table holds %d rows, want the carrier and the stream", len(sessions))
	}
	var carrierSeen, streamSeen bool
	for _, s := range sessions {
		switch s.Stream {
		case 0:
			carrierSeen = true
		case streamID:
			streamSeen = true
			if s.Destination.Address.String() != "example.com" {
				t.Errorf("stream row destination = %v", s.Destination)
			}
			if s.InboundTag != "in" {
				t.Errorf("stream row inbound tag = %q, want in", s.InboundTag)
			}
		default:
			t.Errorf("unexpected stream id %d", s.Stream)
		}
	}
	if !carrierSeen || !streamSeen {
		t.Fatal("the snapshot does not tell the carrier from its streams")
	}

	// A stream row can be dropped on its own, leaving the carrier in place.
	registry.Unregister(stream)
	if got := registry.Sessions(); len(got) != 1 || got[0].Stream != 0 {
		t.Errorf("after dropping the stream the table holds %+v", got)
	}
}
