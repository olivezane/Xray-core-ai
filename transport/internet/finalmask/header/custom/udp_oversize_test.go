package custom

import (
	"net"
	"testing"
	"time"

	"github.com/xtls/xray-core/transport/internet"
)

// Oversized payloads must be dropped, not silently truncated and padded
// with stale scratch-buffer bytes: the masked datagram would otherwise
// exceed the 4096-byte cap the readers and the multi-mask manager rely on.
func TestDSLUDPClientWriteToDropsOversize(t *testing.T) {
	peer, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()

	raw, err := net.ListenUDP("udp", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()

	conn, err := NewConnClientUDP(&UDPConfig{
		Client: []*UDPItem{{Rand: 4}},
	}, raw)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// masked size would be 4+4096 > 4096: must be dropped, never emitted.
	n, err := conn.WriteTo(make([]byte, internet.UDPSize), peer.LocalAddr())

	peer.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	buf := make([]byte, 8192)
	rn, _, rerr := peer.ReadFrom(buf)
	if rerr == nil {
		t.Fatalf("oversized payload was emitted as a %d-byte corrupt datagram", rn)
	}
	if ne, ok := rerr.(net.Error); !ok || !ne.Timeout() {
		t.Fatalf("unexpected read error: %v", rerr)
	}
	if n != 0 || err != nil {
		t.Fatalf("oversized WriteTo: got n=%d err=%v, want drop (0, nil)", n, err)
	}
}

// Exact-fit payloads (header + payload == 4096) must still pass through intact.
func TestDSLUDPClientWriteToExactFitPasses(t *testing.T) {
	peer, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()

	raw, err := net.ListenUDP("udp", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()

	conn, err := NewConnClientUDP(&UDPConfig{
		Client: []*UDPItem{{Rand: 4}},
	}, raw)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	payload := make([]byte, internet.UDPSize-4)
	for i := range payload {
		payload[i] = byte(i % 251)
	}
	n, err := conn.WriteTo(payload, peer.LocalAddr())
	if n != len(payload) || err != nil {
		t.Fatalf("exact-fit WriteTo: got n=%d err=%v", n, err)
	}

	peer.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	buf := make([]byte, 8192)
	rn, _, rerr := peer.ReadFrom(buf)
	if rerr != nil {
		t.Fatalf("exact-fit payload was not emitted: %v", rerr)
	}
	if rn != internet.UDPSize {
		t.Fatalf("wire size: got %d, want %d", rn, internet.UDPSize)
	}
	if string(buf[4:rn]) != string(payload) {
		t.Fatal("wire payload differs from input")
	}
}
