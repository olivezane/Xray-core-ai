package all_test

import (
	"testing"

	"github.com/xtls/xray-core/common/platform"
	"github.com/xtls/xray-core/main/distro/all"
)

// TestWireInstallsTheBuild checks that the wiring main calls before it starts
// really installs what the binary used to get from init() functions: the
// configuration table, the environment-backed settings and the tagged dialer.
func TestWireInstallsTheBuild(t *testing.T) {
	all.Wire()

	// The environment-backed settings are installed by Wire, so reloading them
	// runs the handlers instead of finding none.
	if err := platform.ReloadEnvSettings(); err != nil {
		t.Fatalf("reloading environment-backed settings failed: %v", err)
	}
}
