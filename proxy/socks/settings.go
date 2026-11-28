// Code that is not generated: the settings of this package are plain Go values,
// and they say the name a configuration written for the API calls them.

package socks

import (
	"fmt"
	net "github.com/xtls/xray-core/common/net"
	protocol "github.com/xtls/xray-core/common/protocol"
)

// AuthType is one of the values these settings take.
type AuthType int32

const (
	AuthType_NO_AUTH AuthType = 0
	AuthType_PASSWORD AuthType = 1
)

func (x AuthType) String() string {
	switch x {
	case AuthType_NO_AUTH:
		return "NO_AUTH"
	case AuthType_PASSWORD:
		return "PASSWORD"
	default:
		return fmt.Sprint(int32(x))
	}
}

// Account is settings this package reads.
type Account struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Account) SettingsName() string {
	return "xray.proxy.socks.Account"
}

// ServerConfig is settings this package reads.
type ServerConfig struct {
	AuthType AuthType `json:"authType,omitempty"`
	Accounts map[string]string `json:"accounts,omitempty"`
	Address *net.IPOrDomain `json:"address,omitempty"`
	UdpEnabled bool `json:"udpEnabled,omitempty"`
	UserLevel uint32 `json:"userLevel,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*ServerConfig) SettingsName() string {
	return "xray.proxy.socks.ServerConfig"
}

// ClientConfig is settings this package reads.
type ClientConfig struct {
	Server *protocol.ServerEndpoint `json:"server,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*ClientConfig) SettingsName() string {
	return "xray.proxy.socks.ClientConfig"
}
