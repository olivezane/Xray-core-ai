package tun

import (
	"errors"
	"net"
	"net/netip"

	"github.com/vishvananda/netlink"
)

// ErrNoOutboundInterface is the not-found shape of the RouteTable contract:
// no usable physical outbound interface is available right now.
var ErrNoOutboundInterface = errors.New("no usable outbound interface found")

// RouteTable resolves the physical interface outbound traffic must bind to.
// This is the single seam for physical-interface lookup on every platform;
// per-OS implementations live in route_table_<os>.go.
type RouteTable interface {
	// OutboundInterface returns the physical interface to bind outbound
	// traffic to. It returns ErrNoOutboundInterface (the single not-found
	// shape) when no interface is available, and other errors only on real
	// lookup failures.
	OutboundInterface(tunIndex int, fixedName string) (*net.Interface, error)
}

// usablePhysicalInterface reports whether iface can carry outbound traffic:
// not the TUN interface, not loopback, must be up. The guard lives here once;
// every RouteTable implementation resolves candidates through it.
func usablePhysicalInterface(iface *net.Interface, tunIndex int) bool {
	return iface != nil &&
		iface.Index != tunIndex &&
		iface.Flags&net.FlagLoopback == 0 &&
		iface.Flags&net.FlagUp != 0
}

// byFixedName resolves a fixed-name outbound interface, rejecting the TUN
// interface itself.
func byFixedName(name string, tunIndex int) (*net.Interface, error) {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return nil, ErrNoOutboundInterface
	}
	if iface.Index == tunIndex {
		return nil, ErrNoOutboundInterface
	}
	return iface, nil
}

// netInterfaceByIndex resolves an interface by index, or nil.
func netInterfaceByIndex(index int) *net.Interface {
	iface, err := net.InterfaceByIndex(index)
	if err != nil {
		return nil
	}
	return iface
}

// routeEntry is one default-route candidate.
type routeEntry struct {
	linkIndex int
	priority  int
}

// defaultRoutes converts default-route entries with a nonzero link index from
// a netlink route list into candidates.
func defaultRoutes(routes []netlink.Route) []routeEntry {
	var candidates []routeEntry
	for _, r := range routes {
		if r.LinkIndex == 0 || !isDefaultRoute(r.Dst) {
			continue
		}
		candidates = append(candidates, routeEntry{linkIndex: r.LinkIndex, priority: int(r.Priority)})
	}
	return candidates
}

// defaultInterfaceFromRoutes selects the physical interface carrying a
// default route: usable candidates only, lowest priority wins, first-in-list
// breaks ties.
func defaultInterfaceFromRoutes(routes []routeEntry, tunIndex int, interfaceByIndex func(int) *net.Interface) *net.Interface {
	var selected *net.Interface
	selectedPriority := -1
	for _, r := range routes {
		iface := interfaceByIndex(r.linkIndex)
		if !usablePhysicalInterface(iface, tunIndex) {
			continue
		}
		if selected == nil || r.priority < selectedPriority {
			selected = iface
			selectedPriority = r.priority
		}
	}
	return selected
}

// isDefaultRoute reports whether the route destination is the default route
// (nil destination or a zero-length prefix).
func isDefaultRoute(dst *net.IPNet) bool {
	if dst == nil {
		return true
	}
	ones, _ := dst.Mask.Size()
	return ones == 0
}

// Androoid ip-rule family constants (AF_INET / AF_INET6 values on Linux and
// Android). Defined locally so the rule-family logic stays testable
// cross-platform; only setRules (android build) consumes them.
const (
	androidFamilyIPv4 = 2
	androidFamilyIPv6 = 10
)

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
		f := androidFamilyIPv4
		if prefix.Addr().Is6() {
			f = androidFamilyIPv6
		}
		if !seen[f] {
			seen[f] = true
			fams = append(fams, f)
		}
	}
	return fams
}
