// Code that is not generated: the settings of this package are plain Go values,
// and they say the name a configuration written for the API calls them.

package hysteria

import (
	protocol "github.com/xtls/xray-core/common/protocol"
)

// ClientConfig is settings this package reads.
type ClientConfig struct {
	Server *protocol.ServerEndpoint `json:"server,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*ClientConfig) SettingsName() string {
	return "xray.proxy.hysteria.ClientConfig"
}

// ServerConfig is settings this package reads.
type ServerConfig struct {
	Users []*protocol.User `json:"users,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*ServerConfig) SettingsName() string {
	return "xray.proxy.hysteria.ServerConfig"
}
