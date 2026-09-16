package tun

import (
	"context"
	"errors"
	"io"
	stdnet "net"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/metacubex/mipstack"
	xnet "github.com/xtls/xray-core/common/net"
)

// bridgeDevice is a Packet Device a test drives directly: what the test
// injects is what the stack under test reads, and what that stack writes is
// what the test takes. It is also the Tun the stack is built for.
type bridgeDevice struct {
	fakeTun

	injected chan []byte
	written  chan []byte
	closed   chan struct{}
	once     sync.Once
}

func newBridgeDevice() *bridgeDevice {
	return &bridgeDevice{
		injected: make(chan []byte, 64),
		written:  make(chan []byte, 64),
		closed:   make(chan struct{}),
	}
}

func (d *bridgeDevice) ReadPackets(packets [][]byte) (int, error) {
	select {
	case packet := <-d.injected:
		copy(packets[0], packet)
		return 1, nil
	default:
		return 0, ErrQueueEmpty
	}
}

func (d *bridgeDevice) WritePackets(packets [][]byte) error {
	for _, packet := range packets {
		select {
		case d.written <- append([]byte(nil), packet...):
		case <-d.closed:
			return errors.New("bridge device is closed")
		}
	}
	return nil
}

func (d *bridgeDevice) Wait() {
	select {
	case <-time.After(time.Millisecond):
	case <-d.closed:
	}
}

func (d *bridgeDevice) Close() error {
	d.close()
	return nil
}

func (d *bridgeDevice) close() {
	d.once.Do(func() { close(d.closed) })
}

// inject hands one packet to the stack under test
func (d *bridgeDevice) inject(packet []byte) bool {
	select {
	case d.injected <- append([]byte(nil), packet...):
		return true
	case <-d.closed:
		return false
	}
}

// drain takes one packet the stack under test wrote
func (d *bridgeDevice) drain() ([]byte, bool) {
	select {
	case packet := <-d.written:
		return packet, true
	case <-d.closed:
		return nil, false
	}
}

// bridgeMipstackPeer is a MIPS Stack of the test's own, bridged to the device, so
// a test has a real peer to talk to the stack under test with.
type bridgeMipstackPeer struct {
	stack *mipstack.Stack
	pumps sync.WaitGroup
}

func newBridgeMipstackPeer(t *testing.T, device *bridgeDevice, local ...netip.Prefix) *bridgeMipstackPeer {
	t.Helper()

	stack, err := mipstack.New(mipstack.Config{
		LocalAddresses: local,
		MTU:            mipstackDefaultMTU,
	})
	if err != nil {
		t.Fatalf("create peer stack: %v", err)
	}
	if err := stack.Start(); err != nil {
		_ = stack.Close()
		t.Fatalf("start peer stack: %v", err)
	}

	peer := &bridgeMipstackPeer{stack: stack}
	peer.pumps.Add(2)

	go func() {
		defer peer.pumps.Done()
		packets := newPacketBatch(mipstackDefaultMTU)
		sizes := make([]int, len(packets))
		for {
			count, err := stack.Read(packets, sizes, 0)
			if err != nil {
				return
			}
			for i := 0; i < count; i++ {
				if !device.inject(packets[i][:sizes[i]]) {
					return
				}
			}
		}
	}()

	go func() {
		defer peer.pumps.Done()
		packets := newPacketBatch(mipstackDefaultMTU)
		for {
			packet, ok := device.drain()
			if !ok {
				return
			}
			copy(packets[0], packet)
			if _, err := stack.Write(packets[:1], 0); err != nil {
				return
			}
		}
	}()

	return peer
}

func (p *bridgeMipstackPeer) Close() error {
	err := p.stack.Close()
	p.pumps.Wait()
	return err
}

// acceptedConnection is one connection the stack under test handed to the
// inbound
type acceptedConnection struct {
	connection  stdnet.Conn
	destination xnet.Destination
}

type recordingConnectionHandler struct {
	connections chan acceptedConnection
}

func newRecordingConnectionHandler() *recordingConnectionHandler {
	return &recordingConnectionHandler{connections: make(chan acceptedConnection, 4)}
}

func (h *recordingConnectionHandler) HandleConnection(connection stdnet.Conn, destination xnet.Destination) {
	h.connections <- acceptedConnection{connection: connection, destination: destination}
}

func (h *recordingConnectionHandler) next(t *testing.T, timeout time.Duration) acceptedConnection {
	t.Helper()
	select {
	case accepted := <-h.connections:
		return accepted
	case <-time.After(timeout):
		t.Fatal("the inbound never received the intercepted connection")
		return acceptedConnection{}
	}
}

func TestMipstackCarriesTCPToTheConnectionHandler(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
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
	// LIFO: the stack under test stops first, then the device stops the peer's
	// pumps, and only then does the peer wait for them
	defer peer.Close()
	defer device.close()
	defer stack.Close()

	target := netip.MustParseAddrPort("192.0.2.10:443")
	connection, err := peer.stack.DialTCP(ctx, "tcp", netip.AddrPort{}, target)
	if err != nil {
		t.Fatalf("dial the intercepted destination: %v", err)
	}
	defer connection.Close()

	accepted := handler.next(t, 10*time.Second)
	if accepted.destination.Network != xnet.Network_TCP {
		t.Fatalf("intercepted network = %v, want tcp", accepted.destination.Network)
	}
	if accepted.destination.Address.String() != target.Addr().String() || accepted.destination.Port != xnet.Port(target.Port()) {
		t.Fatalf("intercepted destination = %v, want %v", accepted.destination.NetAddr(), target)
	}

	if _, err := connection.Write([]byte("ping")); err != nil {
		t.Fatalf("write to the intercepted destination: %v", err)
	}
	if err := accepted.connection.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	request := make([]byte, 4)
	if _, err := io.ReadFull(accepted.connection, request); err != nil {
		t.Fatalf("read the intercepted payload: %v", err)
	}
	if string(request) != "ping" {
		t.Fatalf("intercepted payload = %q, want %q", request, "ping")
	}

	if _, err := accepted.connection.Write([]byte("pong")); err != nil {
		t.Fatalf("reply to the intercepted connection: %v", err)
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
}
