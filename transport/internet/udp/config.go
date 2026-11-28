package udp

// Config is the settings of the udp transport. It carries nothing today; it is
// kept because a configuration names it.
type Config struct{}

// SettingsName is what a configuration written for the API calls these settings.
func (*Config) SettingsName() string {
	return "xray.transport.internet.udp.Config"
}

// NewConfig returns the settings a stream settings entry naming this transport
// decodes to when it carries no settings for it.
func NewConfig() *Config {
	return new(Config)
}
