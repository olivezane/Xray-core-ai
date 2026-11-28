// Code that is not generated: the settings of this package are plain Go values,
// and they say the name a configuration written for the API calls them.

package shadowsocks

import (
	"fmt"
	net "github.com/xtls/xray-core/common/net"
	protocol "github.com/xtls/xray-core/common/protocol"
)

// CipherType is one of the values these settings take.
type CipherType int32

const (
	CipherType_UNKNOWN CipherType = 0
	CipherType_AES_128_GCM CipherType = 5
	CipherType_AES_256_GCM CipherType = 6
	CipherType_CHACHA20_POLY1305 CipherType = 7
	CipherType_XCHACHA20_POLY1305 CipherType = 8
)

func (x CipherType) String() string {
	switch x {
	case CipherType_UNKNOWN:
		return "UNKNOWN"
	case CipherType_AES_128_GCM:
		return "AES_128_GCM"
	case CipherType_AES_256_GCM:
		return "AES_256_GCM"
	case CipherType_CHACHA20_POLY1305:
		return "CHACHA20_POLY1305"
	case CipherType_XCHACHA20_POLY1305:
		return "XCHACHA20_POLY1305"
	default:
		return fmt.Sprint(int32(x))
	}
}

// Account is settings this package reads.
type Account struct {
	Password string `json:"password,omitempty"`
	CipherType CipherType `json:"cipherType,omitempty"`
	IvCheck bool `json:"ivCheck,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Account) SettingsName() string {
	return "xray.proxy.shadowsocks.Account"
}

// ServerConfig is settings this package reads.
type ServerConfig struct {
	Users []*protocol.User `json:"users,omitempty"`
	Network []net.Network `json:"network,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*ServerConfig) SettingsName() string {
	return "xray.proxy.shadowsocks.ServerConfig"
}

// ClientConfig is settings this package reads.
type ClientConfig struct {
	Server *protocol.ServerEndpoint `json:"server,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*ClientConfig) SettingsName() string {
	return "xray.proxy.shadowsocks.ClientConfig"
}
