package tun

import (
	"net"
	"testing"
)

func loopbackName(t *testing.T) string {
	t.Helper()
	ifaces, err := net.Interfaces()
	if err != nil {
		t.Fatal(err)
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			return iface.Name
		}
	}
	t.Fatal("no loopback interface")
	return ""
}

// An update that lands between two networks leaves no interface behind, and
// the next event may never come: the next dial has to look again.
func TestInterfaceUpdaterGetRetriesWhenNil(t *testing.T) {
	name := loopbackName(t)
	u := &InterfaceUpdater{fixedName: name}

	if got := u.Get(); got == nil || got.Name != name {
		t.Fatal("unresolved interface was not looked up: ", got)
	}

	// inside the retry window a still-unresolved value is not looked up again
	u.Lock()
	u.iface = nil
	u.Unlock()

	if got := u.Get(); got != nil {
		t.Error("retried inside the retry window: ", got)
	}
}

func TestInterfaceUpdaterGetClosed(t *testing.T) {
	u := &InterfaceUpdater{fixedName: loopbackName(t)}
	u.Close()

	if got := u.Get(); got != nil {
		t.Error("closed updater resolved an interface: ", got)
	}
}
