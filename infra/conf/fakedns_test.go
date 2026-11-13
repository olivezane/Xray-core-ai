package conf_test

import (
	"encoding/json"
	"net/netip"
	"strings"
	"testing"

	"github.com/xtls/xray-core/features/dns"
	. "github.com/xtls/xray-core/infra/conf"
)

func postProcessFakeDNS(t *testing.T, input string) (*Config, error) {
	t.Helper()
	config := new(Config)
	if err := json.Unmarshal([]byte(input), config); err != nil {
		t.Fatal(err)
	}
	return config, PostProcessConfigureFile(config)
}

func fakeDNSPools(t *testing.T, config *Config) []string {
	t.Helper()
	multi, err := config.FakeDNS.Build()
	if err != nil {
		t.Fatal(err)
	}
	pools := make([]string, 0, len(multi.Pools))
	for _, pool := range multi.Pools {
		pools = append(pools, pool.IpPool)
	}
	return pools
}

func TestFakeDNSAutoPoolsWithTun(t *testing.T) {
	const withFakeDNS = `{"dns":{"servers":[{"address":"fakedns"}]}`

	v6Prefix, err := netip.ParsePrefix(dns.FakeIPv6Pool)
	if err != nil {
		t.Fatal(err)
	}
	v6Gateway := netip.PrefixFrom(v6Prefix.Addr().Next(), 64).String()

	t.Run("no tun", func(t *testing.T) {
		config, err := postProcessFakeDNS(t, withFakeDNS+`}`)
		if err != nil {
			t.Fatal(err)
		}
		want := []string{dns.FakeIPv4Pool, dns.FakeIPv6Pool}
		if got := fakeDNSPools(t, config); strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("pools %v, want %v", got, want)
		}
	})

	t.Run("ipv4 tun", func(t *testing.T) {
		config, err := postProcessFakeDNS(t, withFakeDNS+`,"inbounds":[{"protocol":"tun","settings":{"gateway":["10.0.0.1/16"]}}]}`)
		if err != nil {
			t.Fatal(err)
		}
		if got := fakeDNSPools(t, config); len(got) != 1 || got[0] != dns.FakeIPv4Pool {
			t.Fatalf("pools %v, want only %s", got, dns.FakeIPv4Pool)
		}
	})

	t.Run("ipv6 tun", func(t *testing.T) {
		config, err := postProcessFakeDNS(t, withFakeDNS+`,"inbounds":[{"protocol":"tun","settings":{"gateway":["`+v6Gateway+`"]}}]}`)
		if err != nil {
			t.Fatal(err)
		}
		if got := fakeDNSPools(t, config); len(got) != 2 {
			t.Fatalf("pools %v, want both pools", got)
		}
	})

	t.Run("ipv6 route", func(t *testing.T) {
		config, err := postProcessFakeDNS(t, withFakeDNS+`,"inbounds":[{"protocol":"tun","settings":{"gateway":["10.0.0.1/16"],"autoSystemRoutingTable":["::/0"]}}]}`)
		if err != nil {
			t.Fatal(err)
		}
		if got := fakeDNSPools(t, config); len(got) != 2 {
			t.Fatalf("pools %v, want both pools", got)
		}
	})
}

func TestFakeDNSPoolOverlapsTun(t *testing.T) {
	config := `{"dns":{"servers":[{"address":"fakedns"}]},"inbounds":[{"protocol":"tun","settings":{"gateway":["%s"]}}]}`

	_, err := postProcessFakeDNS(t, strings.Replace(config, "%s", "198.18.0.5/15", 1))
	if err == nil || !strings.Contains(err.Error(), "198.18.0.5") {
		t.Fatalf("want an overlap error for 198.18.0.5, got %v", err)
	}

	// The network address, .1 and .2 are reserved for the TUN and its DNS.
	if _, err := postProcessFakeDNS(t, strings.Replace(config, "%s", "198.18.0.1/15", 1)); err != nil {
		t.Fatal(err)
	}

	_, err = postProcessFakeDNS(t, `{"dns":{"servers":[{"address":"fakedns"}]},"inbounds":[{"protocol":"tun","settings":{"gateway":["10.0.0.1/16"],"dns":["198.18.0.9"]}}]}`)
	if err == nil || !strings.Contains(err.Error(), "198.18.0.9") {
		t.Fatalf("want an overlap error for the TUN DNS address, got %v", err)
	}
}
