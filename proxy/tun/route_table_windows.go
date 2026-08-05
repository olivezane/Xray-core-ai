//go:build windows

package tun

import (
	"net"
	"sort"
	"strings"
)

// routeTableWindows resolves the physical outbound interface from the Windows
// interface list. Automatic selection scores candidates (wlan preferred,
// hyper-v vEthernet adapters excluded) and picks the best.
type routeTableWindows struct{}

func newRouteTable() RouteTable { return routeTableWindows{} }

func (routeTableWindows) OutboundInterface(tunIndex int, fixedName string) (*net.Interface, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	if fixedName != "" {
		for _, iface := range interfaces {
			if iface.Index != tunIndex && iface.Name == fixedName {
				return &iface, nil
			}
		}
		return nil, ErrNoOutboundInterface
	}

	var candidates []struct {
		index int
		score int
	}
	for i, iface := range interfaces {
		if strings.Contains(iface.Name, "vEthernet") || !usablePhysicalInterface(&iface, tunIndex) {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil || len(addrs) == 0 {
			continue
		}
		candidates = append(candidates, struct {
			index int
			score int
		}{i, scoreWindowsInterface(&iface, addrs)})
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		return interfaces[candidates[i].index].Name < interfaces[candidates[j].index].Name
	})
	if len(candidates) == 0 {
		return nil, ErrNoOutboundInterface
	}

	iface := interfaces[candidates[0].index]
	return &iface, nil
}

func scoreWindowsInterface(iface *net.Interface, addrs []net.Addr) int {
	score := 0

	name := strings.ToLower(iface.Name)
	if strings.Contains(name, "wlan") || strings.Contains(name, "wi-fi") {
		score += 2
	}

	for _, addr := range addrs {
		if strings.HasPrefix(addr.String(), "192.168.") {
			score++
			break
		}
	}

	return score
}
