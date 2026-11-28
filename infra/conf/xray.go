package conf

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/xtls/xray-core/app/dispatcher"
	"github.com/xtls/xray-core/app/proxyman"
	"github.com/xtls/xray-core/app/stats"
	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/geodata"
	"github.com/xtls/xray-core/common/net"
	core "github.com/xtls/xray-core/core"
	coreconfig "github.com/xtls/xray-core/core/config"
	"github.com/xtls/xray-core/proxy/freedom"
	"github.com/xtls/xray-core/proxy/masque"
	"github.com/xtls/xray-core/proxy/vless"
	"github.com/xtls/xray-core/proxy/vless/inbound"
	"github.com/xtls/xray-core/proxy/vless/outbound"
	"github.com/xtls/xray-core/transport/internet"
)

var (
	inboundConfigLoader = NewJSONConfigLoader(ConfigCreatorCache{
		"tunnel":        func() interface{} { return new(DokodemoConfig) },
		"dokodemo-door": func() interface{} { return new(DokodemoConfig) },
		"http":          func() interface{} { return new(HTTPServerConfig) },
		"shadowsocks":   func() interface{} { return new(ShadowsocksServerConfig) },
		"mixed":         func() interface{} { return new(SocksServerConfig) },
		"socks":         func() interface{} { return new(SocksServerConfig) },
		"vless":         func() interface{} { return new(VLessInboundConfig) },
		"vmess":         func() interface{} { return new(VMessInboundConfig) },
		"trojan":        func() interface{} { return new(TrojanServerConfig) },
		"wireguard":     func() interface{} { return &WireGuardConfig{IsClient: false} },
		"hysteria":      func() interface{} { return new(HysteriaServerConfig) },
		"masque":        func() interface{} { return new(MasqueServerConfig) },
		"tun":           func() interface{} { return new(TunConfig) },
	}, "protocol", "settings")

	outboundConfigLoader = NewJSONConfigLoader(ConfigCreatorCache{
		"block":       func() interface{} { return new(BlackholeConfig) },
		"blackhole":   func() interface{} { return new(BlackholeConfig) },
		"loopback":    func() interface{} { return new(LoopbackConfig) },
		"direct":      func() interface{} { return new(FreedomConfig) },
		"freedom":     func() interface{} { return new(FreedomConfig) },
		"http":        func() interface{} { return new(HTTPClientConfig) },
		"shadowsocks": func() interface{} { return new(ShadowsocksClientConfig) },
		"socks":       func() interface{} { return new(SocksClientConfig) },
		"vless":       func() interface{} { return new(VLessOutboundConfig) },
		"vmess":       func() interface{} { return new(VMessOutboundConfig) },
		"trojan":      func() interface{} { return new(TrojanClientConfig) },
		"hysteria":    func() interface{} { return new(HysteriaClientConfig) },
		"masque":      func() interface{} { return new(MasqueClientConfig) },
		"dns":         func() interface{} { return new(DNSOutboundConfig) },
		"wireguard":   func() interface{} { return &WireGuardConfig{IsClient: true} },
	}, "protocol", "settings")
)

// SniffingConfig is the "sniffing" section of the configuration model.
type SniffingConfig = coreconfig.Sniffing

// BuildSniffing converts the "sniffing" section to its message: the protocol
// names of the user become the protocol list of the message here.
func BuildSniffing(c *coreconfig.Sniffing) (*proxyman.SniffingConfig, error) {
	var protocols []string
	for _, protocol := range c.DestOverride {
		switch strings.ToLower(protocol) {
		case "http":
			protocols = append(protocols, "http")
		case "tls", "https", "ssl":
			protocols = append(protocols, "tls")
		case "quic":
			protocols = append(protocols, "quic")
		case "fakedns", "fakedns+others":
			protocols = append(protocols, "fakedns")
		default:
			return nil, errors.New("unknown protocol: ", protocol)
		}
	}

	domains, err := geodata.ParseDomainRules(c.DomainsExcluded, geodata.Domain_Substr)
	if err != nil {
		return nil, err
	}

	ips, err := geodata.ParseIPRules(c.IPsExcluded)
	if err != nil {
		return nil, err
	}

	return &proxyman.SniffingConfig{
		Enabled:             c.Enabled,
		DestinationOverride: protocols,
		DomainsExcluded:     domains,
		IpsExcluded:         ips,
		MetadataOnly:        c.MetadataOnly,
		RouteOnly:           c.RouteOnly,
	}, nil
}

// InboundDetourConfig is one entry of the "inbounds" list. The data is the model
// type; the conversion to the handler config is the method below.
type InboundDetourConfig struct {
	coreconfig.Inbound
}

// Build implements Buildable.
func (c *InboundDetourConfig) Build() (*core.InboundHandlerConfig, error) {
	receiverSettings := &proxyman.ReceiverConfig{}

	// TUN inbound doesn't need port configuration as it uses network interface instead
	if strings.ToLower(c.Protocol) == "tun" {
		// Skip port validation for TUN
	} else if c.ListenOn == nil || len(c.ListenOn.String()) == 0 {
		// Listen on anyip, must set PortList
		if c.PortList == nil {
			return nil, errors.New("Listen on AnyIP but no Port(s) set in InboundDetour.")
		}
		receiverSettings.PortList = BuildPortList(c.PortList)
	} else {
		// Listen on specific IP or Unix Domain Socket
		receiverSettings.Listen = BuildAddress(c.ListenOn)
		listenDS := c.ListenOn.Family().IsDomain() && (filepath.IsAbs(c.ListenOn.Domain()) || c.ListenOn.Domain()[0] == '@')
		listenIP := c.ListenOn.Family().IsIP() || (c.ListenOn.Family().IsDomain() && c.ListenOn.Domain() == "localhost")
		if listenIP {
			// Listen on specific IP, must set PortList
			if c.PortList == nil {
				return nil, errors.New("Listen on specific ip without port in InboundDetour.")
			}
			// Listen on IP:Port
			receiverSettings.PortList = BuildPortList(c.PortList)
		} else if listenDS {
			if c.PortList != nil {
				// Listen on Unix Domain Socket, PortList should be nil
				receiverSettings.PortList = nil
			}
		} else {
			return nil, errors.New("unable to listen on domain address: ", c.ListenOn.Domain())
		}
	}

	if c.StreamSetting != nil {
		ss, err := (&StreamConfig{*c.StreamSetting}).Build()
		if err != nil {
			return nil, err
		}
		receiverSettings.StreamSettings = ss
		if strings.Contains(ss.SecurityType, "reality") && (receiverSettings.PortList == nil ||
			len(receiverSettings.PortList.Ports()) != 1 || receiverSettings.PortList.Ports()[0] != 443) {
			errors.LogWarning(context.Background(), `REALITY: Listening on non-443 ports will increase the likelihood of your server's IP being blocked by the GFW`)
		}
	}
	if c.SniffingConfig != nil {
		s, err := BuildSniffing(c.SniffingConfig)
		if err != nil {
			return nil, errors.New("failed to build sniffing config").Base(err)
		}
		receiverSettings.SniffingSettings = s
	}

	settings := []byte("{}")
	if c.Settings != nil {
		settings = ([]byte)(*c.Settings)
	}
	rawConfig, err := inboundConfigLoader.LoadWithID(settings, c.Protocol)
	if err != nil {
		return nil, errors.New("failed to load inbound detour config for protocol ", c.Protocol).Base(err)
	}
	if dokodemoConfig, ok := rawConfig.(*DokodemoConfig); ok {
		receiverSettings.ReceiveOriginalDestination = dokodemoConfig.FollowRedirect
	}
	ts, err := rawConfig.(Buildable).Build()
	if err != nil {
		return nil, errors.New("failed to build inbound handler for protocol ", c.Protocol).Base(err)
	}
	if _, ok := ts.(*masque.ServerConfig); !ok && receiverSettings.StreamSettings != nil && receiverSettings.StreamSettings.ProtocolName == "masque" {
		return nil, errors.New("the masque transport can only be used by the masque inbound")
	}
	if err := validateXDrive(ts, receiverSettings.StreamSettings, receiverSettings.PortList); err != nil {
		return nil, err
	}

	return &core.InboundHandlerConfig{
		Tag:              c.Tag,
		ReceiverSettings: receiverSettings,
		ProxySettings:    ts,
	}, nil
}

// OutboundDetourConfig is one entry of the "outbounds" list. The data is the
// model type; the conversion to the handler config is the method below.
type OutboundDetourConfig struct {
	coreconfig.Outbound
}

func requiresTransportSecurity(address *Address) bool {
	if address == nil || address.Address == nil {
		return true
	}
	if address.Family().IsIP() {
		ip := address.IP()
		return !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast()
	}
	domain := strings.TrimSuffix(strings.ToLower(address.Domain()), ".")
	return !geodata.GetPrivateDomainMatcher().MatchAny(domain)
}

func validateOutboundTransportSecurity(rawConfig interface{}, senderSettings *proxyman.SenderConfig) error {
	if senderSettings.StreamSettings != nil && senderSettings.StreamSettings.SecurityType != "" {
		return nil
	}

	if vlessCfg, ok := rawConfig.(*VLessOutboundConfig); ok {
		if vlessCfg.Encryption != "" && vlessCfg.Encryption != "none" {
			return nil
		}
		if requiresTransportSecurity(vlessCfg.Address) {
			return errors.New("vless without TLS or other encryption is prohibited unless the server address is a private IP or domain")
		}
	}

	if tjCfg, ok := rawConfig.(*TrojanClientConfig); ok {
		if requiresTransportSecurity(tjCfg.Servers[0].Address) {
			return errors.New("trojan without TLS is prohibited unless the server address is a private IP or domain")
		}
	}

	return nil
}

// validateXDrive allows only encrypted VLESS on xdrive, with native multiplex on every client connection.
func validateXDrive(ts any, ss *internet.StreamConfig, pl *net.PortList) error {
	if ss == nil || ss.ProtocolName != "xdrive" {
		return nil
	}
	switch c := ts.(type) {
	case *outbound.Config:
		// An account is a plain Go value now, so it is read back by the name it
		// says for itself.
		account, ok := common.UnwrapSettings(c.Vnext.User.Account).(*vless.Account)
		if !ok {
			return errors.New("not a vless account")
		}
		if account.Flow != "" {
			return errors.New(`the xdrive transport doesn't support VLESS "flow"`)
		}
		if account.Encryption == "none" {
			return errors.New(`the xdrive transport needs VLESS "encryption", run "xray vlessenc" to get one`)
		}
		// Reverse opens its own multiplexed connection.
		if account.Reverse == nil {
			if c.Multiplex == nil {
				c.Multiplex = &vless.Multiplex{Concurrency: 128}
			}
			if c.Multiplex.XudpProxyUDP443 == "skip" {
				return errors.New(`the xdrive transport does not support "xudpProxyUDP443":"skip"`)
			}
		}
		if account.Testpre != 0 {
			return errors.New(`the xdrive transport doesn't support VLESS "testpre"`)
		}
	case *inbound.Config:
		if c.Decryption == "none" {
			return errors.New(`the xdrive transport needs VLESS "decryption", run "xray vlessenc" to get one`)
		}
		for _, user := range c.Users {
			account, ok := common.UnwrapSettings(user.Account).(*vless.Account)
			if !ok {
				return errors.New("not a vless account")
			}
			if account.Flow != "" {
				return errors.New(`the xdrive transport doesn't support VLESS "flow"`)
			}
		}
		// every port gets its own listener on the same folder, and each of them accepts the same session
		if pl != nil && len(pl.Ports()) != 1 {
			return errors.New("the xdrive transport can only listen on one port")
		}
	default:
		return errors.New("the xdrive transport can only be used by VLESS")
	}
	return nil
}

// Build implements Buildable.
func (c *OutboundDetourConfig) Build() (*core.OutboundHandlerConfig, error) {
	if c.ProxySettings != nil {
		return nil, errors.PrintRemovedFeatureError(`outbound "proxySettings"`, `"streamSettings.sockopt.dialerProxy"`)
	}

	senderSettings := &proxyman.SenderConfig{}
	switch strings.ToLower(c.TargetStrategy) {
	case "asis", "":
		senderSettings.TargetStrategy = internet.DomainStrategy_AS_IS
	case "useip":
		senderSettings.TargetStrategy = internet.DomainStrategy_USE_IP
	case "useipv4":
		senderSettings.TargetStrategy = internet.DomainStrategy_USE_IP4
	case "useipv6":
		senderSettings.TargetStrategy = internet.DomainStrategy_USE_IP6
	case "useipv4v6":
		senderSettings.TargetStrategy = internet.DomainStrategy_USE_IP46
	case "useipv6v4":
		senderSettings.TargetStrategy = internet.DomainStrategy_USE_IP64
	case "forceip":
		senderSettings.TargetStrategy = internet.DomainStrategy_FORCE_IP
	case "forceipv4":
		senderSettings.TargetStrategy = internet.DomainStrategy_FORCE_IP4
	case "forceipv6":
		senderSettings.TargetStrategy = internet.DomainStrategy_FORCE_IP6
	case "forceipv4v6":
		senderSettings.TargetStrategy = internet.DomainStrategy_FORCE_IP46
	case "forceipv6v4":
		senderSettings.TargetStrategy = internet.DomainStrategy_FORCE_IP64
	default:
		return nil, errors.New("unsupported target domain strategy: ", c.TargetStrategy)
	}

	if c.SendThrough != nil {
		address := ParseSendThough(c.SendThrough)
		// Check if CIDR exists
		if strings.Contains(*c.SendThrough, "/") {
			senderSettings.ViaCidr = strings.Split(*c.SendThrough, "/")[1]
		} else {
			if address.Family().IsDomain() {
				domain := address.Address.Domain()
				if domain != "origin" && domain != "srcip" {
					return nil, errors.New("unable to send through: " + address.String())
				}
			}
		}
		senderSettings.Via = BuildAddress(address)
	}

	if c.StreamSetting != nil {
		ss, err := (&StreamConfig{*c.StreamSetting}).Build()
		if err != nil {
			return nil, errors.New("failed to build stream settings for outbound detour").Base(err)
		}
		senderSettings.StreamSettings = ss
	}

	settings := []byte("{}")
	if c.Settings != nil {
		settings = ([]byte)(*c.Settings)
	}
	rawConfig, err := outboundConfigLoader.LoadWithID(settings, c.Protocol)
	if err != nil {
		return nil, errors.New("failed to load outbound detour config for protocol ", c.Protocol).Base(err)
	}
	if c.LegacyMux != nil {
		errors.LogWarning(context.Background(), `The legacy "mux" setting is ignored. To enable Multiplex for VLESS or VMess outbounds, use "settings.multiplex". If Multiplex is disabled, Xray will use ordinary outbound connections.`)
	}
	ts, err := rawConfig.(Buildable).Build()
	if err != nil {
		return nil, errors.New("failed to build outbound handler for protocol ", c.Protocol).Base(err)
	}

	if senderSettings.StreamSettings != nil && senderSettings.StreamSettings.ProtocolName == "splithttp" {
		if v, ok := ts.(*outbound.Config); ok && v.Multiplex != nil && v.Multiplex.Concurrency != 0 {
			errors.LogWarning(context.Background(), `ATTENTION: Using VLESS Multiplex over XHTTP XMUX`)
		}
	}
	if err := validateXDrive(ts, senderSettings.StreamSettings, nil); err != nil {
		return nil, err
	}
	if err := validateOutboundTransportSecurity(rawConfig, senderSettings); err != nil {
		return nil, err
	}

	if _, ok := ts.(*masque.ClientConfig); !ok && senderSettings.StreamSettings != nil && senderSettings.StreamSettings.ProtocolName == "masque" {
		return nil, errors.New("the masque transport can only be used by the masque outbound")
	}

	if _, ok := rawConfig.(*BlackholeConfig); ok {
		// The legacy blackhole outbound never used its transport settings.
		senderSettings.StreamSettings = nil
	} else if fc, ok := ts.(*freedom.Config); ok {
		if legacy := rawConfig.(*FreedomConfig); legacy.ProxyProtocol > 0 && legacy.ProxyProtocol <= 2 {
			if senderSettings.StreamSettings == nil {
				senderSettings.StreamSettings = &internet.StreamConfig{}
			}
			addProxyProtocolMask(senderSettings.StreamSettings, legacy.ProxyProtocol)
		}
		if senderSettings.StreamSettings != nil &&
			senderSettings.StreamSettings.SocketSettings != nil &&
			senderSettings.StreamSettings.SocketSettings.AddressPortStrategy != internet.AddressPortStrategy_None {
			return nil, errors.New(`freedom outbound does not support "sockopt.addressPortStrategy"`)
		}

		var strategy internet.DomainStrategy
		if strategy = senderSettings.TargetStrategy; strategy != internet.DomainStrategy_AS_IS {
			errors.LogWarning(context.Background(), `The "outbound.targetStrategy" setting is not supported directly by freedom and has been automatically migrated to "sockopt.domainStrategy" with no behavior change.`)
			senderSettings.TargetStrategy = internet.DomainStrategy_AS_IS
		} else if strategy = fc.DomainStrategy; strategy != internet.DomainStrategy_AS_IS {
			errors.LogWarning(context.Background(), `The "freedom.domainStrategy" setting is deprecated and will be removed. For compatibility, its value has been automatically migrated to "sockopt.domainStrategy". Please update your config before removal.`)
		}
		if strategy != internet.DomainStrategy_AS_IS {
			if senderSettings.StreamSettings == nil {
				senderSettings.StreamSettings = &internet.StreamConfig{}
			}
			if senderSettings.StreamSettings.SocketSettings == nil {
				senderSettings.StreamSettings.SocketSettings = &internet.SocketConfig{}
			}
			senderSettings.StreamSettings.SocketSettings.DomainStrategy = strategy
		}
	}

	return &core.OutboundHandlerConfig{
		SenderSettings: senderSettings,
		Tag:            c.Tag,
		ProxySettings:  ts,
	}, nil
}

// EnvConfig is the "env" section of the configuration model.
type EnvConfig = coreconfig.Env

// overrideEnv copies the variables of o into c.
func overrideEnv(c, o coreconfig.Env) {
	for key, value := range o {
		c[key] = value
	}
}

// Config is a whole configuration file. Its sections are the model types; the
// Build method below converts the whole file to the message the instance reads.
type Config struct {
	// Deprecated: global transport config moved to streamSettings, its presence is an error.
	Transport json.RawMessage `json:"transport"`

	Env             EnvConfig              `json:"env"`
	LogConfig       *LogConfig             `json:"log"`
	RouterConfig    *RouterConfig          `json:"routing"`
	DNSConfig       *DNSConfig             `json:"dns"`
	InboundConfigs  []InboundDetourConfig  `json:"inbounds"`
	OutboundConfigs []OutboundDetourConfig `json:"outbounds"`
	Policy          *PolicyConfig          `json:"policy"`
	API             *APIConfig             `json:"api"`
	// Deprecated: removed blocks, kept only to warn.
	Metrics          json.RawMessage `json:"metrics"`
	Stats            json.RawMessage `json:"stats"`
	Observatory      json.RawMessage `json:"observatory"`
	BurstObservatory json.RawMessage `json:"burstObservatory"`
	FakeDNS          json.RawMessage `json:"fakeDns"`
	Reverse          json.RawMessage `json:"reverse"`
	Version          *VersionConfig  `json:"version"`
	Geodata          *GeodataConfig  `json:"geodata"`
}

func (c *Config) findInboundTag(tag string) int {
	found := -1
	for idx, ib := range c.InboundConfigs {
		if ib.Tag == tag {
			found = idx
			break
		}
	}
	return found
}

func (c *Config) findOutboundTag(tag string) int {
	found := -1
	for idx, ob := range c.OutboundConfigs {
		if ob.Tag == tag {
			found = idx
			break
		}
	}
	return found
}

// Override method accepts another Config overrides the current attribute
func (c *Config) Override(o *Config, fn string) {
	// only process the non-deprecated members
	if len(o.Transport) > 0 || len(o.Metrics) > 0 || len(o.Stats) > 0 || len(o.Observatory) > 0 ||
		len(o.BurstObservatory) > 0 || len(o.FakeDNS) > 0 || len(o.Reverse) > 0 {
		// These blocks are not merged, and Build only warns about the merged result.
		errors.LogWarning(context.Background(), fn, ` has a removed block ("transport", "metrics", "stats", "observatory", "burstObservatory", "fakeDns" or "reverse"), it is not merged`)
	}

	if o.LogConfig != nil {
		c.LogConfig = o.LogConfig
	}
	if o.RouterConfig != nil {
		c.RouterConfig = o.RouterConfig
	}
	if o.DNSConfig != nil {
		c.DNSConfig = o.DNSConfig
	}
	if o.Env != nil {
		if c.Env == nil {
			c.Env = EnvConfig{}
		}
		overrideEnv(c.Env, o.Env)
	}
	if o.Policy != nil {
		c.Policy = o.Policy
	}
	if o.API != nil {
		c.API = o.API
	}
	if o.Version != nil {
		c.Version = o.Version
	}

	if o.Geodata != nil {
		c.Geodata = o.Geodata
	}

	// update the Inbound in slice if the only one in override config has same tag
	if len(o.InboundConfigs) > 0 {
		for i := range o.InboundConfigs {
			if idx := c.findInboundTag(o.InboundConfigs[i].Tag); idx > -1 {
				c.InboundConfigs[idx] = o.InboundConfigs[i]
				errors.LogInfo(context.Background(), "[", fn, "] updated inbound with tag: ", o.InboundConfigs[i].Tag)

			} else {
				c.InboundConfigs = append(c.InboundConfigs, o.InboundConfigs[i])
				errors.LogInfo(context.Background(), "[", fn, "] appended inbound with tag: ", o.InboundConfigs[i].Tag)
			}
		}
	}

	// update the Outbound in slice if the only one in override config has same tag
	if len(o.OutboundConfigs) > 0 {
		outboundPrepends := []OutboundDetourConfig{}
		for i := range o.OutboundConfigs {
			if idx := c.findOutboundTag(o.OutboundConfigs[i].Tag); idx > -1 {
				c.OutboundConfigs[idx] = o.OutboundConfigs[i]
				errors.LogInfo(context.Background(), "[", fn, "] updated outbound with tag: ", o.OutboundConfigs[i].Tag)
			} else {
				if strings.Contains(strings.ToLower(fn), "tail") {
					c.OutboundConfigs = append(c.OutboundConfigs, o.OutboundConfigs[i])
					errors.LogInfo(context.Background(), "[", fn, "] appended outbound with tag: ", o.OutboundConfigs[i].Tag)
				} else {
					outboundPrepends = append(outboundPrepends, o.OutboundConfigs[i])
					errors.LogInfo(context.Background(), "[", fn, "] prepend outbound with tag: ", o.OutboundConfigs[i].Tag)
				}
			}
		}
		if !strings.Contains(strings.ToLower(fn), "tail") && len(outboundPrepends) > 0 {
			c.OutboundConfigs = append(outboundPrepends, c.OutboundConfigs...)
		}
	}
}

// Build implements Buildable.
func (c *Config) Build() (*core.Config, error) {
	for key, value := range c.Env {
		if err := os.Setenv(key, value); err != nil {
			return nil, errors.New("failed to apply environment configuration").Base(err)
		}
	}

	if err := PostProcessConfigureFile(c); err != nil {
		return nil, errors.New("failed to post-process configuration file").Base(err)
	}

	config := &core.Config{
		App: []any{
			&dispatcher.Config{},
			&proxyman.OutboundConfig{},
		},
	}

	if len(c.Metrics) > 0 || len(c.Stats) > 0 {
		errors.LogWarning(context.Background(), `The "metrics" and "stats" blocks have been removed. "api" now collects statistics and serves /debug/vars + /debug/pprof.`)
		if c.API == nil {
			// Nothing exposes the counters, so the policy stats switches would only idle.
			errors.LogWarning(context.Background(), `No "api" block: statistics are not collected at all, add "api": {"listen": "127.0.0.1:10085"}`)
		}
	}
	if len(c.Observatory) > 0 || len(c.BurstObservatory) > 0 {
		errors.LogWarning(context.Background(), `The "observatory" and "burstObservatory" blocks have been removed. Move them into the "api" block.`)
	}
	if len(c.FakeDNS) > 0 {
		errors.LogWarning(context.Background(), `The "fakeDns" block has been removed. Move it into the "dns" block.`)
	}
	// The gRPC stats service and the /debug/vars endpoint both read the stats manager.
	if c.API != nil {
		config.App = append(config.App, &stats.Config{})
		apiConf, err := BuildAPI(c.API)
		if err != nil {
			return nil, errors.New("failed to build API configuration").Base(err)
		}
		config.App = append(config.App, apiConf)
		if c.API.Observatory != nil {
			r, err := BuildObservatory(c.API.Observatory)
			if err != nil {
				return nil, errors.New("failed to build observatory configuration").Base(err)
			}
			config.App = append(config.App, r)
		}
		if c.API.BurstObservatory != nil {
			r, err := BuildBurstObservatory(c.API.BurstObservatory)
			if err != nil {
				return nil, errors.New("failed to build burst observatory configuration").Base(err)
			}
			config.App = append(config.App, r)
		}
	}

	var logConf any
	if c.LogConfig != nil {
		logConf = BuildLog(c.LogConfig)
	} else {
		logConf = DefaultLogConfig()
	}
	// let logger module be the first App to start,
	// so that other modules could print log during initiating
	config.App = append([]any{logConf}, config.App...)

	if c.RouterConfig != nil {
		routerConfig, err := c.RouterConfig.Build()
		if err != nil {
			return nil, errors.New("failed to build routing configuration").Base(err)
		}
		config.App = append(config.App, routerConfig)
	}

	if c.DNSConfig != nil {
		dnsApp, err := c.DNSConfig.Build()
		if err != nil {
			return nil, errors.New("failed to build DNS configuration").Base(err)
		}
		config.App = append(config.App, dnsApp)
	}

	if c.Policy != nil {
		pc, err := BuildPolicy(c.Policy)
		if err != nil {
			return nil, errors.New("failed to build policy configuration").Base(err)
		}
		config.App = append(config.App, pc)
	}

	if c.DNSConfig != nil && c.DNSConfig.FakeDNS != nil {
		r, err := BuildFakeDNS(c.DNSConfig.FakeDNS)
		if err != nil {
			return nil, errors.New("failed to build fake DNS configuration").Base(err)
		}
		config.App = append([]any{r}, config.App...)
	}

	if c.Version != nil {
		r, err := BuildVersion(c.Version)
		if err != nil {
			return nil, errors.New("failed to build version configuration").Base(err)
		}
		config.App = append(config.App, r)
	}

	if c.Geodata != nil {
		r, err := c.Geodata.Build()
		if err != nil {
			return nil, errors.New("failed to build geodata configuration").Base(err)
		}
		config.App = append(config.App, r)
	}

	// Initialize the inbound module last to ensure inbound connections are handled correctly.
	config.App = append(config.App, &proxyman.InboundConfig{})

	var inbounds []InboundDetourConfig

	if len(c.InboundConfigs) > 0 {
		inbounds = append(inbounds, c.InboundConfigs...)
	}

	if len(c.Transport) > 0 {
		return nil, errors.PrintRemovedFeatureError("Global transport config", "streamSettings in inbounds and outbounds")
	}

	if len(c.Reverse) > 0 {
		return nil, errors.PrintRemovedFeatureError(`"legacy reverse"`, `"VLESS Reverse Proxy"`)
	}

	for _, rawInboundConfig := range inbounds {
		ic, err := rawInboundConfig.Build()
		if err != nil {
			return nil, errors.New("failed to build inbound config with tag ", rawInboundConfig.Tag).Base(err)
		}
		config.Inbound = append(config.Inbound, ic)
	}

	var outbounds []OutboundDetourConfig

	if len(c.OutboundConfigs) > 0 {
		outbounds = append(outbounds, c.OutboundConfigs...)
	}

	for _, rawOutboundConfig := range outbounds {
		oc, err := rawOutboundConfig.Build()
		if err != nil {
			return nil, errors.New("failed to build outbound config with tag ", rawOutboundConfig.Tag).Base(err)
		}
		config.Outbound = append(config.Outbound, oc)
	}

	return config, nil
}

// Convert string to Address.
func ParseSendThough(Addr *string) *Address {
	var addr Address
	addr.Address = net.ParseAddress(strings.Split(*Addr, "/")[0])
	return &addr
}
