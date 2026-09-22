package tun

import (
	"context"
	stdnet "net"
	"net/netip"
	"testing"
	"time"

	"github.com/xtls/xray-core/common/buf"
	xnet "github.com/xtls/xray-core/common/net"
)

// datagram is one datagram the inbound received, with the destination it was
// addressed to
type datagram struct {
	payload     string
	destination xnet.Destination
}

func readDatagrams(t *testing.T, reader buf.Reader, count int) []datagram {
	t.Helper()

	received := make(chan datagram, count)
	failures := make(chan error, 1)
	go func() {
		for read := 0; read < count; read++ {
			buffers, err := reader.ReadMultiBuffer()
			if err != nil {
				failures <- err
				return
			}
			for _, buffer := range buffers {
				datagram := datagram{payload: string(buffer.Bytes())}
				if buffer.UDP != nil {
					datagram.destination = *buffer.UDP
				}
				received <- datagram
				buffer.Release()
			}
		}
	}()

	datagrams := make([]datagram, 0, count)
	for len(datagrams) < count {
		select {
		case datagram := <-received:
			datagrams = append(datagrams, datagram)
		case err := <-failures:
			t.Fatalf("read datagrams: %v", err)
		case <-time.After(10 * time.Second):
			t.Fatalf("read %d datagrams, got %d", count, len(datagrams))
		}
	}

	return datagrams
}

func writeDatagram(t *testing.T, writer buf.Writer, payload string, destination xnet.Destination) {
	t.Helper()

	buffer := buf.New()
	if _, err := buffer.WriteString(payload); err != nil {
		buffer.Release()
		t.Fatalf("build datagram: %v", err)
	}
	buffer.UDP = &destination
	if err := writer.WriteMultiBuffer(buf.MultiBuffer{buffer}); err != nil {
		t.Fatalf("reply with %q: %v", payload, err)
	}
}

// peerReplies reads the given number of replies and reports, per payload, the
// address each one came from
func peerReplies(t *testing.T, listener stdnet.PacketConn, count int) map[string]string {
	t.Helper()

	sources := make(map[string]string, count)
	for len(sources) < count {
		if err := listener.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
			t.Fatalf("set read deadline: %v", err)
		}
		received := make([]byte, 64)
		n, from, err := listener.ReadFrom(received)
		if err != nil {
			t.Fatalf("peer received %d of %d replies: %v", len(sources), count, err)
		}
		sources[string(received[:n])] = from.String()
	}

	return sources
}

func TestMipstackRepliesToUDPFromEachInterceptedDestination(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
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

	peer := newBridgeMipstackPeer(t, device, netip.MustParsePrefix("10.0.0.2/24"))
	defer peer.Close()
	defer device.close()
	defer stack.Close()

	// one client socket addressing two destinations, which is what FullCone
	// NAT has to carry within a single session
	client, err := peer.stack.ListenUDP(ctx, "udp", netip.AddrPortFrom(netip.MustParseAddr("10.0.0.2"), 0))
	if err != nil {
		t.Fatalf("open the peer's udp socket: %v", err)
	}
	defer client.Close()

	serverA := netip.MustParseAddrPort("192.0.2.10:10000")
	serverB := netip.MustParseAddrPort("192.0.2.11:10001")
	for payload, server := range map[string]netip.AddrPort{"to-a": serverA, "to-b": serverB} {
		if _, err := client.WriteTo([]byte(payload), stdnet.UDPAddrFromAddrPort(server)); err != nil {
			t.Fatalf("send %q: %v", payload, err)
		}
	}

	// the inbound sees one session, and every datagram of it carries the
	// destination it was addressed to
	accepted := handler.next(t, 10*time.Second)
	reader, ok := accepted.connection.(buf.Reader)
	if !ok {
		t.Fatalf("the intercepted UDP session is no buffer reader: %T", accepted.connection)
	}
	writer, ok := accepted.connection.(buf.Writer)
	if !ok {
		t.Fatalf("the intercepted UDP session is no buffer writer: %T", accepted.connection)
	}

	want := map[string]netip.AddrPort{"to-a": serverA, "to-b": serverB}
	for _, datagram := range readDatagrams(t, reader, len(want)) {
		target, ok := want[datagram.payload]
		if !ok {
			t.Fatalf("the inbound received %q, want a datagram the peer sent", datagram.payload)
		}
		if datagram.destination.NetAddr() != target.String() {
			t.Fatalf("datagram %q was addressed to %s, want %v", datagram.payload, datagram.destination.NetAddr(), target)
		}
		delete(want, datagram.payload)
	}
	if len(want) != 0 {
		t.Fatalf("the inbound never received %v", want)
	}

	// every reply keeps the address the client addressed, so a client told
	// apart by source sees each remote answer as itself
	writeDatagram(t, writer, "from-a", xnet.UDPDestination(xnet.ParseAddress(serverA.Addr().String()), xnet.Port(serverA.Port())))
	writeDatagram(t, writer, "from-b", xnet.UDPDestination(xnet.ParseAddress(serverB.Addr().String()), xnet.Port(serverB.Port())))

	sources := peerReplies(t, client, 2)
	if sources["from-a"] != serverA.String() {
		t.Fatalf("the reply to the first destination came from %s, want %v", sources["from-a"], serverA)
	}
	if sources["from-b"] != serverB.String() {
		t.Fatalf("the reply to the second destination came from %s, want %v", sources["from-b"], serverB)
	}
}

// responderCount reports how many UDP Responders the stack still holds
func responderCount(stack *stackMipstack) int {
	stack.udpMu.Lock()
	defer stack.udpMu.Unlock()
	return len(stack.responders)
}

// waitForResponderCount waits until the stack holds the wanted number of UDP
// Responders
func waitForResponderCount(t *testing.T, stack *stackMipstack, want int) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for {
		if got := responderCount(stack); got == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the stack holds %d UDP Responders, want %d", responderCount(stack), want)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestMipstackReusesAndReleasesUDPResponders(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
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

	peer := newBridgeMipstackPeer(t, device, netip.MustParsePrefix("10.0.0.2/24"))
	defer peer.Close()
	defer device.close()
	defer stack.Close()
	underTest := stack.(*stackMipstack)

	client, err := peer.stack.ListenUDP(ctx, "udp", netip.AddrPortFrom(netip.MustParseAddr("10.0.0.2"), 0))
	if err != nil {
		t.Fatalf("open the peer's udp socket: %v", err)
	}
	defer client.Close()

	server := stdnet.UDPAddrFromAddrPort(netip.MustParseAddrPort("192.0.2.10:10000"))
	for i := 0; i < 3; i++ {
		if _, err := client.WriteTo([]byte("ping"), server); err != nil {
			t.Fatalf("send datagram %d: %v", i, err)
		}
	}

	// every datagram of one flow reuses the Responder of that flow
	accepted := handler.next(t, 10*time.Second)
	reader, ok := accepted.connection.(buf.Reader)
	if !ok {
		t.Fatalf("the intercepted UDP session is no buffer reader: %T", accepted.connection)
	}
	readDatagrams(t, reader, 3)
	if got := responderCount(underTest); got != 1 {
		t.Fatalf("the stack holds %d UDP Responders for one flow, want 1", got)
	}

	// the session that owns the flow releases them when it ends
	if err := accepted.connection.Close(); err != nil {
		t.Fatalf("close the intercepted session: %v", err)
	}
	waitForResponderCount(t, underTest, 0)
}

func TestMipstackReleasesIdleUDPResponders(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	device := newBridgeDevice()
	handler := newRecordingConnectionHandler()
	stack, err := newMipstack(ctx, StackOptions{
		Tun:         device,
		MTU:         mipstackDefaultMTU,
		IdleTimeout: 100 * time.Millisecond,
	}, handler)
	if err != nil {
		t.Fatalf("create the MIPS Stack: %v", err)
	}
	if err := stack.Start(); err != nil {
		t.Fatalf("start the MIPS Stack: %v", err)
	}

	peer := newBridgeMipstackPeer(t, device, netip.MustParsePrefix("10.0.0.2/24"))
	defer peer.Close()
	defer device.close()
	defer stack.Close()
	underTest := stack.(*stackMipstack)

	client, err := peer.stack.ListenUDP(ctx, "udp", netip.AddrPortFrom(netip.MustParseAddr("10.0.0.2"), 0))
	if err != nil {
		t.Fatalf("open the peer's udp socket: %v", err)
	}
	defer client.Close()

	server := stdnet.UDPAddrFromAddrPort(netip.MustParseAddrPort("192.0.2.10:10000"))
	if _, err := client.WriteTo([]byte("ping"), server); err != nil {
		t.Fatalf("send datagram: %v", err)
	}

	accepted := handler.next(t, 10*time.Second)
	reader, ok := accepted.connection.(buf.Reader)
	if !ok {
		t.Fatalf("the intercepted UDP session is no buffer reader: %T", accepted.connection)
	}
	readDatagrams(t, reader, 1)
	if got := responderCount(underTest); got != 1 {
		t.Fatalf("the stack holds %d UDP Responders for one flow, want 1", got)
	}

	// the flow stays unused, so Connection Idle releases it even though the
	// session is still open
	waitForResponderCount(t, underTest, 0)

	writer, ok := accepted.connection.(buf.Writer)
	if !ok {
		t.Fatalf("the intercepted UDP session is no buffer writer: %T", accepted.connection)
	}

	// a reply that arrives after the release leaves no mapping, so it is
	// dropped while the session lives on: the next datagram the client sends
	// opens the flow again and its reply is delivered
	writeDatagram(t, writer, "late", xnet.UDPDestination(xnet.ParseAddress(server.IP.String()), xnet.Port(server.Port)))

	if _, err := client.WriteTo([]byte("ping again"), server); err != nil {
		t.Fatalf("send the next datagram: %v", err)
	}
	readDatagrams(t, reader, 1)
	waitForResponderCount(t, underTest, 1)

	writeDatagram(t, writer, "answered", xnet.UDPDestination(xnet.ParseAddress(server.IP.String()), xnet.Port(server.Port)))
	if sources := peerReplies(t, client, 1); sources["answered"] != server.String() {
		t.Fatalf("the reply to the reopened flow came from %s, want %s", sources["answered"], server.String())
	}
}

func TestMipstackBoundsIdleUDPResponders(t *testing.T) {
	cases := []struct {
		name         string
		idleTimeout  time.Duration
		wantIdle     time.Duration
		wantInterval time.Duration
	}{
		{"the idle policy of the user level", 2 * time.Second, 2 * time.Second, time.Second},
		{"a disabled idle policy", 0, mipstackFallbackResponderIdle, mipstackMaxResponderSweepInterval},
		{"a short idle policy", 4 * time.Millisecond, 4 * time.Millisecond, mipstackMinResponderSweepInterval},
		{"a long idle policy", time.Hour, time.Hour, mipstackMaxResponderSweepInterval},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			stack := &stackMipstack{idleTimeout: testCase.idleTimeout}
			if got := stack.responderIdleTimeout(); got != testCase.wantIdle {
				t.Fatalf("idle bound = %s, want %s", got, testCase.wantIdle)
			}
			if got := stack.responderSweepInterval(); got != testCase.wantInterval {
				t.Fatalf("sweep interval = %s, want %s", got, testCase.wantInterval)
			}
		})
	}
}
