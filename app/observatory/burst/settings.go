// Code that is not generated: the settings of this package are plain Go values,
// and they say the name a configuration written for the API calls them.

package burst

// Config is settings this package reads.
type Config struct {
	SubjectSelector []string `json:"subjectSelector,omitempty"`
	PingConfig *HealthPingConfig `json:"pingConfig,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Config) SettingsName() string {
	return "xray.core.app.observatory.burst.Config"
}

// HealthPingConfig is settings this package reads.
type HealthPingConfig struct {
	Destination string `json:"destination,omitempty"`
	Connectivity string `json:"connectivity,omitempty"`
	Interval int64 `json:"interval,omitempty"`
	SamplingCount int32 `json:"samplingCount,omitempty"`
	Timeout int64 `json:"timeout,omitempty"`
	HttpMethod string `json:"httpMethod,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*HealthPingConfig) SettingsName() string {
	return "xray.core.app.observatory.burst.HealthPingConfig"
}
