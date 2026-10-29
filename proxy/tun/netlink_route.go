//go:build linux

package tun

import (
	"net"
	"net/netip"

	"github.com/vishvananda/netlink"
	"github.com/xtls/xray-core/common/errors"
	"golang.org/x/sys/unix"
)

// The helpers here are pure, so they live outside the android build tag and are
// covered by netlink_route_test.go. Android implies the linux tag.

// androidRoutes returns routes on a custom routing table.
// Gateway addresses are not used on Android (point-to-point TUN).
func androidRoutes(cidrs []string, table, linkIndex int) ([]netlink.Route, error) {
	var routes []netlink.Route
	for _, cidr := range cidrs {
		prefix, err := netip.ParsePrefix(cidr)
		if err != nil {
			return nil, errors.New("invalid route ", cidr).Base(err)
		}
		prefix = prefix.Masked()
		_, ipNet, err := net.ParseCIDR(prefix.String())
		if err != nil {
			return nil, errors.New("invalid CIDR ", prefix.String()).Base(err)
		}
		routes = append(routes, netlink.Route{
			Dst:       ipNet,
			LinkIndex: linkIndex,
			Table:     table,
		})
	}
	return routes, nil
}

// androidRuleFamilies returns the IP families that need ip-rule redirects for
// the configured autoSystemRoutingTable CIDRs. Empty input yields no
// families: with no routes in the custom table, adding rules would only
// pollute the ip-rule space. Invalid CIDRs are skipped — setRoutes reports
// them before rules ever matter.
func androidRuleFamilies(cidrs []string) []int {
	var fams []int
	seen := map[int]bool{}
	for _, cidr := range cidrs {
		prefix, err := netip.ParsePrefix(cidr)
		if err != nil {
			continue
		}
		f := unix.AF_INET
		if prefix.Addr().Is6() {
			f = unix.AF_INET6
		}
		if !seen[f] {
			seen[f] = true
			fams = append(fams, f)
		}
	}
	return fams
}

// androidDefaultInterface selects the interface carrying the default route with
// the lowest metric, skipping the TUN interface, loopback, and down interfaces.
func androidDefaultInterface(routes []netlink.Route, tunIndex int) *net.Interface {
	var selected *net.Interface
	selectedMetric := -1
	for _, route := range routes {
		if route.Dst != nil {
			if ones, _ := route.Dst.Mask.Size(); ones != 0 {
				continue
			}
		}
		if route.LinkIndex == 0 {
			continue
		}
		iface, err := net.InterfaceByIndex(route.LinkIndex)
		if err != nil || !usableOutboundInterface(iface, tunIndex) {
			continue
		}
		if selected == nil || int(route.Priority) < selectedMetric {
			selected = iface
			selectedMetric = int(route.Priority)
		}
	}
	return selected
}

// usableOutboundInterface reports whether iface can carry outbound traffic.
func usableOutboundInterface(iface *net.Interface, tunIndex int) bool {
	return iface != nil &&
		iface.Index != tunIndex &&
		iface.Flags&net.FlagLoopback == 0 &&
		iface.Flags&net.FlagUp != 0
}
