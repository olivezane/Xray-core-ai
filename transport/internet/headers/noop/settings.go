// Code that is not generated: the settings of this package are plain Go values,
// and they say the name a configuration written for the API calls them.

package noop

// Config is settings this package reads.
type Config struct {
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Config) SettingsName() string {
	return "xray.transport.internet.headers.noop.Config"
}

// ConnectionConfig is settings this package reads.
type ConnectionConfig struct {
}

// SettingsName is what a configuration written for the API calls these settings.
func (*ConnectionConfig) SettingsName() string {
	return "xray.transport.internet.headers.noop.ConnectionConfig"
}
