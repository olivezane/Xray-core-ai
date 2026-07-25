//go:build linux && !android

package tun

import (
	"net"

	"github.com/vishvananda/netlink"
)

// routeTableLinux resolves the physical outbound interface from the main
// routing table: the default route with the lowest priority wins.
type routeTableLinux struct{}

func newRouteTable() RouteTable { return routeTableLinux{} }

func (routeTableLinux) OutboundInterface(tunIndex int, fixedName string) (*net.Interface, error) {
	if fixedName != "" {
		return byFixedName(fixedName, tunIndex)
	}

	for _, family := range []int{
		netlink.FAMILY_V4,
		netlink.FAMILY_V6,
	} {
		routes, err := netlink.RouteList(nil, family)
		if err != nil {
			continue
		}
		if iface := defaultInterfaceFromRoutes(defaultRoutes(routes), tunIndex, netInterfaceByIndex); iface != nil {
			return iface, nil
		}
	}

	return nil, ErrNoOutboundInterface
}
