// Code that is not generated: the settings of this package are plain Go values,
// and they say the name a configuration written for the API calls them.

package observatory

// Intensity is how often an outbound is probed.
type Intensity struct {
	// The time interval for a probe request in ms.
	ProbeInterval uint32 `json:"probeInterval,omitempty"`
}

// Config is the settings of the observatory.
type Config struct {
	// The selectors for outbound under observation
	SubjectSelector   []string `json:"subjectSelector,omitempty"`
	ProbeUrl          string   `json:"probeUrl,omitempty"`
	ProbeInterval     int64    `json:"probeInterval,omitempty"`
	EnableConcurrency bool     `json:"enableConcurrency,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Config) SettingsName() string {
	return "xray.core.app.observatory.Config"
}

func (*Intensity) SettingsName() string {
	return "xray.core.app.observatory.Intensity"
}
