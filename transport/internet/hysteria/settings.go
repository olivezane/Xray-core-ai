// Code that is not generated: the settings of this package are plain Go values,
// and they say the name a configuration written for the API calls them.

package hysteria

// Config is settings this package reads.
type Config struct {
	Auth string `json:"auth,omitempty"`
	UdpIdleTimeout int64 `json:"udpIdleTimeout,omitempty"`
	MasqType string `json:"masqType,omitempty"`
	MasqFile string `json:"masqFile,omitempty"`
	MasqUrl string `json:"masqUrl,omitempty"`
	MasqUrlRewriteHost bool `json:"masqUrlRewriteHost,omitempty"`
	MasqUrlXForwarded bool `json:"masqUrlXForwarded,omitempty"`
	MasqUrlInsecure bool `json:"masqUrlInsecure,omitempty"`
	MasqString string `json:"masqString,omitempty"`
	MasqStringHeaders map[string]string `json:"masqStringHeaders,omitempty"`
	MasqStringStatusCode int32 `json:"masqStringStatusCode,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Config) SettingsName() string {
	return "xray.transport.internet.hysteria.Config"
}
