//go:build android

package tun

import (
	"bufio"
	"net"
	"os"
	"strings"

	"github.com/vishvananda/netlink"
	"github.com/xtls/xray-core/common/errors"
	"golang.org/x/sys/unix"
)

// routeTableAndroid resolves the physical outbound interface. On Android the
// default route lives in a per-network table, not RT_TABLE_MAIN, so it is
// located via ip rules, with a /proc/net/route fallback for when netlink is
// banned by the platform.
type routeTableAndroid struct{}

func newRouteTable() RouteTable { return routeTableAndroid{} }

func (routeTableAndroid) OutboundInterface(tunIndex int, fixedName string) (*net.Interface, error) {
	if fixedName != "" {
		return byFixedName(fixedName, tunIndex)
	}

	table, err := androidDefaultRouteTable(unix.AF_INET)
	if err != nil {
		table, err = androidDefaultRouteTable(unix.AF_INET6)
	}
	if err == nil {
		routes, err := netlink.RouteListFiltered(netlink.FAMILY_ALL,
			&netlink.Route{Table: table}, netlink.RT_FILTER_TABLE)
		if err == nil {
			if iface := defaultInterfaceFromRoutes(defaultRoutes(routes), tunIndex, netInterfaceByIndex); iface != nil {
				return iface, nil
			}
		}
	}

	if iface := procDefaultRoute(tunIndex); iface != nil {
		return iface, nil
	}
	return nil, ErrNoOutboundInterface
}

// androidDefaultRouteTable scans ip rules to find the routing table
// Android uses for the physical default route.
//
// On Android the default route lives in a per-network table (e.g. wlan0),
// not RT_TABLE_MAIN. The ip rules entry for it has Mask=0xFFFF.
// This matches sing-tun's detection logic.
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

// procDefaultRoute finds the physical interface with the IPv4 default route
// by scanning /proc/net/route, used as fallback when netlink is banned.
func procDefaultRoute(tunIndex int) *net.Interface {
	f, err := os.Open("/proc/net/route")
	if err != nil {
		return nil
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "Iface") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[1] != "00000000" {
			continue
		}
		iface, err := net.InterfaceByName(fields[0])
		if err != nil {
			continue
		}
		if usablePhysicalInterface(iface, tunIndex) {
			return iface
		}
	}
	return nil
}
