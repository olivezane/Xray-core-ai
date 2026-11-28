// Code that is not generated: the settings of this package are plain Go values,
// and they say the name a configuration written for the API calls them.

package grpc

// Config is settings this package reads.
type Config struct {
	Authority string `json:"authority,omitempty"`
	ServiceName string `json:"serviceName,omitempty"`
	MultiMode bool `json:"multiMode,omitempty"`
	IdleTimeout int32 `json:"idleTimeout,omitempty"`
	HealthCheckTimeout int32 `json:"healthCheckTimeout,omitempty"`
	PermitWithoutStream bool `json:"permitWithoutStream,omitempty"`
	InitialWindowsSize int32 `json:"initialWindowsSize,omitempty"`
	UserAgent string `json:"userAgent,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Config) SettingsName() string {
	return "xray.transport.internet.grpc.encoding.Config"
}
