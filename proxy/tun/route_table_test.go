package tun

import (
	"errors"
	"net"
	"testing"

	"github.com/vishvananda/netlink"
)

func fakeInterface(index int, name string, up, loopback bool) *net.Interface {
	var flags net.Flags
	if up {
		flags |= net.FlagUp
	}
	if loopback {
		flags |= net.FlagLoopback
	}
	return &net.Interface{Index: index, Name: name, Flags: flags}
}

func TestUsablePhysicalInterface(t *testing.T) {
	const tunIndex = 7
	if usablePhysicalInterface(nil, tunIndex) {
		t.Error("nil interface must not be usable")
	}
	if usablePhysicalInterface(fakeInterface(tunIndex, "tun0", true, false), tunIndex) {
		t.Error("the TUN interface must not be usable")
	}
	if usablePhysicalInterface(fakeInterface(3, "eth0", false, false), tunIndex) {
		t.Error("a down interface must not be usable")
	}
	if usablePhysicalInterface(fakeInterface(3, "lo", true, true), tunIndex) {
		t.Error("a loopback interface must not be usable")
	}
	if !usablePhysicalInterface(fakeInterface(3, "eth0", true, false), tunIndex) {
		t.Error("an up, non-loopback physical interface must be usable")
	}
}

func TestDefaultInterfaceFromRoutes(t *testing.T) {
	const tunIndex = 5
	byIndex := func(index int) *net.Interface {
		switch index {
		case 2:
			return fakeInterface(2, "eth0", true, false)
		case 3:
			return fakeInterface(3, "eth1", false, false) // down
		case 4:
			return fakeInterface(4, "lo", true, true) // loopback
		case 5:
			return fakeInterface(5, "tun0", true, false) // the TUN itself
		default:
			return nil // unknown index
		}
	}

	t.Run("picks the only usable interface", func(t *testing.T) {
		got := defaultInterfaceFromRoutes([]routeEntry{
			{linkIndex: 4, priority: 10}, // loopback
			{linkIndex: 3, priority: 10}, // down
			{linkIndex: 2, priority: 100},
			{linkIndex: 9, priority: 1}, // unknown index
		}, tunIndex, byIndex)
		if got == nil || got.Index != 2 {
			t.Fatalf("expected eth0 (index 2), got %v", got)
		}
	})

	t.Run("lowest priority wins", func(t *testing.T) {
		got := defaultInterfaceFromRoutes([]routeEntry{
			{linkIndex: 6, priority: 100},
			{linkIndex: 2, priority: 10},
		}, tunIndex, func(index int) *net.Interface {
			if iface := byIndex(index); iface != nil {
				return iface
			}
			return fakeInterface(6, "eth2", true, false)
		})
		if got == nil || got.Index != 2 {
			t.Fatalf("expected eth0 (index 2, priority 10), got %v", got)
		}
	})

	t.Run("ties keep first in list order", func(t *testing.T) {
		got := defaultInterfaceFromRoutes([]routeEntry{
			{linkIndex: 6, priority: 10},
			{linkIndex: 2, priority: 10},
		}, tunIndex, func(index int) *net.Interface {
			if iface := byIndex(index); iface != nil {
				return iface
			}
			return fakeInterface(6, "eth2", true, false)
		})
		if got == nil || got.Index != 6 {
			t.Fatalf("expected eth2 (index 6, first in list), got %v", got)
		}
	})

	t.Run("the TUN index is excluded", func(t *testing.T) {
		got := defaultInterfaceFromRoutes([]routeEntry{
			{linkIndex: 5, priority: 1},
			{linkIndex: 2, priority: 10},
		}, tunIndex, byIndex)
		if got == nil || got.Index != 2 {
			t.Fatalf("expected eth0 (index 2), got %v", got)
		}
	})

	t.Run("no candidates yields nil", func(t *testing.T) {
		if got := defaultInterfaceFromRoutes(nil, tunIndex, byIndex); got != nil {
			t.Fatalf("expected nil, got %v", got)
		}
		if got := defaultInterfaceFromRoutes([]routeEntry{{linkIndex: 4, priority: 1}}, tunIndex, byIndex); got != nil {
			t.Fatalf("expected nil for loopback-only routes, got %v", got)
		}
	})
}

func TestDefaultRoutes(t *testing.T) {
	routes := []netlink.Route{
		{Dst: nil, LinkIndex: 2, Priority: 10},                                                              // default
		{Dst: &net.IPNet{IP: net.IPv4zero, Mask: net.CIDRMask(0, 32)}, LinkIndex: 3, Priority: 20},          // default
		{Dst: &net.IPNet{IP: net.IPv4(10, 0, 0, 0), Mask: net.CIDRMask(8, 32)}, LinkIndex: 4, Priority: 30}, // not default
		{Dst: nil, LinkIndex: 0, Priority: 40},                                                              // no link index
	}
	got := defaultRoutes(routes)
	if len(got) != 2 {
		t.Fatalf("expected 2 default-route candidates, got %d", len(got))
	}
	if got[0].linkIndex != 2 || got[0].priority != 10 {
		t.Errorf("unexpected first candidate: %+v", got[0])
	}
	if got[1].linkIndex != 3 || got[1].priority != 20 {
		t.Errorf("unexpected second candidate: %+v", got[1])
	}
}

func TestIsDefaultRoute(t *testing.T) {
	if !isDefaultRoute(nil) {
		t.Error("nil destination is a default route")
	}
	if !isDefaultRoute(&net.IPNet{IP: net.IPv4zero, Mask: net.CIDRMask(0, 32)}) {
		t.Error("0.0.0.0/0 is a default route")
	}
	if isDefaultRoute(&net.IPNet{IP: net.IPv4(192, 168, 1, 0), Mask: net.CIDRMask(24, 32)}) {
		t.Error("192.168.1.0/24 is not a default route")
	}
}

type fakeRouteTable struct {
	iface *net.Interface
	err   error
}

func (f *fakeRouteTable) OutboundInterface(int, string) (*net.Interface, error) {
	return f.iface, f.err
}

func TestInterfaceUpdaterKeepsLastKnownGoodDuringTransitions(t *testing.T) {
	updater := &InterfaceUpdater{table: &fakeRouteTable{}}
	first := fakeInterface(2, "eth0", true, false)
	second := fakeInterface(3, "eth1", true, false)

	updater.table = &fakeRouteTable{iface: first}
	updater.Update()
	if got := updater.Get(); got == nil || got.Index != 2 {
		t.Fatalf("expected eth0 after first update, got %v", got)
	}

	// a failed lookup (e.g. wifi -> cellular handover) must not drop the binding
	updater.table = &fakeRouteTable{err: ErrNoOutboundInterface}
	updater.Update()
	if got := updater.Get(); got == nil || got.Index != 2 {
		t.Fatalf("expected last known good interface to survive a not-found update, got %v", got)
	}

	// a real lookup error must not drop it either
	updater.table = &fakeRouteTable{err: errors.New("netlink is gone")}
	updater.Update()
	if got := updater.Get(); got == nil || got.Index != 2 {
		t.Fatalf("expected last known good interface to survive an error update, got %v", got)
	}

	// recovery refreshes the binding
	updater.table = &fakeRouteTable{iface: second}
	updater.Update()
	if got := updater.Get(); got == nil || got.Index != 3 {
		t.Fatalf("expected eth1 after recovery, got %v", got)
	}

	// Reset clears it
	updater.Reset()
	if got := updater.Get(); got != nil {
		t.Fatalf("expected nil after Reset, got %v", got)
	}
}

func TestAndroidRuleFamilies(t *testing.T) {
	t.Run("no routes means no rules", func(t *testing.T) {
		if got := androidRuleFamilies(nil); len(got) != 0 {
			t.Fatalf("expected no families for empty cidrs, got %v", got)
		}
		if got := androidRuleFamilies([]string{}); len(got) != 0 {
			t.Fatalf("expected no families for empty cidrs, got %v", got)
		}
	})
	t.Run("IPv4 only", func(t *testing.T) {
		got := androidRuleFamilies([]string{"0.0.0.0/0", "10.0.0.0/8"})
		if len(got) != 1 || got[0] != androidFamilyIPv4 {
			t.Fatalf("expected only IPv4 family, got %v", got)
		}
	})
	t.Run("IPv6 only", func(t *testing.T) {
		got := androidRuleFamilies([]string{"::/0"})
		if len(got) != 1 || got[0] != androidFamilyIPv6 {
			t.Fatalf("expected only IPv6 family, got %v", got)
		}
	})
	t.Run("mixed dedupes", func(t *testing.T) {
		got := androidRuleFamilies([]string{"0.0.0.0/0", "::/0", "2001:db8::/32"})
		if len(got) != 2 {
			t.Fatalf("expected two families (v4, v6), got %v", got)
		}
	})
}
