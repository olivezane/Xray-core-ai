// Code that is not generated: the settings of this package are plain Go values,
// and they say the name a configuration written for the API calls them.

package websocket

// Config is settings this package reads.
type Config struct {
	Host string `json:"host,omitempty"`
	Path string `json:"path,omitempty"`
	Header map[string]string `json:"header,omitempty"`
	AcceptProxyProtocol bool `json:"acceptProxyProtocol,omitempty"`
	Ed uint32 `json:"ed,omitempty"`
	HeartbeatPeriod uint32 `json:"heartbeatPeriod,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Config) SettingsName() string {
	return "xray.transport.internet.websocket.Config"
}
