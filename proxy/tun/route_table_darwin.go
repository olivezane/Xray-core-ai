//go:build darwin

package tun

import (
	"net"
	"net/netip"

	"golang.org/x/net/route"
	"golang.org/x/sys/unix"
)

// routeTableDarwin resolves the physical outbound interface from the kernel
// routing information base (raw AF_ROUTE): the interface of the first
// default-route gateway.
type routeTableDarwin struct{}

func newRouteTable() RouteTable { return routeTableDarwin{} }

func (routeTableDarwin) OutboundInterface(tunIndex int, fixedName string) (*net.Interface, error) {
	if fixedName != "" {
		return byFixedName(fixedName, tunIndex)
	}

	rib, err := route.FetchRIB(unix.AF_UNSPEC, route.RIBTypeRoute, 0)
	if err != nil {
		return nil, err
	}
	messages, err := route.ParseRIB(route.RIBTypeRoute, rib)
	if err != nil {
		return nil, err
	}

	var ipv6Index int
	for _, message := range messages {
		routeMessage, ok := message.(*route.RouteMessage)
		if !ok || routeMessage.Index == tunIndex {
			continue
		}
		if routeMessage.Flags&unix.RTF_UP == 0 || routeMessage.Flags&unix.RTF_GATEWAY == 0 {
			continue
		}

		family, ok := defaultRouteFamily(routeMessage)
		if !ok {
			continue
		}
		if family == unix.AF_INET {
			return usableDarwinInterface(routeMessage.Index, tunIndex)
		}
		if family == unix.AF_INET6 && ipv6Index == 0 {
			ipv6Index = routeMessage.Index
		}
	}

	if ipv6Index != 0 {
		return usableDarwinInterface(ipv6Index, tunIndex)
	}
	return nil, ErrNoOutboundInterface
}

func defaultRouteFamily(message *route.RouteMessage) (int, bool) {
	if len(message.Addrs) <= unix.RTAX_NETMASK {
		return 0, false
	}

	switch destination := message.Addrs[unix.RTAX_DST].(type) {
	case *route.Inet4Addr:
		mask, ok := message.Addrs[unix.RTAX_NETMASK].(*route.Inet4Addr)
		if !ok || destination.IP != netip.IPv4Unspecified().As4() {
			return 0, false
		}
		ones, bits := net.IPMask(mask.IP[:]).Size()
		return unix.AF_INET, ones == 0 && bits == 32
	case *route.Inet6Addr:
		mask, ok := message.Addrs[unix.RTAX_NETMASK].(*route.Inet6Addr)
		if !ok || destination.IP != netip.IPv6Unspecified().As16() {
			return 0, false
		}
		ones, bits := net.IPMask(mask.IP[:]).Size()
		return unix.AF_INET6, ones == 0 && bits == 128
	default:
		return 0, false
	}
}

func usableDarwinInterface(index int, tunIndex int) (*net.Interface, error) {
	iface, err := net.InterfaceByIndex(index)
	if err != nil {
		return nil, err
	}
	if !usablePhysicalInterface(iface, tunIndex) {
		return nil, ErrNoOutboundInterface
	}
	return iface, nil
}
