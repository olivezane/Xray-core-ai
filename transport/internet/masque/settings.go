// Code that is not generated: the settings of this package are plain Go values,
// and they say the name a configuration written for the API calls them.

package masque

import (
	splithttp "github.com/xtls/xray-core/transport/internet/splithttp"
)

// Config is settings this package reads.
type Config struct {
	Host string `json:"host,omitempty"`
	Path string `json:"path,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Warp *Warp `json:"warp,omitempty"`
	Xmux *splithttp.XmuxConfig `json:"xmux,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Config) SettingsName() string {
	return "xray.transport.internet.masque.Config"
}

// Warp is settings this package reads.
type Warp struct {
	PrivateKey []byte `json:"privateKey,omitempty"`
	PublicKey []byte `json:"publicKey,omitempty"`
	Address []string `json:"address,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Warp) SettingsName() string {
	return "xray.transport.internet.masque.Warp"
}
