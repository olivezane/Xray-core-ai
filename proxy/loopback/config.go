package loopback

import "github.com/xtls/xray-core/app/proxyman"

// Config is the settings of a loopback inbound: it hands every connection back to
// the dispatcher, on the inbound it names.
type Config struct {
	InboundTag string                  `json:"inbound_tag,omitempty"`
	Sniffing   *proxyman.SniffingConfig `json:"sniffing,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Config) SettingsName() string {
	return "xray.proxy.loopback.Config"
}
