// Code that is not generated: the settings of this package are plain Go values,
// and they say the name a configuration written for the API calls them.

package dokodemo

import (
	net "github.com/xtls/xray-core/common/net"
)

// Config is settings this package reads.
type Config struct {
	AllowedNetworks []net.Network `json:"allowedNetworks,omitempty"`
	RewriteAddress *net.IPOrDomain `json:"rewriteAddress,omitempty"`
	RewritePort uint32 `json:"rewritePort,omitempty"`
	PortMap map[string]string `json:"portMap,omitempty"`
	FollowRedirect bool `json:"followRedirect,omitempty"`
	UserLevel uint32 `json:"userLevel,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Config) SettingsName() string {
	return "xray.proxy.dokodemo.Config"
}
