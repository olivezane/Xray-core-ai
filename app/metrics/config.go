package metrics

// Config is the settings of the metrics service.
type Config struct {
	// Tag of the outbound handler that handles metrics http connections.
	Tag    string `json:"tag,omitempty"`
	Listen string `json:"listen,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Config) SettingsName() string {
	return "xray.app.metrics.Config"
}
