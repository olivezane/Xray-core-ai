package shadowsocks_2022_test

import (
	"os"
	"testing"

	ss2022 "github.com/xtls/xray-core/proxy/shadowsocks_2022"
	"github.com/xtls/xray-core/testing/configs"
)

// TestMain says which configurations this package reads: an account crosses the
// API by name like a setting, so the names of this package are installed here.
func TestMain(m *testing.M) {
	configs.MustInstall(
		ss2022.ConfigObjectsShadowsocks2022Inbound,
		ss2022.ConfigObjectsShadowsocks2022InboundMulti,
		ss2022.ConfigObjectsShadowsocks2022InboundRelay,
		ss2022.ConfigObjectsShadowsocks2022Outbound,
	)
	os.Exit(m.Run())
}
