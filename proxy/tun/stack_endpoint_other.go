//go:build !linux

package tun

import (
	"github.com/xtls/xray-core/common/errors"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

// newEndpoint builds the gVisor LinkEndpoint for the device. Endpoint
// construction lives here, in the stack module: Windows/Darwin/FreeBSD
// devices implement GVisorDevice and get wrapped by the in-process
// LinkEndpoint.
func newEndpoint(device Tun, mtu uint32) (stack.LinkEndpoint, error) {
	gvDevice, ok := device.(GVisorDevice)
	if !ok {
		return nil, errors.New("tun device supports no endpoint construction path")
	}
	return &LinkEndpoint{deviceMTU: mtu, device: gvDevice}, nil
}
