package all

import (
	"github.com/xtls/xray-core/common"

	// The table below lists what each of these packages creates from a
	// configuration. A package is imported here because this file is the one place
	// that decides what a build contains; nothing registers itself.

	// Mandatory features. Can't remove unless there are replacements.
	appdispatcher "github.com/xtls/xray-core/app/dispatcher"
	appproxymaninbound "github.com/xtls/xray-core/app/proxyman/inbound"
	appproxymanoutbound "github.com/xtls/xray-core/app/proxyman/outbound"

	// Default commander and all its services. This is an optional feature.
	appcommander "github.com/xtls/xray-core/app/commander"
	applogcommand "github.com/xtls/xray-core/app/log/command"
	appproxymancommand "github.com/xtls/xray-core/app/proxyman/command"
	appstatscommand "github.com/xtls/xray-core/app/stats/command"
	approutercommand "github.com/xtls/xray-core/app/router/command"

	// Developer preview services
	appobservatorycommand "github.com/xtls/xray-core/app/observatory/command"

	// Other optional features.
	appdns "github.com/xtls/xray-core/app/dns"
	appfakedns "github.com/xtls/xray-core/app/dns/fakedns"
	appgeodata "github.com/xtls/xray-core/app/geodata"
	applog "github.com/xtls/xray-core/app/log"
	appmetrics "github.com/xtls/xray-core/app/metrics"
	apppolicy "github.com/xtls/xray-core/app/policy"
	approuter "github.com/xtls/xray-core/app/router"
	appstats "github.com/xtls/xray-core/app/stats"
	appversion "github.com/xtls/xray-core/app/version"

	// Developer preview features
	appobservatory "github.com/xtls/xray-core/app/observatory"
	appobservatoryburst "github.com/xtls/xray-core/app/observatory/burst"

	// Inbound and outbound proxies.
	proxydns "github.com/xtls/xray-core/proxy/dns"
	proxydokodemo "github.com/xtls/xray-core/proxy/dokodemo"
	proxyfreedom "github.com/xtls/xray-core/proxy/freedom"
	proxyhttp "github.com/xtls/xray-core/proxy/http"
	proxyhysteria "github.com/xtls/xray-core/proxy/hysteria"
	proxyloopback "github.com/xtls/xray-core/proxy/loopback"
	proxymasque "github.com/xtls/xray-core/proxy/masque"
	proxyshadowsocks "github.com/xtls/xray-core/proxy/shadowsocks"
	proxyshadowsocks2022 "github.com/xtls/xray-core/proxy/shadowsocks_2022"
	proxysocks "github.com/xtls/xray-core/proxy/socks"
	proxytrojan "github.com/xtls/xray-core/proxy/trojan"
	proxytun "github.com/xtls/xray-core/proxy/tun"
	proxyvlessinbound "github.com/xtls/xray-core/proxy/vless/inbound"
	proxyvlessoutbound "github.com/xtls/xray-core/proxy/vless/outbound"
	proxyvmessinbound "github.com/xtls/xray-core/proxy/vmess/inbound"
	proxyvmessoutbound "github.com/xtls/xray-core/proxy/vmess/outbound"
	proxywireguard "github.com/xtls/xray-core/proxy/wireguard"

	// Transports. The list itself is transport/internet/transports.
	_ "github.com/xtls/xray-core/transport/internet/transports"
	_ "github.com/xtls/xray-core/transport/internet/reality"
	_ "github.com/xtls/xray-core/transport/internet/tls"

	// Transport headers. The list itself is transport/internet/headers.
	_ "github.com/xtls/xray-core/transport/internet/headers"

	// Fix dependency cycle caused by core import in internet package. Wire
	// installs it, because it cannot install itself without an init().
	"github.com/xtls/xray-core/transport/internet/tagged/taggedimpl"

	// JSON & TOML & YAML
	_ "github.com/xtls/xray-core/main/json"
	_ "github.com/xtls/xray-core/main/toml"
	_ "github.com/xtls/xray-core/main/yaml"

	// Load config from file or http(s)
	_ "github.com/xtls/xray-core/main/confloader/external"

	// Commands
	_ "github.com/xtls/xray-core/main/commands/all"
)

// configObjects is every configuration this build creates an object from, one
// line per package. The packages keep their creators next to the code they build;
// this list is where they are wired, so adding a feature means adding its line
// here instead of relying on an init() function.
func configObjects() []common.ConfigObject {
	var configs []common.ConfigObject
	for _, list := range [][]common.ConfigObject{
		// Mandatory features. Can't remove unless there are replacements
		appdispatcher.ConfigObjects,
		appproxymaninbound.ConfigObjects,
		appproxymanoutbound.ConfigObjects,
		// Default commander and all its services. This is an optional feature
		appcommander.ConfigObjectsCommander,
		appcommander.ConfigObjectsCommanderService,
		applogcommand.ConfigObjects,
		appproxymancommand.ConfigObjects,
		appstatscommand.ConfigObjects,
		approutercommand.ConfigObjects,
		// Developer preview services
		appobservatorycommand.ConfigObjects,
		// Other optional features
		appdns.ConfigObjects,
		appfakedns.ConfigObjects,
		appgeodata.ConfigObjects,
		applog.ConfigObjects,
		appmetrics.ConfigObjects,
		apppolicy.ConfigObjects,
		approuter.ConfigObjects,
		appstats.ConfigObjects,
		appversion.ConfigObjects,
		// Developer preview features
		appobservatory.ConfigObjects,
		appobservatoryburst.ConfigObjects,
		// Inbound and outbound proxies
		proxydns.ConfigObjects,
		proxydokodemo.ConfigObjects,
		proxyfreedom.ConfigObjects,
		proxyhttp.ConfigObjectsHttpClient,
		proxyhttp.ConfigObjectsHttpServer,
		proxyhysteria.ConfigObjectsHysteriaClient,
		proxyhysteria.ConfigObjectsHysteriaServer,
		proxyloopback.ConfigObjects,
		proxymasque.ConfigObjectsMasqueClient,
		proxymasque.ConfigObjectsMasqueServer,
		proxyshadowsocks.ConfigObjectsShadowsocksClient,
		proxyshadowsocks.ConfigObjectsShadowsocksServer,
		proxyshadowsocks2022.ConfigObjectsShadowsocks2022InboundMulti,
		proxyshadowsocks2022.ConfigObjectsShadowsocks2022InboundRelay,
		proxyshadowsocks2022.ConfigObjectsShadowsocks2022Inbound,
		proxyshadowsocks2022.ConfigObjectsShadowsocks2022Outbound,
		proxysocks.ConfigObjectsSocksClient,
		proxysocks.ConfigObjectsSocksServer,
		proxytrojan.ConfigObjectsTrojanClient,
		proxytrojan.ConfigObjectsTrojanServer,
		proxytun.ConfigObjects,
		proxyvlessinbound.ConfigObjects,
		proxyvlessoutbound.ConfigObjects,
		proxyvmessinbound.ConfigObjects,
		proxyvmessoutbound.ConfigObjects,
		proxywireguard.ConfigObjects,
	} {
		configs = append(configs, list...)
	}
	return configs
}

// Wire installs the parts of this build that a package cannot install for
// itself: the configuration table above, and the tagged dialer, which needs core
// while core cannot import it. Everything else in this file is a plain import, so
// this function is the whole runtime wiring of a build.
func Wire() {
	common.Must(common.SetConfigObjects(configObjects()))
	taggedimpl.Install()
}
