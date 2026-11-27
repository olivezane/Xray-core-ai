package conf

import (
	"context"
	"encoding/json"
	"net"
	"net/netip"
	"strings"

	"github.com/xtls/xray-core/app/dns/fakedns"
	"github.com/xtls/xray-core/common/errors"
	coreconfig "github.com/xtls/xray-core/core/config"
	"github.com/xtls/xray-core/features/dns"
)

// FakeDNSPoolElementConfig is one fake DNS pool of the configuration model.
type FakeDNSPoolElementConfig = coreconfig.FakeDNSPool

// FakeDNSConfig is the fake DNS section of the configuration model.
type FakeDNSConfig = coreconfig.FakeDNS


// BuildFakeDNS converts the fake DNS section to its message.
func BuildFakeDNS(f *coreconfig.FakeDNS) (*fakedns.FakeDnsPoolMulti, error) {
	pools := f.Pools()
	if len(pools) == 0 {
		return nil, errors.New("no valid FakeDNS config")
	}
	built := new(fakedns.FakeDnsPoolMulti)
	for _, v := range pools {
		built.Pools = append(built.Pools, &fakedns.FakeDnsPool{IpPool: v.IPPool, LruSize: v.LRUSize})
	}
	return built, nil
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
		if config.DNSConfig.FakeDNS == nil {
			config.DNSConfig.FakeDNS = &FakeDNSConfig{}
			switch {
			case isIPv4Enable && isIPv6Enable:
				config.DNSConfig.FakeDNS.SetPools([]*FakeDNSPoolElementConfig{
					{
						IPPool:  dns.FakeIPv4Pool,
						LRUSize: 32768,
					},
					{
						IPPool:  dns.FakeIPv6Pool,
						LRUSize: 32768,
					},
				})
			case !isIPv4Enable && isIPv6Enable:
				config.DNSConfig.FakeDNS.SetPool(&FakeDNSPoolElementConfig{
					IPPool:  dns.FakeIPv6Pool,
					LRUSize: 65535,
				})
			case isIPv4Enable && !isIPv6Enable:
				config.DNSConfig.FakeDNS.SetPool(&FakeDNSPoolElementConfig{
					IPPool:  dns.FakeIPv4Pool,
					LRUSize: 65535,
				})
			}
		}

		if err := validateFakeDNSPools(config.DNSConfig.FakeDNS, tunConfigs); err != nil {
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
	pools := fakeDNS.Pools()
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
