package transports

import (
	"testing"

	"github.com/xtls/xray-core/transport/internet"
	"github.com/xtls/xray-core/transport/internet/hysteria"
	"github.com/xtls/xray-core/transport/internet/kcp"
	"github.com/xtls/xray-core/transport/internet/masque"
)

// TestAllIsInstalled guards that the list above is what internet looks up: a
// transport that is listed but not installed, or the other way round, is a
// transport nobody can use.
func TestAllIsInstalled(t *testing.T) {
	installed := internet.TransportNames()
	if len(installed) != len(All) {
		t.Fatalf("installed %d transports %v, but %d are listed", len(installed), installed, len(All))
	}
	for _, transport := range All {
		if installed, found := internet.TransportByName(transport.Name); !found || installed.Name != transport.Name {
			t.Errorf("transport %q is listed but not installed", transport.Name)
		}
	}
}

// TestAllIsWired checks every line of the list: a name, something that dials,
// and a listener for everything but udp, which is only dialed.
func TestAllIsWired(t *testing.T) {
	seen := make(map[string]bool, len(All))
	for _, transport := range All {
		if transport.Name == "" {
			t.Error("a listed transport has no name")
		}
		if seen[transport.Name] {
			t.Errorf("transport %q is listed twice", transport.Name)
		}
		seen[transport.Name] = true
		if transport.Dial == nil {
			t.Errorf("transport %q cannot dial", transport.Name)
		}
		if transport.Listen == nil && transport.Name != "udp" {
			t.Errorf("transport %q cannot listen", transport.Name)
		}
	}
}

// TestAllHasConfigCreator catches the drift that matters after wiring: a listed
// transport whose protocol name has no config creator can be dialed and
// listened on, but its settings cannot be decoded from JSON.
func TestAllHasConfigCreator(t *testing.T) {
	for _, transport := range All {
		if _, err := internet.CreateTransportConfig(transport.Name); err != nil {
			t.Errorf("transport %q has no config creator: %v", transport.Name, err)
		}
	}
}

// TestDefaultsSurviveTheMove checks the settings that used to be written in the
// per-package init() functions: they are defaults of the transport, so they must
// still be there when a stream settings entry carries no settings of its own.
func TestDefaultsSurviveTheMove(t *testing.T) {
	kcpConfig, err := internet.CreateTransportConfig("mkcp")
	if err != nil {
		t.Fatalf("mkcp: %v", err)
	}
	mkcp, ok := kcpConfig.(*kcp.Config)
	if !ok {
		t.Fatalf("mkcp config is %T, want *kcp.Config", kcpConfig)
	}
	if mkcp.Mtu != 1350 || mkcp.Tti != 50 || mkcp.UplinkCapacity != 5 || mkcp.DownlinkCapacity != 20 || mkcp.CwndMultiplier != 1 || mkcp.MaxSendingWindow != 2*1024*1024 {
		t.Errorf("mkcp defaults changed: %v", mkcp)
	}

	masqueConfig, err := internet.CreateTransportConfig("masque")
	if err != nil {
		t.Fatalf("masque: %v", err)
	}
	masqueDef, ok := masqueConfig.(*masque.Config)
	if !ok || masqueDef.Path != masque.DefaultPath {
		t.Errorf("masque defaults changed: %#v", masqueConfig)
	}

	hysteriaConfig, err := internet.CreateTransportConfig("hysteria")
	if err != nil {
		t.Fatalf("hysteria: %v", err)
	}
	if hysteriaConfig.(*hysteria.Config).UdpIdleTimeout != 60 {
		t.Errorf("hysteria defaults changed: %#v", hysteriaConfig)
	}
}
