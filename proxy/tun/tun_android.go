//go:build android

package tun

import (
	"bufio"
	"context"
	"net"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/platform"
	"golang.org/x/sys/unix"
)

const (
	ipRoute2TableIndex = 2022
	ipRoute2RuleIndex  = 9000
)

// AndroidTun manages the TUN interface on Android.
// Two modes:
//   - FD mode: xray.tun.fd env set > 0, interface provided externally (V2RayNG etc.)
//   - Self-create mode: no FD, creates TUN via /dev/tun and configures addresses/routes/rules
type AndroidTun struct {
	*fdTun
	options *Config
	ownsTun bool // self-created vs FD-provided
	tunLink netlink.Link

	ifAddrs []netlink.Addr
	routes  []netlink.Route

	routeMonitorStop chan struct{}
	routeMonitorOnce sync.Once
}

var _ Tun = (*AndroidTun)(nil)

// checkBareTunFd refuses a TUN descriptor that puts a header in front of every
// packet, so that an application handing its device over learns at startup
// rather than from the packets that never arrive. Both stacks read the Android
// device as bare IP packets, and neither of them can skip a header the device
// does not announce to them.
func checkBareTunFd(fd int) error {
	ifr, err := unix.NewIfreq("")
	if err != nil {
		return errors.New("invalid ", platform.TunFdKey).Base(err)
	}
	if err = unix.IoctlIfreq(fd, unix.TUNGETIFF, ifr); err != nil {
		return errors.New("invalid ", platform.TunFdKey, ": not a TUN file descriptor").Base(err)
	}
	flags := ifr.Uint16()
	if flags&unix.IFF_NO_PI == 0 || flags&unix.IFF_VNET_HDR != 0 {
		return errors.New("invalid ", platform.TunFdKey, ": TUN device must use IFF_NO_PI and no IFF_VNET_HDR")
	}
	return nil
}

// NewTun builds a new AndroidTun.
// If xray.tun.fd > 0 is provided, it wraps that FD directly (legacy/App mode).
// Otherwise it creates a TUN device via the kernel interface.
func NewTun(options *Config) (Tun, error) {
	fdStr := platform.NewEnvFlag(platform.TunFdKey).GetValue(func() string { return "" })

	if fdStr != "" {
		fd, err := strconv.Atoi(fdStr)
		if err == nil && fd > 0 {
			if err = checkBareTunFd(fd); err != nil {
				return nil, err
			}
			if err = unix.SetNonblock(fd, true); err != nil {
				return nil, err
			}
			// the descriptor stays with the application, Close ends the reading of a duplicate
			fd, err = unix.FcntlInt(uintptr(fd), unix.F_DUPFD_CLOEXEC, 0)
			if err != nil {
				return nil, err
			}
			return &AndroidTun{fdTun: newFdTun(fd, 0), options: options}, nil
		}
	}

	// No external FD — create TUN ourselves
	tunFd, tunLink, err := openTun(options.Name)
	if err != nil {
		return nil, err
	}

	return &AndroidTun{
		fdTun:   newFdTun(tunFd, 0),
		options: options,
		ownsTun: true,
		tunLink: tunLink,
	}, nil
}

// openTun opens or creates the TUN device.
// Android may have /dev/tun or /dev/net/tun depending on kernel config.
func openTun(name string) (int, netlink.Link, error) {
	ctrlPath := "/dev/tun"
	if _, err := os.Stat(ctrlPath); os.IsNotExist(err) {
		ctrlPath = "/dev/net/tun"
	}

	fd, err := unix.Open(ctrlPath, unix.O_RDWR, 0)
	if err != nil {
		return -1, nil, errors.New("failed to open TUN device: ", ctrlPath).Base(err)
	}

	ifr, err := unix.NewIfreq(name)
	if err != nil {
		unix.Close(fd)
		return -1, nil, err
	}

	ifr.SetUint16(uint16(unix.IFF_TUN | unix.IFF_NO_PI))
	if err = unix.IoctlIfreq(fd, unix.TUNSETIFF, ifr); err != nil {
		unix.Close(fd)
		return -1, nil, err
	}

	actualName := ifr.Name()
	if err = unix.SetNonblock(fd, true); err != nil {
		unix.Close(fd)
		return -1, nil, err
	}

	tunLink, err := netlink.LinkByName(actualName)
	if err != nil {
		unix.Close(fd)
		return -1, nil, err
	}

	return fd, tunLink, nil
}

// Start sets up TUN: MTU, IP addresses, link up, routes, ip rules.
// In FD mode the external App is expected to handle setup.
func (t *AndroidTun) Start() error {
	// registered before the ownsTun check so FD mode keeps refreshing too
	if updater != nil {
		t.routeMonitorStop = make(chan struct{})
		go t.monitorRouteChanges()
	}

	if !t.ownsTun {
		return nil
	}

	if err := netlink.LinkSetMTU(t.tunLink, int(t.options.MTU)); err != nil {
		return errors.New("failed to set MTU").Base(err)
	}
	if err := t.setInterfaceAddresses(); err != nil {
		return err
	}
	if err := netlink.LinkSetUp(t.tunLink); err != nil {
		t.unsetInterfaceAddresses()
		return err
	}
	if err := t.setRoutes(); err != nil {
		t.unsetInterfaceAddresses()
		netlink.LinkSetDown(t.tunLink)
		return err
	}
	if err := t.setRules(); err != nil {
		t.unsetRoutes()
		t.unsetInterfaceAddresses()
		netlink.LinkSetDown(t.tunLink)
		return err
	}

	return nil
}

// Close tears down TUN: rules, routes, addresses, link down, fd.
func (t *AndroidTun) Close() error {
	t.routeMonitorOnce.Do(func() {
		if t.routeMonitorStop != nil {
			close(t.routeMonitorStop)
		}
	})
	if t.ownsTun {
		t.unsetRules()
		t.unsetRoutes()
		t.unsetInterfaceAddresses()
		netlink.LinkSetDown(t.tunLink)
	}
	return t.closeFile()
}

// Name returns the TUN interface name.
func (t *AndroidTun) Name() (string, error) {
	if t.tunLink != nil {
		return t.tunLink.Attrs().Name, nil
	}
	ifr, err := unix.NewIfreq("")
	if err != nil {
		return "", err
	}
	if err = unix.IoctlIfreq(t.fd, unix.TUNGETIFF, ifr); err != nil {
		return "", err
	}
	return ifr.Name(), nil
}

// Index returns the TUN interface system index.
func (t *AndroidTun) Index() (int, error) {
	if t.tunLink != nil {
		return t.tunLink.Attrs().Index, nil
	}
	name, err := t.Name()
	if err != nil {
		return 0, err
	}
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return 0, err
	}
	return iface.Index, nil
}

// ---------------------------------------------------------------------------
// Address management
// ---------------------------------------------------------------------------

func (t *AndroidTun) setInterfaceAddresses() error {
	for _, addrStr := range t.options.Gateway {
		addr, err := netlink.ParseAddr(addrStr)
		if err != nil {
			t.unsetInterfaceAddresses()
			return errors.New("invalid interface address ", addrStr).Base(err)
		}
		if err := netlink.AddrReplace(t.tunLink, addr); err != nil {
			t.unsetInterfaceAddresses()
			return errors.New("failed to add address ", addrStr).Base(err)
		}
		t.ifAddrs = append(t.ifAddrs, *addr)
	}
	return nil
}

func (t *AndroidTun) unsetInterfaceAddresses() error {
	var errs []error
	for i := len(t.ifAddrs) - 1; i >= 0; i-- {
		if err := netlink.AddrDel(t.tunLink, &t.ifAddrs[i]); err != nil {
			errs = append(errs, err)
		}
	}
	t.ifAddrs = nil
	return errors.Combine(errs...)
}

// ---------------------------------------------------------------------------
// Route management — uses a custom routing table, not RT_TABLE_MAIN
// ---------------------------------------------------------------------------

func (t *AndroidTun) setRoutes() error {
	routes, err := androidRoutes(t.options.AutoSystemRoutingTable, ipRoute2TableIndex, t.tunLink.Attrs().Index)
	if err != nil {
		return err
	}
	for _, route := range routes {
		if err := netlink.RouteAdd(&route); err != nil {
			t.unsetRoutes()
			return errors.New("failed to add route").Base(err)
		}
		t.routes = append(t.routes, route)
	}
	return nil
}

func (t *AndroidTun) unsetRoutes() error {
	var errs []error
	for i := len(t.routes) - 1; i >= 0; i-- {
		if err := netlink.RouteDel(&t.routes[i]); err != nil {
			errs = append(errs, err)
		}
	}
	t.routes = nil
	return errors.Combine(errs...)
}

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

// ---------------------------------------------------------------------------
// IP rule management — redirect traffic to the custom routing table
// ---------------------------------------------------------------------------

func (t *AndroidTun) setRules() error {
	// Only install redirect rules for families that actually have routes in
	// the custom table; with no autoSystemRoutingTable the rules would only
	// pollute the ip-rule space and can interfere with other VPNs.
	families := androidRuleFamilies(t.options.AutoSystemRoutingTable)
	for _, family := range families {
		r := netlink.NewRule()
		r.Priority = ipRoute2RuleIndex
		r.Family = family
		r.Table = ipRoute2TableIndex
		if err := netlink.RuleAdd(r); err != nil {
			// the rule is already there, either ours from a previous run or another app's
			if errors.Cause(err) == unix.EEXIST {
				continue
			}
			t.unsetRules()
			return errors.New("failed to add rule").Base(err)
		}
	}
	return nil
}

func (t *AndroidTun) unsetRules() error {
	list, err := netlink.RuleList(netlink.FAMILY_ALL)
	if err != nil {
		return err
	}
	var errs []error
	for _, rule := range list {
		// only touch our own rule: RuleDel matches on family and priority alone,
		// so without the table check it would delete any other app's pref-9000 rule
		if rule.Priority != ipRoute2RuleIndex || rule.Table != ipRoute2TableIndex {
			continue
		}
		del := netlink.NewRule()
		del.Family = rule.Family
		del.Priority = rule.Priority
		del.Table = rule.Table
		if err := netlink.RuleDel(del); err != nil {
			errs = append(errs, errors.New("failed to delete rule priority ", rule.Priority).Base(err))
		}
	}
	return errors.Combine(errs...)
}

// ---------------------------------------------------------------------------
// Route monitoring — re-find outbound interface on network changes
// ---------------------------------------------------------------------------

func (t *AndroidTun) monitorRouteChanges() {
	routeCh := make(chan netlink.RouteUpdate, 1)
	if err := netlink.RouteSubscribe(routeCh, t.routeMonitorStop); err != nil {
		errors.LogInfo(context.Background(), "[tun] android: failed to subscribe route changes: ", err)
		t.pollRouteChanges()
		return
	}

	linkCh := make(chan netlink.LinkUpdate, 1)
	if err := netlink.LinkSubscribe(linkCh, t.routeMonitorStop); err != nil {
		errors.LogInfo(context.Background(), "[tun] android: failed to subscribe link changes: ", err)
		t.pollRouteChanges()
		return
	}

	for {
		select {
		case _, ok := <-routeCh:
			if !ok {
				return
			}
			if updater != nil {
				updater.Update()
			}
		case _, ok := <-linkCh:
			if !ok {
				return
			}
			if updater != nil {
				updater.Update()
			}
		case <-t.routeMonitorStop:
			return
		}
	}
}

// pollRouteChanges polls /proc/net/route every 5s as fallback when netlink is banned.
func (t *AndroidTun) pollRouteChanges() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if updater != nil {
				updater.Update()
			}
		case <-t.routeMonitorStop:
			return
		}
	}
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

// ---------------------------------------------------------------------------
// Outbound interface binding
// ---------------------------------------------------------------------------

func setinterface(network, address string, fd uintptr, iface *net.Interface) error {
	return unix.BindToDevice(int(fd), iface.Name)
}

// findOutboundInterface resolves the physical interface outbound traffic binds
// to. A configured name is used as is. With no name the physical default route
// is looked up, because Android keeps it in a per-network table of the current
// network rather than in RT_TABLE_MAIN.
func findOutboundInterface(tunIndex int, fixedName string) (*net.Interface, error) {
	if fixedName != "" {
		iface, err := net.InterfaceByName(fixedName)
		if err != nil {
			return nil, err
		}
		if iface.Index == tunIndex {
			return nil, errors.New("outbound interface cannot be the TUN interface")
		}
		return iface, nil
	}

	for _, family := range []int{unix.AF_INET, unix.AF_INET6} {
		table, err := androidDefaultRouteTable(family)
		if err != nil {
			continue
		}
		routes, err := netlink.RouteListFiltered(netlink.FAMILY_ALL, &netlink.Route{Table: table}, netlink.RT_FILTER_TABLE)
		if err != nil {
			continue
		}
		if iface := androidDefaultInterface(routes, tunIndex); iface != nil {
			return iface, nil
		}
	}

	if iface := procDefaultInterface(tunIndex); iface != nil {
		return iface, nil
	}

	return nil, errors.New("no usable outbound interface found")
}

// androidDefaultRouteTable scans the ip rules for the table Android keeps the
// default route of the current network in: a per-network table (e.g. wlan0),
// not RT_TABLE_MAIN. Its rule carries the full mask.
func androidDefaultRouteTable(family int) (int, error) {
	rules, err := netlink.RuleList(family)
	if err != nil {
		return 0, errors.New("netlink rule list failed").Base(err)
	}
	for _, r := range rules {
		if r.Table != unix.RT_TABLE_MAIN && r.Table != unix.RT_TABLE_LOCAL && r.Mask != nil && *r.Mask == 0xFFFF {
			return r.Table, nil
		}
	}
	return 0, errors.New("no Android default route table found")
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

// procDefaultInterface finds the interface with the IPv4 default route by
// scanning /proc/net/route, for when netlink is banned by the platform.
func procDefaultInterface(tunIndex int) *net.Interface {
	f, err := os.Open("/proc/net/route")
	if err != nil {
		return nil
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 || fields[1] != "00000000" {
			continue
		}
		iface, err := net.InterfaceByName(fields[0])
		if err != nil {
			continue
		}
		if usableOutboundInterface(iface, tunIndex) {
			return iface
		}
	}
	return nil
}
