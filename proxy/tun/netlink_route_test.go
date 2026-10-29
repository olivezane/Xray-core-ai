//go:build linux

package tun

import (
	"net"
	"reflect"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

func TestAndroidRoutes(t *testing.T) {
	routes, err := androidRoutes([]string{"10.0.0.5/8", "2001:db8::1/32"}, 100, 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(routes))
	}
	if got := routes[0].Dst.String(); got != "10.0.0.0/8" {
		t.Error("host bits not masked: ", got)
	}
	if got := routes[1].Dst.String(); got != "2001:db8::/32" {
		t.Error("host bits not masked: ", got)
	}
	for _, r := range routes {
		if r.Table != 100 || r.LinkIndex != 7 {
			t.Error("route lost table/link: ", r)
		}
		if r.Gw != nil {
			t.Error("point-to-point TUN route must not carry a gateway")
		}
	}

	if _, err := androidRoutes([]string{"10.0.0.0/8", "not-a-cidr"}, 100, 7); err == nil {
		t.Error("invalid CIDR accepted")
	}
}

func TestAndroidRuleFamilies(t *testing.T) {
	cases := []struct {
		in   []string
		want []int
	}{
		{nil, nil},
		{[]string{"10.0.0.0/8"}, []int{unix.AF_INET}},
		{[]string{"10.0.0.0/8", "192.168.0.0/16"}, []int{unix.AF_INET}},
		{[]string{"2001:db8::/32"}, []int{unix.AF_INET6}},
		{[]string{"10.0.0.0/8", "2001:db8::/32"}, []int{unix.AF_INET, unix.AF_INET6}},
		{[]string{"garbage"}, nil},
		{[]string{"garbage", "10.0.0.0/8"}, []int{unix.AF_INET}},
	}
	for _, c := range cases {
		if got := androidRuleFamilies(c.in); !reflect.DeepEqual(got, c.want) {
			t.Error("androidRuleFamilies(", c.in, ") = ", got, ", want ", c.want)
		}
	}
}

func TestUsableOutboundInterface(t *testing.T) {
	up := &net.Interface{Index: 5, Flags: net.FlagUp}
	upLoopback := &net.Interface{Index: 1, Flags: net.FlagUp | net.FlagLoopback}
	down := &net.Interface{Index: 5}

	if !usableOutboundInterface(up, 6) {
		t.Error("up interface rejected")
	}
	if usableOutboundInterface(up, 5) {
		t.Error("TUN interface accepted as outbound")
	}
	if usableOutboundInterface(upLoopback, 6) {
		t.Error("loopback accepted as outbound")
	}
	if usableOutboundInterface(down, 6) {
		t.Error("down interface accepted as outbound")
	}
	if usableOutboundInterface(nil, 6) {
		t.Error("nil interface accepted as outbound")
	}
}

// Only the "nothing usable" branch is reachable without a live non-loopback
// interface; the metric and mask selection needs one, so it is not covered.
func TestAndroidDefaultInterfaceWithoutCandidate(t *testing.T) {
	cases := [][]netlink.Route{
		nil,
		{{LinkIndex: 0}},
		{{LinkIndex: 1}},       // loopback on linux
		{{LinkIndex: 1 << 20}}, // no such interface
		{{Dst: nil, LinkIndex: 1 << 20, Priority: 1}},
	}
	for _, routes := range cases {
		if iface := androidDefaultInterface(routes, 5); iface != nil {
			t.Error("unexpected interface ", iface.Name, " for ", routes)
		}
	}
}
