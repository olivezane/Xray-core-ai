// Code that is not generated: the settings of this package are plain Go values,
// and they say the name a configuration written for the API calls them.

package inbound

import (
	protocol "github.com/xtls/xray-core/common/protocol"
)

// DetourConfig is settings this package reads.
type DetourConfig struct {
	To string `json:"to,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*DetourConfig) SettingsName() string {
	return "xray.proxy.vmess.inbound.DetourConfig"
}

// DefaultConfig is settings this package reads.
type DefaultConfig struct {
	Level uint32 `json:"level,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*DefaultConfig) SettingsName() string {
	return "xray.proxy.vmess.inbound.DefaultConfig"
}

// Config is settings this package reads.
type Config struct {
	User []*protocol.User `json:"user,omitempty"`
	Default *DefaultConfig `json:"default,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Config) SettingsName() string {
	return "xray.proxy.vmess.inbound.Config"
}
