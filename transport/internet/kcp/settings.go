// Code that is not generated: the settings of this package are plain Go values,
// and they say the name a configuration written for the API calls them.

package kcp

// Config is settings this package reads.
type Config struct {
	Mtu uint32 `json:"mtu,omitempty"`
	Tti uint32 `json:"tti,omitempty"`
	UplinkCapacity uint32 `json:"uplinkCapacity,omitempty"`
	DownlinkCapacity uint32 `json:"downlinkCapacity,omitempty"`
	CwndMultiplier uint32 `json:"cwndMultiplier,omitempty"`
	MaxSendingWindow uint32 `json:"maxSendingWindow,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Config) SettingsName() string {
	return "xray.transport.internet.kcp.Config"
}
