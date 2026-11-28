// Code that is not generated: the settings of this package are plain Go values,
// and they say the name a configuration written for the API calls them.

package inbound

import (
	protocol "github.com/xtls/xray-core/common/protocol"
)

// Fallback is settings this package reads.
type Fallback struct {
	Name string `json:"name,omitempty"`
	Alpn string `json:"alpn,omitempty"`
	Path string `json:"path,omitempty"`
	Type string `json:"type,omitempty"`
	Dest string `json:"dest,omitempty"`
	Xver uint64 `json:"xver,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Fallback) SettingsName() string {
	return "xray.proxy.vless.inbound.Fallback"
}

// Config is settings this package reads.
type Config struct {
	Users []*protocol.User `json:"users,omitempty"`
	Fallbacks []*Fallback `json:"fallbacks,omitempty"`
	Decryption string `json:"decryption,omitempty"`
	XorMode uint32 `json:"xorMode,omitempty"`
	SecondsFrom int64 `json:"secondsFrom,omitempty"`
	SecondsTo int64 `json:"secondsTo,omitempty"`
	Padding string `json:"padding,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Config) SettingsName() string {
	return "xray.proxy.vless.inbound.Config"
}
