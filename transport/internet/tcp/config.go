package tcp

import (
	"encoding/json"

	"github.com/xtls/xray-core/common/serial"
	"github.com/xtls/xray-core/transport/internet"
)

// Config is the settings of the tcp transport.
type Config struct {
	HeaderSettings      any                 `json:"-"`
	AcceptProxyProtocol bool                 `json:"accept_proxy_protocol,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Config) SettingsName() string {
	return "xray.transport.internet.tcp.Config"
}

// NewConfig returns the settings a stream settings entry naming this transport
// decodes to when it carries no settings for it.
func NewConfig() *Config {
	return new(Config)
}

// MarshalJSON writes the header settings with the name of their type, because a
// value on its own does not say what type it is.
func (c *Config) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		HeaderSettings      *serial.TypedMessage `json:"headerSettings,omitempty"`
		AcceptProxyProtocol bool                 `json:"acceptProxyProtocol,omitempty"`
	}{
		HeaderSettings:      serial.ToTypedMessage(c.HeaderSettings),
		AcceptProxyProtocol: c.AcceptProxyProtocol,
	})
}

// UnmarshalJSON reads the header settings as the type they name.
func (c *Config) UnmarshalJSON(b []byte) error {
	var in struct {
		HeaderSettings      *serial.TypedMessage `json:"headerSettings,omitempty"`
		AcceptProxyProtocol bool                 `json:"acceptProxyProtocol,omitempty"`
	}
	if err := json.Unmarshal(b, &in); err != nil {
		return err
	}
	c.AcceptProxyProtocol = in.AcceptProxyProtocol
	settings, err := internet.UnwrapSettings(in.HeaderSettings)
	if err != nil {
		return err
	}
	c.HeaderSettings = settings
	return nil
}
