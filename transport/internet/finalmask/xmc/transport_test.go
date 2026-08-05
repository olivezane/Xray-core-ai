package xmc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	cnet "github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/serial"
	"github.com/xtls/xray-core/transport/internet"
	"github.com/xtls/xray-core/transport/internet/stat"
	_ "github.com/xtls/xray-core/transport/internet/tcp" // register the tcp transport dialer/listener
)

// TestXMCThroughTCPTransport drives XMC through its only real usage: the
// TcpmaskManager → tcp transport chain. The client dials through tcp.Dial
// (mask wraps the dialed conn) and the server accepts through tcp.ListenTCP
// (mask wraps the listener), so the Minecraft login handshake runs over a real
// transport, not a net.Pipe bypass.
func TestXMCThroughTCPTransport(t *testing.T) {
	password := "transport-shared-key"
	profiles := []loginProfile{testLoginProfile("transport_user")}
	privateKey, publicKey := deriveTestRSAKey(t, password)

	mss, err := internet.ToMemoryStreamConfig(&internet.StreamConfig{
		ProtocolName: "tcp",
		Tcpmasks: []*serial.TypedMessage{
			serial.ToTypedMessage(&Config{
				Password:      password,
				RsaPrivateKey: privateKey,
				RsaPublicKey:  publicKey,
				Hostname:      "localhost",
				Profiles: []*Profile{
					{
						Username:          profiles[0].Username,
						Uuid:              profiles[0].UUID[:],
						TexturesValue:     profiles[0].TexturesValue,
						TexturesSignature: profiles[0].TexturesSignature,
					},
				},
			}),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	serverDone := make(chan error, 1)
	ln, err := internet.ListenTCP(context.Background(), cnet.LocalHostIP, cnet.Port(freeTCPPort(t)), mss, func(conn stat.Connection) {
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
		request := make([]byte, len("hello transport"))
		if _, err := io.ReadFull(conn, request); err != nil {
			serverDone <- err
			return
		}
		if !bytes.Equal(request, []byte("hello transport")) {
			serverDone <- fmt.Errorf("unexpected payload: %q", request)
			return
		}
		_, err := conn.Write([]byte("hello xmc"))
		serverDone <- err
	})
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	conn, err := internet.Dial(context.Background(), cnet.TCPDestination(cnet.LocalHostIP, cnet.Port(ln.Addr().(*net.TCPAddr).Port)), mss)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))

	if _, err = conn.Write([]byte("hello transport")); err != nil {
		t.Fatalf("client write: %v", err)
	}
	response := make([]byte, len("hello xmc"))
	if _, err = io.ReadFull(conn, response); err != nil {
		t.Fatalf("client read: %v", err)
	}
	if !bytes.Equal(response, []byte("hello xmc")) {
		t.Fatalf("unexpected response: %q", response)
	}

	select {
	case err := <-serverDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("server side of the XMC handshake did not complete")
	}
}

func freeTCPPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}
