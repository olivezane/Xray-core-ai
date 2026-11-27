package tagged_test

import (
	"context"
	"net"
	"strings"
	"testing"

	xnet "github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/features/routing"
	"github.com/xtls/xray-core/transport/internet/tagged"
)

// TestDialWithoutDialer checks that using the tagged dialer before it is
// installed says so, instead of panicking on a nil function.
func TestDialWithoutDialer(t *testing.T) {
	if tagged.Ready() {
		t.Skip("a tagged dialer is already installed")
	}
	_, err := tagged.Dial(context.Background(), nil, xnet.TCPDestination(xnet.LocalHostIP, 1), "")
	if err == nil {
		t.Fatal("dialing without a tagged dialer succeeded")
	}
	if !strings.Contains(err.Error(), "main/distro/all") {
		t.Errorf("error does not say where the dialer is installed: %v", err)
	}
}

// TestDialUsesInstalledDialer checks that SetDialer is what Dial calls, so the
// wiring is visible at the call site instead of hidden in an init().
func TestDialUsesInstalledDialer(t *testing.T) {
	called := false
	tagged.SetDialer(func(ctx context.Context, dispatcher routing.Dispatcher, dest xnet.Destination, tag string) (net.Conn, error) {
		called = true
		if tag != "proxy" || dest.Address.String() != xnet.LocalHostIP.String() {
			t.Errorf("dialer got dest %v, tag %q", dest, tag)
		}
		return nil, nil
	})
	if !tagged.Ready() {
		t.Fatal("dialer was not installed")
	}
	if _, err := tagged.Dial(context.Background(), nil, xnet.TCPDestination(xnet.LocalHostIP, 1), "proxy"); err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	if !called {
		t.Fatal("installed dialer was not called")
	}
}
