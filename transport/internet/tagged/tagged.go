package tagged

import (
	"context"

	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/features/routing"
)

type DialFunc func(ctx context.Context, dispatcher routing.Dispatcher, dest net.Destination, tag string) (net.Conn, error)

// dialer is what Dial uses. It is installed by SetDialer, not by an init()
// function: the implementation needs core, core cannot import internet, and a
// package-level assignment would make the wiring depend on import order.
var dialer DialFunc

// SetDialer installs f as the tagged dialer of this process. The xray binary
// installs it in main/distro/all, which main wires before it starts.
func SetDialer(f DialFunc) {
	dialer = f
}

// Ready reports whether a tagged dialer is installed.
func Ready() bool {
	return dialer != nil
}

// Dial reaches dest through the outbound tag by dispatching, using the dialer
// installed by SetDialer.
func Dial(ctx context.Context, dispatcher routing.Dispatcher, dest net.Destination, tag string) (net.Conn, error) {
	if dialer == nil {
		return nil, errors.New("tagged dialer is not installed; the xray binary installs it in main/distro/all")
	}
	return dialer(ctx, dispatcher, dest, tag)
}
