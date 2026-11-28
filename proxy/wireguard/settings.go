// Code that is not generated: the settings of this package are plain Go values,
// and they say the name a configuration written for the API calls them.

package wireguard

import (
	protocol "github.com/xtls/xray-core/common/protocol"
)

// PeerConfig is settings this package reads.
type PeerConfig struct {
	PublicKey string `json:"publicKey,omitempty"`
	PreSharedKey string `json:"preSharedKey,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`
	KeepAlive string `json:"keepAlive,omitempty"`
	AllowedIps []string `json:"allowedIps,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*PeerConfig) SettingsName() string {
	return "xray.proxy.wireguard.PeerConfig"
}

// DeviceConfig is settings this package reads.
type DeviceConfig struct {
	SecretKey string `json:"secretKey,omitempty"`
	Endpoint []string `json:"endpoint,omitempty"`
	Peers []*PeerConfig `json:"peers,omitempty"`
	Users []*protocol.User `json:"users,omitempty"`
	Mtu int32 `json:"mtu,omitempty"`
	Reserved []byte `json:"reserved,omitempty"`
	IsClient bool `json:"isClient,omitempty"`
	NoKernelTun bool `json:"noKernelTun,omitempty"`
	DNS []string `json:"DNS,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*DeviceConfig) SettingsName() string {
	return "xray.proxy.wireguard.DeviceConfig"
}
