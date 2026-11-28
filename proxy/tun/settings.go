// Code that is not generated: the settings of this package are plain Go values,
// and they say the name a configuration written for the API calls them.

package tun

// Config is settings this package reads.
type Config struct {
	Name string `json:"name,omitempty"`
	MTU uint32 `json:"MTU,omitempty"`
	Gateway []string `json:"gateway,omitempty"`
	DNS []string `json:"DNS,omitempty"`
	UserLevel uint32 `json:"userLevel,omitempty"`
	AutoSystemRoutingTable []string `json:"autoSystemRoutingTable,omitempty"`
	AutoOutboundsInterface string `json:"autoOutboundsInterface,omitempty"`
	Desc string `json:"desc,omitempty"`
	AutoSystemDnsToGateway bool `json:"autoSystemDnsToGateway,omitempty"`
	AutoSystemWfpBlockLeak []string `json:"autoSystemWfpBlockLeak,omitempty"`
	GUID string `json:"GUID,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Config) SettingsName() string {
	return "xray.proxy.tun.Config"
}
