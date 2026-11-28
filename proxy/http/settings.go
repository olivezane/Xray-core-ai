// Code that is not generated: the settings of this package are plain Go values,
// and they say the name a configuration written for the API calls them.

package http

import (
	protocol "github.com/xtls/xray-core/common/protocol"
)

// Account is settings this package reads.
type Account struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Account) SettingsName() string {
	return "xray.proxy.http.Account"
}

// ServerConfig is settings this package reads.
type ServerConfig struct {
	Accounts map[string]string `json:"accounts,omitempty"`
	AllowTransparent bool `json:"allowTransparent,omitempty"`
	UserLevel uint32 `json:"userLevel,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*ServerConfig) SettingsName() string {
	return "xray.proxy.http.ServerConfig"
}

// Header is settings this package reads.
type Header struct {
	Key string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Header) SettingsName() string {
	return "xray.proxy.http.Header"
}

// ClientConfig is settings this package reads.
type ClientConfig struct {
	Server *protocol.ServerEndpoint `json:"server,omitempty"`
	Header []*Header `json:"header,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*ClientConfig) SettingsName() string {
	return "xray.proxy.http.ClientConfig"
}
