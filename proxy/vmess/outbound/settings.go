// Code that is not generated: the settings of this package are plain Go values,
// and they say the name a configuration written for the API calls them.

package outbound

import (
	protocol "github.com/xtls/xray-core/common/protocol"
	vless "github.com/xtls/xray-core/proxy/vless"
)

// Config is settings this package reads.
type Config struct {
	Receiver *protocol.ServerEndpoint `json:"Receiver,omitempty"`
	Multiplex *vless.Multiplex `json:"multiplex,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Config) SettingsName() string {
	return "xray.proxy.vmess.outbound.Config"
}
