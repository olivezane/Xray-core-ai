package all

import (
	// The following are necessary as they register handlers in their init functions.

	// Mandatory features. Can't remove unless there are replacements.
	_ "github.com/xtls/xray-core/app/dispatcher"
	_ "github.com/xtls/xray-core/app/proxyman/inbound"
	_ "github.com/xtls/xray-core/app/proxyman/outbound"

	// Default commander and all its services. This is an optional feature.
	_ "github.com/xtls/xray-core/app/commander"
	_ "github.com/xtls/xray-core/app/log/command"
	_ "github.com/xtls/xray-core/app/proxyman/command"
	_ "github.com/xtls/xray-core/app/stats/command"

	// Developer preview services
	_ "github.com/xtls/xray-core/app/observatory/command"

	// Other optional features.
	_ "github.com/xtls/xray-core/app/dns"
	_ "github.com/xtls/xray-core/app/dns/fakedns"
	_ "github.com/xtls/xray-core/app/geodata"
	_ "github.com/xtls/xray-core/app/log"
	_ "github.com/xtls/xray-core/app/metrics"
	_ "github.com/xtls/xray-core/app/policy"
	_ "github.com/xtls/xray-core/app/router"
	_ "github.com/xtls/xray-core/app/stats"

	// Fix dependency cycle caused by core import in internet package. Wire
	// installs it, because it cannot install itself without an init().
	"github.com/xtls/xray-core/transport/internet/tagged/taggedimpl"

	// Developer preview features
	_ "github.com/xtls/xray-core/app/observatory"

	// Inbound and outbound proxies.
	_ "github.com/xtls/xray-core/proxy/dns"
	_ "github.com/xtls/xray-core/proxy/dokodemo"
	_ "github.com/xtls/xray-core/proxy/freedom"
	_ "github.com/xtls/xray-core/proxy/http"
	_ "github.com/xtls/xray-core/proxy/loopback"
	_ "github.com/xtls/xray-core/proxy/masque"
	_ "github.com/xtls/xray-core/proxy/shadowsocks"
	_ "github.com/xtls/xray-core/proxy/socks"
	_ "github.com/xtls/xray-core/proxy/trojan"
	_ "github.com/xtls/xray-core/proxy/vless/inbound"
	_ "github.com/xtls/xray-core/proxy/vless/outbound"
	_ "github.com/xtls/xray-core/proxy/vmess/inbound"
	_ "github.com/xtls/xray-core/proxy/vmess/outbound"
	_ "github.com/xtls/xray-core/proxy/wireguard"

	// Transports. The list itself is transport/internet/transports.
	_ "github.com/xtls/xray-core/transport/internet/transports"
	_ "github.com/xtls/xray-core/transport/internet/reality"
	_ "github.com/xtls/xray-core/transport/internet/tls"

	// Transport headers. The list itself is transport/internet/headers.
	_ "github.com/xtls/xray-core/transport/internet/headers"

	// JSON & TOML & YAML
	_ "github.com/xtls/xray-core/main/json"
	_ "github.com/xtls/xray-core/main/toml"
	_ "github.com/xtls/xray-core/main/yaml"

	// Load config from file or http(s)
	_ "github.com/xtls/xray-core/main/confloader/external"

	// Commands
	_ "github.com/xtls/xray-core/main/commands/all"
)

// Wire installs the parts of this build that a package cannot install for
// itself: they need core, and core cannot import them. Everything else in this
// file is a plain import, so this function is the whole runtime wiring.
func Wire() {
	taggedimpl.Install()
}
