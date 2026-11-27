package conf

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/xtls/xray-core/app/dns"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/geodata"
	"github.com/xtls/xray-core/common/net"
	coreconfig "github.com/xtls/xray-core/core/config"
)

// NameServerConfig is one name server of the configuration model.
type NameServerConfig = coreconfig.NameServer

func BuildNameServer(c *coreconfig.NameServer) (*dns.NameServer, error) {
	if c.Address == nil {
		return nil, errors.New("nameserver address is not specified")
	}

	domainRules, err := geodata.ParseDomainRules(c.Domains, geodata.Domain_Substr)
	if err != nil {
		return nil, err
	}

	if len(c.ExpectedIPs) == 0 {
		c.ExpectedIPs = c.ExpectIPs
	}

	actPrior := false
	var newExpectedIPs StringList
	for _, s := range c.ExpectedIPs {
		if s == "*" {
			actPrior = true
		} else {
			newExpectedIPs = append(newExpectedIPs, s)
		}
	}

	actUnprior := false
	var newUnexpectedIPs StringList
	for _, s := range c.UnexpectedIPs {
		if s == "*" {
			actUnprior = true
		} else {
			newUnexpectedIPs = append(newUnexpectedIPs, s)
		}
	}

	expectedIPRules, err := geodata.ParseIPRules(newExpectedIPs)
	if err != nil {
		return nil, err
	}

	unexpectedIPRules, err := geodata.ParseIPRules(newUnexpectedIPs)
	if err != nil {
		return nil, err
	}

	var myClientIP []byte
	if c.ClientIP != nil {
		if !c.ClientIP.Family().IsIP() {
			return nil, errors.New("not an IP address:", c.ClientIP.String())
		}
		myClientIP = []byte(c.ClientIP.IP())
	}

	return &dns.NameServer{
		Id: c.ID,
		Address: &net.Endpoint{
			Network: net.Network_UDP,
			Address: BuildAddress(c.Address),
			Port:    uint32(c.Port),
		},
		ClientIp:        myClientIP,
		SkipFallback:    c.SkipFallback,
		Domain:          domainRules,
		ExpectedIp:      expectedIPRules,
		QueryStrategy:   resolveQueryStrategy(c.QueryStrategy),
		ActPrior:        actPrior,
		Tag:             c.Tag,
		TimeoutMs:       c.TimeoutMs,
		DisableCache:    c.DisableCache,
		ServeStale:      c.ServeStale,
		ServeExpiredTTL: c.ServeExpiredTTL,
		FinalQuery:      c.FinalQuery,
		UnexpectedIp:    unexpectedIPRules,
		ActUnprior:      actUnprior,
	}, nil
}

// DNSConfig is the "dns" section of the configuration.
type DNSConfig struct {
	coreconfig.DNS
}

// HostAddress is one host of the "hosts" table of the configuration model.
type HostAddress = coreconfig.HostAddress

// HostsWrapper is the "hosts" table of the configuration model.
type HostsWrapper = coreconfig.Hosts

func newHostMapping(ha *coreconfig.HostAddress) *dns.Config_HostMapping {
	if addr, single := ha.Single(); single {
		if addr.Family().IsDomain() {
			return &dns.Config_HostMapping{ProxiedDomain: addr.Domain()}
		}
		return &dns.Config_HostMapping{Ip: [][]byte{addr.IP()}}
	}

	addrs := ha.Addrs()
	ips := make([][]byte, 0, len(addrs))
	for _, addr := range addrs {
		if addr.Family().IsDomain() {
			return &dns.Config_HostMapping{ProxiedDomain: addr.Domain()}
		}
		ips = append(ips, []byte(addr.IP()))
	}
	return &dns.Config_HostMapping{Ip: ips}
}

// BuildHosts converts the "hosts" table to its messages.
func BuildHosts(m *coreconfig.Hosts) ([]*dns.Config_HostMapping, error) {
	mappings := make([]*dns.Config_HostMapping, 0, len(m.Hosts))
	for rule, addrs := range m.Hosts {
		mapping := newHostMapping(addrs)
		rule, err := geodata.ParseDomainRule(rule, geodata.Domain_Full)
		if err != nil {
			return nil, err
		}
		mapping.Domain = rule
		mappings = append(mappings, mapping)
	}
	return mappings, nil
}

// Build implements Buildable
func (c *DNSConfig) Build() (*dns.Config, error) {
	if c.Script != "" {
		if c.DisableFallback || c.DisableFallbackIfMatch || c.EnableParallelQuery {
			return nil, errors.New("DNS script cannot be used with disableFallback, disableFallbackIfMatch or enableParallelQuery")
		}
		for i, server := range c.Servers {
			if len(server.Domains) > 0 || server.SkipFallback || server.FinalQuery {
				return nil, errors.New("DNS script cannot be used with domains, skipFallback or finalQuery in server ", i)
			}
		}
	}

	config := &dns.Config{
		Tag:                    c.Tag,
		DisableCache:           c.DisableCache,
		ServeStale:             c.ServeStale,
		ServeExpiredTTL:        c.ServeExpiredTTL,
		DisableFallback:        c.DisableFallback,
		DisableFallbackIfMatch: c.DisableFallbackIfMatch,
		EnableParallelQuery:    c.EnableParallelQuery,
		QueryStrategy:          resolveQueryStrategy(c.QueryStrategy),
		Script:                 c.Script,
	}

	if c.ClientIP != nil {
		if !c.ClientIP.Family().IsIP() {
			return nil, errors.New("not an IP address:", c.ClientIP.String())
		}
		config.ClientIp = []byte(c.ClientIP.IP())
	}

	// Build PolicyID
	policyMap := map[string]uint32{}
	nextPolicyID := uint32(1)
	buildPolicyID := func(nsc *NameServerConfig) uint32 {
		var sb strings.Builder

		// ClientIP
		if nsc.ClientIP != nil {
			sb.WriteString("client=")
			sb.WriteString(nsc.ClientIP.String())
			sb.WriteByte('|')
		} else {
			sb.WriteString("client=none|")
		}

		// SkipFallback
		if nsc.SkipFallback {
			sb.WriteString("skip=1|")
		} else {
			sb.WriteString("skip=0|")
		}

		// QueryStrategy
		sb.WriteString("qs=")
		sb.WriteString(strings.ToLower(strings.TrimSpace(nsc.QueryStrategy)))
		sb.WriteByte('|')

		// Tag
		sb.WriteString("tag=")
		sb.WriteString(strings.ToLower(strings.TrimSpace(nsc.Tag)))
		sb.WriteByte('|')

		// []string helper
		writeList := func(tag string, lst []string) {
			if len(lst) == 0 {
				sb.WriteString(tag)
				sb.WriteString("=[]|")
				return
			}
			cp := make([]string, len(lst))
			for i, s := range lst {
				cp[i] = strings.TrimSpace(strings.ToLower(s))
			}
			sort.Strings(cp)
			sb.WriteString(tag)
			sb.WriteByte('=')
			sb.WriteString(strings.Join(cp, ","))
			sb.WriteByte('|')
		}

		writeList("domains", nsc.Domains)
		writeList("expected", nsc.ExpectedIPs)
		writeList("expect", nsc.ExpectIPs)
		writeList("unexpected", nsc.UnexpectedIPs)

		key := sb.String()

		if id, ok := policyMap[key]; ok {
			return id
		}
		id := nextPolicyID
		nextPolicyID++
		policyMap[key] = id
		return id
	}

	for _, server := range c.Servers {
		ns, err := BuildNameServer(server)
		if err != nil {
			return nil, errors.New("failed to build nameserver").Base(err)
		}
		ns.PolicyID = buildPolicyID(server)
		config.NameServer = append(config.NameServer, ns)
	}

	if c.Hosts != nil {
		staticHosts, err := BuildHosts(c.Hosts)
		if err != nil {
			return nil, errors.New("failed to build hosts").Base(err)
		}
		config.StaticHosts = append(config.StaticHosts, staticHosts...)
	}
	if c.UseSystemHosts {
		systemHosts, err := readSystemHosts()
		if err != nil {
			return nil, errors.New("failed to read system hosts").Base(err)
		}
		config.StaticHosts = append(config.StaticHosts, systemHosts...)
	}

	return config, nil
}

func resolveQueryStrategy(queryStrategy string) dns.QueryStrategy {
	switch strings.ToLower(queryStrategy) {
	case "useip", "use_ip", "use-ip":
		return dns.QueryStrategy_USE_IP
	case "useip4", "useipv4", "use_ip4", "use_ipv4", "use_ip_v4", "use-ip4", "use-ipv4", "use-ip-v4":
		return dns.QueryStrategy_USE_IP4
	case "useip6", "useipv6", "use_ip6", "use_ipv6", "use_ip_v6", "use-ip6", "use-ipv6", "use-ip-v6":
		return dns.QueryStrategy_USE_IP6
	case "usesys", "usesystem", "use_sys", "use_system", "use-sys", "use-system":
		return dns.QueryStrategy_USE_SYS
	default:
		return dns.QueryStrategy_USE_IP
	}
}

func readSystemHosts() ([]*dns.Config_HostMapping, error) {
	var hostsPath string
	switch runtime.GOOS {
	case "windows":
		hostsPath = filepath.Join(os.Getenv("SystemRoot"), "System32", "drivers", "etc", "hosts")
	default:
		hostsPath = "/etc/hosts"
	}

	file, err := os.Open(hostsPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return readSystemHostsFrom(file)
}

func readSystemHostsFrom(r io.Reader) ([]*dns.Config_HostMapping, error) {
	hosts := make(map[string][][]byte, 16)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if i := strings.IndexByte(line, '#'); i >= 0 {
			// Strip inline comments before splitting the line into fields.
			line = line[0:i]
		}
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		addr := net.ParseAddress(f[0])
		if addr.Family().IsDomain() {
			continue
		}
		for i := 1; i < len(f); i++ {
			domain := strings.TrimSuffix(f[i], ".")
			domain = strings.ToLower(domain)
			hosts[domain] = append(hosts[domain], addr.IP())
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	hostsMap := make([]*dns.Config_HostMapping, 0, len(hosts))
	for domain, ips := range hosts {
		// ParseDomainRule accepts rule syntax too, not just plain domains.
		rule, err := geodata.ParseDomainRule(domain, geodata.Domain_Full)
		if err != nil {
			return nil, err
		}
		hostsMap = append(hostsMap, &dns.Config_HostMapping{
			Domain: rule,
			Ip:     ips,
		})
	}

	return hostsMap, nil
}
