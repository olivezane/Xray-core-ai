//go:build linux

package tun

import (
	"github.com/xtls/xray-core/common/errors"
	"gvisor.dev/gvisor/pkg/tcpip/link/fdbased"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

// fdEndpointDevice is implemented by devices whose packets flow through a
// plain file descriptor (Linux/Android). The stack builds an fdbased
// endpoint for them.
type fdEndpointDevice interface {
	FDs() []int
}

// newEndpoint builds the gVisor LinkEndpoint for the device. Endpoint
// construction lives here, in the stack module: Linux/Android devices expose
// a file descriptor and get an fdbased endpoint; any other device (fakes in
// tests) is wrapped via GVisorDevice.
func newEndpoint(device Tun, mtu uint32) (stack.LinkEndpoint, error) {
	if fdDevice, ok := device.(fdEndpointDevice); ok {
		return fdbased.New(&fdbased.Options{
			FDs:               fdDevice.FDs(),
			MTU:               mtu,
			RXChecksumOffload: true,
		})
	}
	if gvDevice, ok := device.(GVisorDevice); ok {
		return &LinkEndpoint{deviceMTU: mtu, device: gvDevice}, nil
	}
	return nil, errors.New("tun device supports no endpoint construction path")
}
