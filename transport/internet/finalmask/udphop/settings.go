// Code that is not generated: the settings of this package are plain Go values,
// and they say the name a configuration written for the API calls them.

package udphop

// Config is settings this package reads.
type Config struct {
	Local bool `json:"local,omitempty"`
	Remote bool `json:"remote,omitempty"`
	RemoteOnce bool `json:"remoteOnce,omitempty"`
	IntervalMin int64 `json:"intervalMin,omitempty"`
	IntervalMax int64 `json:"intervalMax,omitempty"`
	RemoteIPs []string `json:"remoteIPs,omitempty"`
	RemotePorts []uint32 `json:"remotePorts,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Config) SettingsName() string {
	return "xray.transport.internet.finalmask.udphop.Config"
}
