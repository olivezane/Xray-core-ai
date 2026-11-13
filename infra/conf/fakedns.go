package conf

import (
	"context"
	"encoding/json"
	"net"
	"net/netip"
	"strings"

	"github.com/xtls/xray-core/app/dns/fakedns"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/features/dns"
)

type FakeDNSPoolElementConfig struct {
	IPPool  string `json:"ipPool"`
	LRUSize int64  `json:"poolSize"`
}

type FakeDNSConfig struct {
	pool  *FakeDNSPoolElementConfig
	pools []*FakeDNSPoolElementConfig
}

// MarshalJSON implements encoding/json.Marshaler.MarshalJSON
func (f *FakeDNSConfig) MarshalJSON() ([]byte, error) {
	if (f.pool != nil) != (f.pools != nil) {
		if f.pool != nil {
			return json.Marshal(f.pool)
		} else if f.pools != nil {
			return json.Marshal(f.pools)
		}
	}
	return nil, errors.New("unexpected config state")
}

// UnmarshalJSON implements encoding/json.Unmarshaler.UnmarshalJSON
func (f *FakeDNSConfig) UnmarshalJSON(data []byte) error {
	var pool FakeDNSPoolElementConfig
	var pools []*FakeDNSPoolElementConfig
	switch {
	case json.Unmarshal(data, &pool) == nil:
		f.pool = &pool
	case json.Unmarshal(data, &pools) == nil:
		f.pools = pools
	default:
		return errors.New("invalid fakedns config")
	}
	return nil
}

func (f *FakeDNSConfig) Build() (*fakedns.FakeDnsPoolMulti, error) {
	fakeDNSPool := fakedns.FakeDnsPoolMulti{}

	if f.pool != nil {
		fakeDNSPool.Pools = append(fakeDNSPool.Pools, &fakedns.FakeDnsPool{
			IpPool:  f.pool.IPPool,
			LruSize: f.pool.LRUSize,
		})
		return &fakeDNSPool, nil
	}

	if f.pools != nil {
		for _, v := range f.pools {
			fakeDNSPool.Pools = append(fakeDNSPool.Pools, &fakedns.FakeDnsPool{IpPool: v.IPPool, LruSize: v.LRUSize})
		}
		return &fakeDNSPool, nil
	}

	return nil, errors.New("no valid FakeDNS config")
}

type FakeDNSPostProcessingStage struct{}

func (FakeDNSPostProcessingStage) Process(config *Config) error {
	fakeDNSInUse := false
	isIPv4Enable, isIPv6Enable := true, true

	if config.DNSConfig != nil {
		for _, v := range config.DNSConfig.Servers {
			if v.Address.Family().IsDomain() && strings.EqualFold(v.Address.Domain(), "fakedns") {
				fakeDNSInUse = true
			}
		}

		switch strings.ToLower(config.DNSConfig.QueryStrategy) {
		case "useip4", "useipv4", "use_ip4", "use_ipv4", "use_ip_v4", "use-ip4", "use-ipv4", "use-ip-v4":
			isIPv4Enable, isIPv6Enable = true, false
		case "useip6", "useipv6", "use_ip6", "use_ipv6", "use_ip_v6", "use-ip6", "use-ipv6", "use-ip-v6":
			isIPv4Enable, isIPv6Enable = false, true
		}
	}

	if fakeDNSInUse {
		tunConfigs := fakeDNSTunConfigs(config)
		hasTun, hasTunIPv6 := false, false
		for _, tunConfig := range tunConfigs {
			hasTun = true
			hasTunIPv6 = hasTunIPv6 || tunHasIPv6(tunConfig)
		}
		if isIPv4Enable && isIPv6Enable && hasTun && !hasTunIPv6 {
			// A fake AAAA on a TUN without IPv6 is unroutable.
			isIPv6Enable = false
		}

		// Add a Fake DNS Config if there is none
		if config.FakeDNS == nil {
			config.FakeDNS = &FakeDNSConfig{}
			switch {
			case isIPv4Enable && isIPv6Enable:
				config.FakeDNS.pools = []*FakeDNSPoolElementConfig{
					{
						IPPool:  dns.FakeIPv4Pool,
						LRUSize: 32768,
					},
					{
						IPPool:  dns.FakeIPv6Pool,
						LRUSize: 32768,
					},
				}
			case !isIPv4Enable && isIPv6Enable:
				config.FakeDNS.pool = &FakeDNSPoolElementConfig{
					IPPool:  dns.FakeIPv6Pool,
					LRUSize: 65535,
				}
			case isIPv4Enable && !isIPv6Enable:
				config.FakeDNS.pool = &FakeDNSPoolElementConfig{
					IPPool:  dns.FakeIPv4Pool,
					LRUSize: 65535,
				}
			}
		}

		if err := validateFakeDNSPools(config.FakeDNS, tunConfigs); err != nil {
			return err
		}

		// Check if there is a Inbound with necessary sniffer on
		for _, v := range config.InboundConfigs {
			if v.SniffingConfig != nil && v.SniffingConfig.Enabled {
				for _, d := range v.SniffingConfig.DestOverride {
					if strings.EqualFold(d, "fakedns") || strings.EqualFold(d, "fakedns+others") {
						return nil
					}
				}
			}
		}
		errors.LogWarning(context.Background(), "Defined FakeDNS but haven't enabled FakeDNS destOverride at any inbound.")
	}

	return nil
}

// fakeDNSTunConfigs parses the settings of the TUN inbounds, skipping ones the
// inbound itself will reject.
func fakeDNSTunConfigs(config *Config) []*TunConfig {
	var tunConfigs []*TunConfig
	for i := range config.InboundConfigs {
		inbound := &config.InboundConfigs[i]
		if !strings.EqualFold(inbound.Protocol, "tun") || inbound.Settings == nil {
			continue
		}
		tunConfig := new(TunConfig)
		if err := json.Unmarshal(*inbound.Settings, tunConfig); err != nil {
			continue
		}
		tunConfigs = append(tunConfigs, tunConfig)
	}
	return tunConfigs
}

func tunHasIPv6(config *TunConfig) bool {
	for _, prefixes := range [][]string{config.Gateway, config.AutoSystemRoutingTable} {
		for _, prefix := range prefixes {
			if parsed, err := netip.ParsePrefix(prefix); err == nil && parsed.Addr().Is6() {
				return true
			}
		}
	}
	return false
}

// validateFakeDNSPools rejects a pool that would hand out the TUN's own address
// or its DNS address, which the pool never allocates: connections to them would
// not reach the TUN.
func validateFakeDNSPools(fakeDNS *FakeDNSConfig, tunConfigs []*TunConfig) error {
	pools := fakeDNS.pools
	if fakeDNS.pool != nil {
		pools = []*FakeDNSPoolElementConfig{fakeDNS.pool}
	}
	for _, pool := range pools {
		_, ipRange, err := net.ParseCIDR(pool.IPPool)
		if err != nil {
			continue // the pool is rejected when it is initialized
		}
		var addrs []netip.Addr
		for _, tunConfig := range tunConfigs {
			for _, gateway := range tunConfig.Gateway {
				prefix, err := netip.ParsePrefix(gateway)
				if err != nil {
					continue
				}
				addrs = append(addrs, prefix.Addr(), prefix.Addr().Next())
			}
			for _, dnsServer := range tunConfig.DNS {
				if addr, err := netip.ParseAddr(dnsServer); err == nil {
					addrs = append(addrs, addr)
				}
			}
		}
		for _, addr := range addrs {
			if !addr.IsValid() {
				continue
			}
			ip := net.IP(addr.AsSlice())
			if ipRange.Contains(ip) && !fakedns.IsReservedIP(ipRange, ip) {
				return errors.New("fakeDNS pool ", pool.IPPool, " contains the TUN address ", addr, "; change the fake DNS pool or the TUN gateway")
			}
		}
	}
	return nil
}
