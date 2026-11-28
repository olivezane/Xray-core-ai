// Code that is not generated: the settings of this package are plain Go values,
// and they say the name a configuration written for the API calls them.

package proxyman

import (
	"github.com/xtls/xray-core/common/geodata"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/transport/internet"
)

// InboundConfig is settings this package reads.
type InboundConfig struct {
}

// SettingsName is what a configuration written for the API calls these settings.
func (*InboundConfig) SettingsName() string {
	return "xray.app.proxyman.InboundConfig"
}

// SniffingConfig is settings this package reads.
type SniffingConfig struct {
	Enabled bool `json:"enabled,omitempty"`
	DestinationOverride []string `json:"destinationOverride,omitempty"`
	DomainsExcluded []*geodata.DomainRule `json:"domainsExcluded,omitempty"`
	IpsExcluded []*geodata.IPRule `json:"ipsExcluded,omitempty"`
	MetadataOnly bool `json:"metadataOnly,omitempty"`
	RouteOnly bool `json:"routeOnly,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*SniffingConfig) SettingsName() string {
	return "xray.app.proxyman.SniffingConfig"
}

// ReceiverConfig is settings this package reads.
type ReceiverConfig struct {
	PortList *net.PortList `json:"portList,omitempty"`
	Listen *net.IPOrDomain `json:"listen,omitempty"`
	StreamSettings *internet.StreamConfig `json:"streamSettings,omitempty"`
	ReceiveOriginalDestination bool `json:"receiveOriginalDestination,omitempty"`
	SniffingSettings *SniffingConfig `json:"sniffingSettings,omitempty"`
}

// GetSniffingSettings is the sniffing of this receiver, never nil.
func (x *ReceiverConfig) GetSniffingSettings() *SniffingConfig {
	if x == nil {
		return nil
	}
	return x.SniffingSettings
}

// GetEnabled reports whether sniffing is on.
func (x *SniffingConfig) GetEnabled() bool {
	if x == nil {
		return false
	}
	return x.Enabled
}

// GetDestinationOverride is the protocols sniffing may override the target with.
func (x *SniffingConfig) GetDestinationOverride() []string {
	if x == nil {
		return nil
	}
	return x.DestinationOverride
}

// SettingsName is what a configuration written for the API calls these settings.
func (*ReceiverConfig) SettingsName() string {
	return "xray.app.proxyman.ReceiverConfig"
}

// OutboundConfig is settings this package reads.
type OutboundConfig struct {
}

// SettingsName is what a configuration written for the API calls these settings.
func (*OutboundConfig) SettingsName() string {
	return "xray.app.proxyman.OutboundConfig"
}

// SenderConfig is settings this package reads.
type SenderConfig struct {
	Via *net.IPOrDomain `json:"via,omitempty"`
	StreamSettings *internet.StreamConfig `json:"streamSettings,omitempty"`
	ViaCidr string `json:"viaCidr,omitempty"`
	TargetStrategy internet.DomainStrategy `json:"targetStrategy,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*SenderConfig) SettingsName() string {
	return "xray.app.proxyman.SenderConfig"
}
