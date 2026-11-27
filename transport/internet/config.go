package internet

import (
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/serial"
)

type ConfigCreator func() interface{}

// ConfigOf adapts a typed constructor to a ConfigCreator, so that a transport
// keeps its own default settings in its own package while the place that wires
// transports stays a list of names and functions.
func ConfigOf[T any](newConfig func() T) ConfigCreator {
	return func() interface{} { return newConfig() }
}

var strategy = [11][3]byte{
	//              name        strategy,   prefer, fallback
	{0, 0, 0}, //   AsIs        none,       /,      /
	{1, 0, 0}, //   UseIP       use,        both,   none
	{1, 4, 0}, //   UseIPv4     use,        4,      none
	{1, 6, 0}, //   UseIPv6     use,        6,      none
	{1, 4, 6}, //   UseIPv4v6   use,        4,      6
	{1, 6, 4}, //   UseIPv6v4   use,        6,      4
	{2, 0, 0}, //   ForceIP     force,      both,   none
	{2, 4, 0}, //   ForceIPv4   force,      4,      none
	{2, 6, 0}, //   ForceIPv6   force,      6,      none
	{2, 4, 6}, //   ForceIPv4v6 force,      4,      6
	{2, 6, 4}, //   ForceIPv6v4 force,      6,      4
}

// CreateTransportConfig returns the default protocol settings of a transport,
// the ones a stream settings entry naming that protocol decodes to when it
// carries no settings of its own. The transport and its settings are wired in
// transport/internet/transports, one line each; a transport that is not listed
// there cannot be configured, which is what #3244 was about.
func CreateTransportConfig(name string) (interface{}, error) {
	config := transportFor(name).Config
	if config == nil {
		return nil, errors.New(name, " is not a stream transport of this build; transports are listed in transport/internet/transports")
	}
	return config(), nil
}

func (c *TransportConfig) GetTypedSettings() (interface{}, error) {
	return c.Settings.GetInstance()
}

func (c *TransportConfig) GetUnifiedProtocolName() string {
	return c.ProtocolName
}

func (c *StreamConfig) GetEffectiveProtocol() string {
	if c == nil || len(c.ProtocolName) == 0 {
		return "tcp"
	}

	return c.ProtocolName
}

func (c *StreamConfig) GetEffectiveTransportSettings() (interface{}, error) {
	protocol := c.GetEffectiveProtocol()
	return c.GetTransportSettingsFor(protocol)
}

func (c *StreamConfig) GetTransportSettingsFor(protocol string) (interface{}, error) {
	if c != nil {
		for _, settings := range c.TransportSettings {
			if settings.GetUnifiedProtocolName() == protocol {
				return settings.GetTypedSettings()
			}
		}
	}

	return CreateTransportConfig(protocol)
}

func (c *StreamConfig) GetEffectiveSecuritySettings() (interface{}, error) {
	for _, settings := range c.SecuritySettings {
		if settings.Type == c.SecurityType {
			return settings.GetInstance()
		}
	}
	return serial.GetInstance(c.SecurityType)
}

func (c *StreamConfig) HasSecuritySettings() bool {
	return len(c.SecuritySettings) > 0
}

func (m SocketConfig_TProxyMode) IsEnabled() bool {
	return m != SocketConfig_Off
}

func (s DomainStrategy) HasStrategy() bool {
	return strategy[s][0] != 0
}

func (s DomainStrategy) ForceIP() bool {
	return strategy[s][0] == 2
}

func (s DomainStrategy) PreferIP4() bool {
	return strategy[s][1] == 4 || strategy[s][1] == 0
}

func (s DomainStrategy) PreferIP6() bool {
	return strategy[s][1] == 6 || strategy[s][1] == 0
}

func (s DomainStrategy) HasFallback() bool {
	return strategy[s][2] != 0
}

func (s DomainStrategy) FallbackIP4() bool {
	return strategy[s][2] == 4
}

func (s DomainStrategy) FallbackIP6() bool {
	return strategy[s][2] == 6
}

func (s DomainStrategy) GetDynamicStrategy(addrFamily net.AddressFamily) DomainStrategy {
	if addrFamily.IsDomain() {
		return s
	}
	switch s {
	case DomainStrategy_USE_IP:
		if addrFamily.IsIPv4() {
			return DomainStrategy_USE_IP46
		} else if addrFamily.IsIPv6() {
			return DomainStrategy_USE_IP64
		}
	case DomainStrategy_FORCE_IP:
		if addrFamily.IsIPv4() {
			return DomainStrategy_FORCE_IP46
		} else if addrFamily.IsIPv6() {
			return DomainStrategy_FORCE_IP64
		}
	default:
	}
	return s
}
