//go:build !linux && !windows && !android && !darwin

package tun

import "net"

// routeTableFallback supports fixed-name outbound interfaces only; automatic
// selection is not available on these platforms.
type routeTableFallback struct{}

func newRouteTable() RouteTable { return routeTableFallback{} }

func (routeTableFallback) OutboundInterface(tunIndex int, fixedName string) (*net.Interface, error) {
	if fixedName == "" {
		return nil, ErrNoOutboundInterface
	}
	return byFixedName(fixedName, tunIndex)
}
