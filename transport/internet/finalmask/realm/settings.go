// Code that is not generated: the settings of this package are plain Go values,
// and they say the name a configuration written for the API calls them.

package realm

import (
	"fmt"
	tls "github.com/xtls/xray-core/transport/internet/tls"
)

// Family is one of the values these settings take.
type Family int32

const (
	Family_Dual Family = 0
	Family_V4 Family = 1
	Family_V6 Family = 2
)

func (x Family) String() string {
	switch x {
	case Family_Dual:
		return "Dual"
	case Family_V4:
		return "V4"
	case Family_V6:
		return "V6"
	default:
		return fmt.Sprint(int32(x))
	}
}

// PortMapping is settings this package reads.
type PortMapping struct {
	Enabled bool `json:"enabled,omitempty"`
	Timeout int64 `json:"timeout,omitempty"`
	Lifetime int64 `json:"lifetime,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*PortMapping) SettingsName() string {
	return "xray.transport.internet.finalmask.realm.PortMapping"
}

// Config is settings this package reads.
type Config struct {
	Scheme string `json:"scheme,omitempty"`
	Host string `json:"host,omitempty"`
	Port string `json:"port,omitempty"`
	Token string `json:"token,omitempty"`
	ID string `json:"ID,omitempty"`
	StunServers []string `json:"stunServers,omitempty"`
	TlsConfig *tls.Config `json:"tlsConfig,omitempty"`
	IPMode string `json:"IPMode,omitempty"`
	PortMapping *PortMapping `json:"portMapping,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Config) SettingsName() string {
	return "xray.transport.internet.finalmask.realm.Config"
}
