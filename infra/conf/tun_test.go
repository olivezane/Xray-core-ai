package conf_test

import (
	"testing"

	. "github.com/xtls/xray-core/infra/conf"
	"github.com/xtls/xray-core/proxy/tun"
)

func TestTunConfigStackSelection(t *testing.T) {
	creator := func() Buildable {
		return new(TunConfig)
	}

	runMultiTestCase(t, []TestCase{
		{
			Input:  `{"name": "tun0"}`,
			Parser: loadJSON(creator),
			Output: &tun.Config{
				Name:  "tun0",
				Desc:  "Wintun",
				MTU:   1500,
				Stack: tun.StackGVisor,
			},
		},
		{
			Input:  `{"name": "tun0", "stack": "MIPSTACK"}`,
			Parser: loadJSON(creator),
			Output: &tun.Config{
				Name:          "tun0",
				Desc:          "Wintun",
				MTU:           1500,
				Stack:         tun.StackMipstack,
				TcpCongestion: "cubic",
			},
		},
		{
			Input:  `{"name": "tun0", "stack": "mipstack", "tcpCongestion": "bbr3"}`,
			Parser: loadJSON(creator),
			Output: &tun.Config{
				Name:          "tun0",
				Desc:          "Wintun",
				MTU:           1500,
				Stack:         tun.StackMipstack,
				TcpCongestion: "bbr3",
			},
		},
	})
}

func TestTunConfigRejectsAStackSelectionItCannotUse(t *testing.T) {
	parser := loadJSON(func() Buildable {
		return new(TunConfig)
	})

	for _, input := range []string{
		`{"name": "tun0", "stack": "tun2socks"}`,
		`{"name": "tun0", "stack": "mips"}`,
		`{"name": "tun0", "stack": "mipstack", "tcpCongestion": "vegas"}`,
		`{"name": "tun0", "stack": "gvisor", "tcpCongestion": "bbr"}`,
		`{"name": "tun0", "tcpCongestion": "bbr"}`,
	} {
		if _, err := parser(input); err == nil {
			t.Errorf("configuration %s must be rejected", input)
		}
	}
}
