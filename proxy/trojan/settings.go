// Code that is not generated: the settings of this package are plain Go values,
// and they say the name a configuration written for the API calls them.

package trojan

import (
	protocol "github.com/xtls/xray-core/common/protocol"
)

// Account is settings this package reads.
type Account struct {
	Password string `json:"password,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Account) SettingsName() string {
	return "xray.proxy.trojan.Account"
}

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
	return "xray.proxy.trojan.Fallback"
}

// ClientConfig is settings this package reads.
type ClientConfig struct {
	Server *protocol.ServerEndpoint `json:"server,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*ClientConfig) SettingsName() string {
	return "xray.proxy.trojan.ClientConfig"
}

// ServerConfig is settings this package reads.
type ServerConfig struct {
	Users []*protocol.User `json:"users,omitempty"`
	Fallbacks []*Fallback `json:"fallbacks,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*ServerConfig) SettingsName() string {
	return "xray.proxy.trojan.ServerConfig"
}
