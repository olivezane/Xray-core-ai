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

// TestMipstackRepliesToTheGatewayAddressOnTheDevice locks in that the gateway
// address of the device is not one of the stack's own Local Addresses. mipstack
// sends output addressed to a Local Address to its loopback queue instead of
// the device, so a stack owning the gateway address swallows every reply the
// inbound sends back to the host, and the host never sees an answer.
func TestMipstackRepliesToTheGatewayAddressOnTheDevice(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	device := newBridgeDevice()

	handler := newRecordingConnectionHandler()
	stack, err := newMipstack(ctx, StackOptions{
		Tun:         device,
		MTU:         mipstackDefaultMTU,
		IdleTimeout: time.Minute,
		Gateway:     []string{"10.0.0.1/24"},
	}, handler)
	if err != nil {
		t.Fatalf("create the MIPS Stack: %v", err)
	}
	if err := stack.Start(); err != nil {
		t.Fatalf("start the MIPS Stack: %v", err)
	}

	// the host on the other side of the device owns the gateway address
	peer := newBridgeMipstackPeer(t, device, netip.MustParsePrefix("10.0.0.1/24"))

	// registered so that the device closes before the peer's pumps are waited
	// for, matching the other mipstack tests
	defer peer.Close()
	defer device.close()
	defer stack.Close()

	client, err := peer.stack.ListenUDP(ctx, "udp", netip.AddrPortFrom(netip.MustParseAddr("10.0.0.1"), 0))
	if err != nil {
		t.Fatalf("open the host's udp socket: %v", err)
	}
	defer client.Close()

	server := netip.MustParseAddrPort("192.0.2.10:53")
	if _, err := client.WriteTo([]byte("query"), stdnet.UDPAddrFromAddrPort(server)); err != nil {
		t.Fatalf("send the query: %v", err)
	}

	accepted := handler.next(t, 10*time.Second)
	reader, ok := accepted.connection.(buf.Reader)
	if !ok {
		t.Fatalf("the intercepted UDP session is no buffer reader: %T", accepted.connection)
	}
	writer, ok := accepted.connection.(buf.Writer)
	if !ok {
		t.Fatalf("the intercepted UDP session is no buffer writer: %T", accepted.connection)
	}

	if datagrams := readDatagrams(t, reader, 1); datagrams[0].payload != "query" {
		t.Fatalf("the inbound received %q, want the query", datagrams[0].payload)
	}
	writeDatagram(t, writer, "answer", xnet.UDPDestination(xnet.ParseAddress(server.Addr().String()), xnet.Port(server.Port())))

	sources := peerReplies(t, client, 1)
	if sources["answer"] != server.String() {
		t.Fatalf("the reply came from %s, want %v (the host never received it otherwise)", sources["answer"], server)
	}
}
