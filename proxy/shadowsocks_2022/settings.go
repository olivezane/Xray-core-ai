// Code that is not generated: the settings of this package are plain Go values,
// and they say the name a configuration written for the API calls them.

package shadowsocks_2022

import (
	net "github.com/xtls/xray-core/common/net"
	protocol "github.com/xtls/xray-core/common/protocol"
)

// ServerConfig is settings this package reads.
type ServerConfig struct {
	Method string `json:"method,omitempty"`
	Key string `json:"key,omitempty"`
	Email string `json:"email,omitempty"`
	Level int32 `json:"level,omitempty"`
	Network []net.Network `json:"network,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*ServerConfig) SettingsName() string {
	return "xray.proxy.shadowsocks_2022.ServerConfig"
}

// MultiUserServerConfig is settings this package reads.
type MultiUserServerConfig struct {
	Method string `json:"method,omitempty"`
	Key string `json:"key,omitempty"`
	Users []*protocol.User `json:"users,omitempty"`
	Network []net.Network `json:"network,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*MultiUserServerConfig) SettingsName() string {
	return "xray.proxy.shadowsocks_2022.MultiUserServerConfig"
}

// RelayDestination is settings this package reads.
type RelayDestination struct {
	Key string `json:"key,omitempty"`
	Address *net.IPOrDomain `json:"address,omitempty"`
	Port uint32 `json:"port,omitempty"`
	Email string `json:"email,omitempty"`
	Level int32 `json:"level,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*RelayDestination) SettingsName() string {
	return "xray.proxy.shadowsocks_2022.RelayDestination"
}

// RelayServerConfig is settings this package reads.
type RelayServerConfig struct {
	Method string `json:"method,omitempty"`
	Key string `json:"key,omitempty"`
	Destinations []*RelayDestination `json:"destinations,omitempty"`
	Network []net.Network `json:"network,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*RelayServerConfig) SettingsName() string {
	return "xray.proxy.shadowsocks_2022.RelayServerConfig"
}

// Account is settings this package reads.
type Account struct {
	Key string `json:"key,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Account) SettingsName() string {
	return "xray.proxy.shadowsocks_2022.Account"
}

// ClientConfig is settings this package reads.
type ClientConfig struct {
	Address *net.IPOrDomain `json:"address,omitempty"`
	Port uint32 `json:"port,omitempty"`
	Method string `json:"method,omitempty"`
	Key string `json:"key,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*ClientConfig) SettingsName() string {
	return "xray.proxy.shadowsocks_2022.ClientConfig"
}
