package version

// Config is the settings of the version service: which core version a
// configuration was written for.
type Config struct {
	CoreVersion string `json:"core_version,omitempty"`
	MinVersion  string `json:"min_version,omitempty"`
	MaxVersion  string `json:"max_version,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Config) SettingsName() string {
	return "xray.app.version.Config"
}
