package tun

import (
	"context"
	"fmt"
	"io"
	stdnet "net"
	"net/netip"
	"testing"
	"time"

	"gvisor.dev/gvisor/pkg/tcpip/header"
)

// TestMipstackInputFeedsTheStack drives Input alone: nothing is injected into
// the device, so the answer can only be built from the packet Input handed the
// stack, the way a caller that reads the device itself hands it one.
func TestMipstackInputFeedsTheStack(t *testing.T) {
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
	defer device.close()
	defer stack.Close()

	client := netip.MustParseAddrPort("10.0.0.2:40000")
	server := netip.MustParseAddrPort("192.0.2.10:80")
	stack.(*stackMipstack).Input(buildTCPSegment(t, client, server, header.TCPFlagSyn, 1, 0))

	parseSynAck(t, readWrittenPacket(t, device, waitForPacketUntil), client, server)
}

// TestMipstackDialTCPReachesThePeer dials out of the stack at a peer serving a
// port, and requires the connection to carry both directions: the stack writes
// the handshake itself, so only a served connection proves the dial went out.
func TestMipstackDialTCPReachesThePeer(t *testing.T) {
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

	peer := newBridgeMipstackPeer(t, device, netip.MustParsePrefix("10.0.0.1/24"))
	defer peer.Close()
	defer device.close()
	defer stack.Close()

	address := netip.MustParseAddrPort("10.0.0.1:8000")
	listener, err := peer.stack.ListenTCP(ctx, "tcp", address)
	if err != nil {
		t.Fatalf("the peer cannot listen on %v: %v", address, err)
	}
	defer listener.Close()

	served := make(chan error, 1)
	go func() {
		connection, err := listener.Accept()
		if err != nil {
			served <- err
			return
		}
		defer connection.Close()

		request := make([]byte, 4)
		if _, err := io.ReadFull(connection, request); err != nil {
			served <- err
			return
		}
		if string(request) != "ping" {
			served <- fmt.Errorf("the peer received %q, want %q", request, "ping")
			return
		}

		_, err = connection.Write([]byte("pong"))
		served <- err
	}()

	connection, err := stack.(*stackMipstack).DialTCP(ctx, address)
	if err != nil {
		t.Fatalf("dial the peer at %v: %v", address, err)
	}
	defer connection.Close()

	if _, err := connection.Write([]byte("ping")); err != nil {
		t.Fatalf("write to the peer: %v", err)
	}
	if err := connection.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	response := make([]byte, 4)
	if _, err := io.ReadFull(connection, response); err != nil {
		t.Fatalf("read the reply: %v", err)
	}
	if string(response) != "pong" {
		t.Fatalf("reply = %q, want %q", response, "pong")
	}

	if err := <-served; err != nil {
		t.Fatalf("the peer did not serve the connection: %v", err)
	}
}

// TestMipstackDialUDPCarriesDatagrams requires the port to be usable in both
// directions against a peer, which is what a caller of the method needs.
func TestMipstackDialUDPCarriesDatagrams(t *testing.T) {
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

	peer := newBridgeMipstackPeer(t, device, netip.MustParsePrefix("10.0.0.1/24"))
	defer peer.Close()
	defer device.close()
	defer stack.Close()

	server, err := peer.stack.ListenUDP(ctx, "udp", netip.AddrPortFrom(netip.MustParseAddr("10.0.0.1"), 0))
	if err != nil {
		t.Fatalf("the peer cannot listen: %v", err)
	}
	defer server.Close()
	remote, err := netip.ParseAddrPort(server.LocalAddr().String())
	if err != nil {
		t.Fatalf("parse the peer address %s: %v", server.LocalAddr(), err)
	}

	connection, err := stack.(*stackMipstack).DialUDP()
	if err != nil {
		t.Fatalf("open the stack's udp port: %v", err)
	}
	defer connection.Close()

	// the stack holds no Local Address of its own, so the port is opened on
	// the address family the gateway prefixes of the device give it
	local, err := netip.ParseAddrPort(connection.LocalAddr().String())
	if err != nil {
		t.Fatalf("parse the local address %s: %v", connection.LocalAddr(), err)
	}
	if !local.Addr().IsUnspecified() || local.Port() == 0 {
		t.Fatalf("the stack's udp port is %s, want an allocated port of the device's family", connection.LocalAddr())
	}

	if _, err := connection.WriteTo([]byte("query"), stdnet.UDPAddrFromAddrPort(remote)); err != nil {
		t.Fatalf("send the query: %v", err)
	}

	if err := server.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatalf("set the peer read deadline: %v", err)
	}
	received := make([]byte, 64)
	n, from, err := server.ReadFrom(received)
	if err != nil {
		t.Fatalf("the peer received nothing: %v", err)
	}
	if string(received[:n]) != "query" {
		t.Fatalf("the peer received %q, want %q", received[:n], "query")
	}

	if _, err := server.WriteTo([]byte("answer"), from); err != nil {
		t.Fatalf("answer the query: %v", err)
	}

	if err := connection.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatalf("set the stack read deadline: %v", err)
	}
	n, source, err := connection.ReadFrom(received)
	if err != nil {
		t.Fatalf("the stack's udp port received nothing: %v", err)
	}
	if string(received[:n]) != "answer" {
		t.Fatalf("the stack's udp port received %q, want %q", received[:n], "answer")
	}
	if source.String() != remote.String() {
		t.Fatalf("the answer came from %s, want %v", source, remote)
	}
}

// TestMipstackDialUDPWithoutADeviceAddress reports the limit of the method: a
// device that configures no gateway gives the stack no address to open a port
// on, and the caller is told instead of being handed a port that cannot send.
func TestMipstackDialUDPWithoutADeviceAddress(t *testing.T) {
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
	defer device.close()
	defer stack.Close()

	if connection, err := stack.(*stackMipstack).DialUDP(); err == nil {
		connection.Close()
		t.Fatalf("the stack opened %s on a device with no address of its own", connection.LocalAddr())
	}
}
