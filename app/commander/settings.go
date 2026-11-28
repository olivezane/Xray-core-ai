// Code that is not generated: the settings of this package are plain Go values,
// and they say the name a configuration written for the API calls them.

package commander

import (
	serial "github.com/xtls/xray-core/common/serial"
)

// Config is settings this package reads.
type Config struct {
	Tag string `json:"tag,omitempty"`
	Listen string `json:"listen,omitempty"`
	Service []*serial.TypedMessage `json:"service,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Config) SettingsName() string {
	return "xray.app.commander.Config"
}

// ReflectionConfig is settings this package reads.
type ReflectionConfig struct {
}

// SettingsName is what a configuration written for the API calls these settings.
func (*ReflectionConfig) SettingsName() string {
	return "xray.app.commander.ReflectionConfig"
}
