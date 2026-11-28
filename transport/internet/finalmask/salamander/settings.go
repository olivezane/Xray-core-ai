// Code that is not generated: the settings of this package are plain Go values,
// and they say the name a configuration written for the API calls them.

package salamander

// Config is settings this package reads.
type Config struct {
	Password string `json:"password,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Config) SettingsName() string {
	return "xray.transport.internet.finalmask.salamander.Config"
}

// GeckoConfig is settings this package reads.
type GeckoConfig struct {
	Password string `json:"password,omitempty"`
	MinPacketSize int32 `json:"MinPacketSize,omitempty"`
	MaxPacketSize int32 `json:"MaxPacketSize,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*GeckoConfig) SettingsName() string {
	return "xray.transport.internet.finalmask.salamander.GeckoConfig"
}
