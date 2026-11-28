// Code that is not generated: the settings of this package are plain Go values,
// and they say the name a configuration written for the API calls them.

package geodata

// Asset is settings this package reads.
type Asset struct {
	Url string `json:"url,omitempty"`
	File string `json:"file,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Asset) SettingsName() string {
	return "xray.app.geodata.Asset"
}

// Config is settings this package reads.
type Config struct {
	Cron string `json:"cron,omitempty"`
	Outbound string `json:"outbound,omitempty"`
	Assets []*Asset `json:"assets,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Config) SettingsName() string {
	return "xray.app.geodata.Config"
}
