// Code that is not generated: the settings of this package are plain Go values,
// and they say the name a configuration written for the API calls them.

package masque

import (
	protocol "github.com/xtls/xray-core/common/protocol"
)

// ClientConfig is settings this package reads.
type ClientConfig struct {
	Server *protocol.ServerEndpoint `json:"server,omitempty"`
	RemoteDns []string `json:"remoteDns,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*ClientConfig) SettingsName() string {
	return "xray.proxy.masque.ClientConfig"
}

// Account is settings this package reads.
type Account struct {
	Password string `json:"password,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Account) SettingsName() string {
	return "xray.proxy.masque.Account"
}

// ServerConfig is settings this package reads.
type ServerConfig struct {
	Users []*protocol.User `json:"users,omitempty"`
	Address []string `json:"address,omitempty"`
	Mtu uint32 `json:"mtu,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*ServerConfig) SettingsName() string {
	return "xray.proxy.masque.ServerConfig"
}
