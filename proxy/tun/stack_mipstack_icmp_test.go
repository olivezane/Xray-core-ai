package tun

import (
	"context"
	"errors"
	stdnet "net"
	"net/netip"
	"testing"
	"time"

	"github.com/metacubex/mipstack"
)

// icmpPeer is a client of the stack under test talking raw ICMP
type icmpPeer struct {
	connection stdnet.Conn
	client     netip.Addr
	remote     netip.Addr
}

func dialICMPPeer(t *testing.T, ctx context.Context, peer *bridgeMipstackPeer, client, remote netip.Addr) *icmpPeer {
	t.Helper()

	network := "ip4:icmp"
	if client.Is6() {
		network = "ip6:ipv6-icmp"
	}
	connection, err := peer.stack.DialIP(ctx, network, client, remote)
	if err != nil {
		t.Fatalf("open the peer's icmp socket: %v", err)
	}

	return &icmpPeer{connection: connection, client: client, remote: remote}
}

func (p *icmpPeer) close() {
	_ = p.connection.Close()
}

// sendEchoRequest writes one echo request carrying the given payload
func (p *icmpPeer) sendEchoRequest(t *testing.T, identifier, sequence uint16, payload string) {
	t.Helper()

	request := mipstack.ICMPMessage{Source: p.client, Destination: p.remote}
	if err := request.SetEchoRequest(identifier, sequence, []byte(payload)); err != nil {
		t.Fatalf("build the echo request: %v", err)
	}
	wire, err := request.MarshalBinary()
	if err != nil {
		t.Fatalf("encode the echo request: %v", err)
	}
	if _, err := p.connection.Write(wire); err != nil {
		t.Fatalf("send the echo request: %v", err)
	}
}

// sendEchoReply writes an echo reply, which is no echo request and must not be
// answered
func (p *icmpPeer) sendEchoReply(t *testing.T, identifier, sequence uint16) {
	t.Helper()

	reply := mipstack.ICMPMessage{Source: p.client, Destination: p.remote}
	if err := reply.SetEchoReply(identifier, sequence, []byte("no request")); err != nil {
		t.Fatalf("build the echo reply: %v", err)
	}
	wire, err := reply.MarshalBinary()
	if err != nil {
		t.Fatalf("encode the echo reply: %v", err)
	}
	if _, err := p.connection.Write(wire); err != nil {
		t.Fatalf("send the echo reply: %v", err)
	}
}

// expectEchoReply reads one message and reports whether it is the expected
// echo reply
func (p *icmpPeer) expectEchoReply(t *testing.T, identifier, sequence uint16, payload string) {
	t.Helper()

	if err := p.connection.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatalf("set the icmp read deadline: %v", err)
	}
	received := make([]byte, 64)
	n, err := p.connection.Read(received)
	if err != nil {
		t.Fatalf("the peer received no reply to its echo request: %v", err)
	}

	// decoding validates the checksum of the message the stack wrote
	protocol := int(mipstack.ProtocolICMPv4)
	if p.client.Is6() {
		protocol = mipstack.ProtocolICMPv6
	}
	message, err := mipstack.IPPacket{
		Source:      p.remote,
		Destination: p.client,
		Protocol:    protocol,
		Payload:     received[:n],
	}.ICMPMessage()
	if err != nil {
		t.Fatalf("the peer received an invalid icmp message: %v", err)
	}
	if !message.IsEchoReply() {
		t.Fatalf("the peer received icmp type %d code %d, want an echo reply", message.Type, message.Code)
	}

	gotIdentifier, gotSequence, gotPayload, ok := message.Echo()
	if !ok {
		t.Fatal("the echo reply carries no echo body")
	}
	if gotIdentifier != identifier || gotSequence != sequence || string(gotPayload) != payload {
		t.Fatalf("the echo reply is id=%d seq=%d payload=%q, want id=%d seq=%d payload=%q",
			gotIdentifier, gotSequence, gotPayload, identifier, sequence, payload)
	}
}

// expectNoReply reports whether the peer receives nothing before the deadline
func (p *icmpPeer) expectNoReply(t *testing.T, within time.Duration) {
	t.Helper()

	if err := p.connection.SetReadDeadline(time.Now().Add(within)); err != nil {
		t.Fatalf("set the icmp read deadline: %v", err)
	}
	received := make([]byte, 64)
	n, err := p.connection.Read(received)
	if err == nil {
		t.Fatalf("the peer received %d bytes, want no reply", n)
	}
	var timeout stdnet.Error
	if !errors.As(err, &timeout) || !timeout.Timeout() {
		t.Fatalf("read from the icmp socket: %v, want a timeout", err)
	}
}

func TestMipstackAnswersICMPEchoRequestsOnly(t *testing.T) {
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

	peer := newBridgeMipstackPeer(t, device, netip.MustParsePrefix("10.0.0.2/24"), netip.MustParsePrefix("fdfe:dcba:9876::2/64"))
	defer peer.Close()
	defer device.close()
	defer stack.Close()

	client := dialICMPPeer(t, ctx, peer, netip.MustParseAddr("10.0.0.2"), netip.MustParseAddr("192.0.2.10"))
	defer client.close()

	client.sendEchoRequest(t, 0x1234, 7, "ping")
	client.expectEchoReply(t, 0x1234, 7, "ping")

	// an echo reply is no echo request: it is consumed without an answer,
	// while the stack keeps answering what it should
	client.sendEchoReply(t, 0x1234, 8)
	client.expectNoReply(t, 300*time.Millisecond)

	client.sendEchoRequest(t, 0x1234, 9, "still here")
	client.expectEchoReply(t, 0x1234, 9, "still here")

	// the other address family is answered too
	client6 := dialICMPPeer(t, ctx, peer, netip.MustParseAddr("fdfe:dcba:9876::2"), netip.MustParseAddr("2001:db8::10"))
	defer client6.close()

	client6.sendEchoRequest(t, 0x5678, 1, "ping6")
	client6.expectEchoReply(t, 0x5678, 1, "ping6")
}
