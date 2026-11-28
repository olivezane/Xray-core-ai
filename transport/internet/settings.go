// Code that is not generated: the settings of this package are plain Go values,
// and they say the name a configuration written for the API calls them.

package internet

import (
	"encoding/json"
	"fmt"

	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/serial"
)

// DomainStrategy is one of the values these settings take.
type DomainStrategy int32

const (
	DomainStrategy_AS_IS DomainStrategy = 0
	DomainStrategy_USE_IP DomainStrategy = 1
	DomainStrategy_USE_IP4 DomainStrategy = 2
	DomainStrategy_USE_IP6 DomainStrategy = 3
	DomainStrategy_USE_IP46 DomainStrategy = 4
	DomainStrategy_USE_IP64 DomainStrategy = 5
	DomainStrategy_FORCE_IP DomainStrategy = 6
	DomainStrategy_FORCE_IP4 DomainStrategy = 7
	DomainStrategy_FORCE_IP6 DomainStrategy = 8
	DomainStrategy_FORCE_IP46 DomainStrategy = 9
	DomainStrategy_FORCE_IP64 DomainStrategy = 10
)

func (x DomainStrategy) String() string {
	switch x {
	case DomainStrategy_AS_IS:
		return "AS_IS"
	case DomainStrategy_USE_IP:
		return "USE_IP"
	case DomainStrategy_USE_IP4:
		return "USE_IP4"
	case DomainStrategy_USE_IP6:
		return "USE_IP6"
	case DomainStrategy_USE_IP46:
		return "USE_IP46"
	case DomainStrategy_USE_IP64:
		return "USE_IP64"
	case DomainStrategy_FORCE_IP:
		return "FORCE_IP"
	case DomainStrategy_FORCE_IP4:
		return "FORCE_IP4"
	case DomainStrategy_FORCE_IP6:
		return "FORCE_IP6"
	case DomainStrategy_FORCE_IP46:
		return "FORCE_IP46"
	case DomainStrategy_FORCE_IP64:
		return "FORCE_IP64"
	default:
		return fmt.Sprint(int32(x))
	}
}

// AddressPortStrategy is one of the values these settings take.
type AddressPortStrategy int32

const (
	AddressPortStrategy_None AddressPortStrategy = 0
	AddressPortStrategy_SrvPortOnly AddressPortStrategy = 1
	AddressPortStrategy_SrvAddressOnly AddressPortStrategy = 2
	AddressPortStrategy_SrvPortAndAddress AddressPortStrategy = 3
	AddressPortStrategy_TxtPortOnly AddressPortStrategy = 4
	AddressPortStrategy_TxtAddressOnly AddressPortStrategy = 5
	AddressPortStrategy_TxtPortAndAddress AddressPortStrategy = 6
)

func (x AddressPortStrategy) String() string {
	switch x {
	case AddressPortStrategy_None:
		return "None"
	case AddressPortStrategy_SrvPortOnly:
		return "SrvPortOnly"
	case AddressPortStrategy_SrvAddressOnly:
		return "SrvAddressOnly"
	case AddressPortStrategy_SrvPortAndAddress:
		return "SrvPortAndAddress"
	case AddressPortStrategy_TxtPortOnly:
		return "TxtPortOnly"
	case AddressPortStrategy_TxtAddressOnly:
		return "TxtAddressOnly"
	case AddressPortStrategy_TxtPortAndAddress:
		return "TxtPortAndAddress"
	default:
		return fmt.Sprint(int32(x))
	}
}

// SocketConfig_TProxyMode is one of the values these settings take.
type SocketConfig_TProxyMode int32

const (
	SocketConfig_Off SocketConfig_TProxyMode = 0
	SocketConfig_TProxy SocketConfig_TProxyMode = 1
	SocketConfig_Redirect SocketConfig_TProxyMode = 2
)

func (x SocketConfig_TProxyMode) String() string {
	switch x {
	case SocketConfig_Off:
		return "SocketConfig_Off"
	case SocketConfig_TProxy:
		return "SocketConfig_TProxy"
	case SocketConfig_Redirect:
		return "SocketConfig_Redirect"
	default:
		return fmt.Sprint(int32(x))
	}
}

// TransportConfig is settings this package reads.
type TransportConfig struct {
	ProtocolName string `json:"protocolName,omitempty"`
	Settings any `json:"-"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*TransportConfig) SettingsName() string {
	return "xray.transport.internet.TransportConfig"
}

// StreamConfig is settings this package reads.
type StreamConfig struct {
	Address *net.IPOrDomain `json:"address,omitempty"`
	Port uint32 `json:"port,omitempty"`
	ProtocolName string `json:"protocolName,omitempty"`
	TransportSettings []*TransportConfig `json:"transportSettings,omitempty"`
	SecurityType string `json:"securityType,omitempty"`
	SecuritySettings []any `json:"-"`
	Udpmasks []any `json:"-"`
	Tcpmasks []any `json:"-"`
	QuicParams *QuicParams `json:"quicParams,omitempty"`
	SocketSettings *SocketConfig `json:"socketSettings,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*StreamConfig) SettingsName() string {
	return "xray.transport.internet.StreamConfig"
}

// QuicParams is settings this package reads.
type QuicParams struct {
	Congestion string `json:"congestion,omitempty"`
	BbrProfile string `json:"bbrProfile,omitempty"`
	BrutalUp uint64 `json:"brutalUp,omitempty"`
	BrutalDown uint64 `json:"brutalDown,omitempty"`
	BrutalDisableLossCompensation bool `json:"brutalDisableLossCompensation,omitempty"`
	InitStreamReceiveWindow uint64 `json:"initStreamReceiveWindow,omitempty"`
	MaxStreamReceiveWindow uint64 `json:"maxStreamReceiveWindow,omitempty"`
	InitConnReceiveWindow uint64 `json:"initConnReceiveWindow,omitempty"`
	MaxConnReceiveWindow uint64 `json:"maxConnReceiveWindow,omitempty"`
	MaxIdleTimeout int64 `json:"maxIdleTimeout,omitempty"`
	KeepAlivePeriod int64 `json:"keepAlivePeriod,omitempty"`
	DisablePathMtuDiscovery bool `json:"disablePathMtuDiscovery,omitempty"`
	DisableChromeParrot bool `json:"disableChromeParrot,omitempty"`
	DisableGSO bool `json:"disableGSO,omitempty"`
	MaxIncomingStreams int64 `json:"maxIncomingStreams,omitempty"`
	DisableStatelessReset bool `json:"disableStatelessReset,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*QuicParams) SettingsName() string {
	return "xray.transport.internet.QuicParams"
}

// CustomSockopt is settings this package reads.
type CustomSockopt struct {
	System string `json:"system,omitempty"`
	Network string `json:"network,omitempty"`
	Level string `json:"level,omitempty"`
	Opt string `json:"opt,omitempty"`
	Value string `json:"value,omitempty"`
	Type string `json:"type,omitempty"`
}

// SocketConfig is settings this package reads.
type SocketConfig struct {
	Mark int32 `json:"mark,omitempty"`
	Tfo int32 `json:"tfo,omitempty"`
	Tproxy SocketConfig_TProxyMode `json:"tproxy,omitempty"`
	ReceiveOriginalDestAddress bool `json:"receiveOriginalDestAddress,omitempty"`
	AcceptProxyProtocol bool `json:"acceptProxyProtocol,omitempty"`
	DomainStrategy DomainStrategy `json:"domainStrategy,omitempty"`
	DialerProxy string `json:"dialerProxy,omitempty"`
	TcpKeepAliveInterval int32 `json:"tcpKeepAliveInterval,omitempty"`
	TcpKeepAliveIdle int32 `json:"tcpKeepAliveIdle,omitempty"`
	TcpCongestion string `json:"tcpCongestion,omitempty"`
	Interface string `json:"interface,omitempty"`
	V6Only bool `json:"v6only,omitempty"`
	TcpWindowClamp int32 `json:"tcpWindowClamp,omitempty"`
	TcpUserTimeout int32 `json:"tcpUserTimeout,omitempty"`
	TcpMaxSeg int32 `json:"tcpMaxSeg,omitempty"`
	Penetrate bool `json:"penetrate,omitempty"`
	TcpMptcp bool `json:"tcpMptcp,omitempty"`
	CustomSockopt []*CustomSockopt `json:"customSockopt,omitempty"`
	AddressPortStrategy AddressPortStrategy `json:"addressPortStrategy,omitempty"`
	HappyEyeballs *HappyEyeballsConfig `json:"happyEyeballs,omitempty"`
	TrustedXForwardedFor []string `json:"trustedXForwardedFor,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*SocketConfig) SettingsName() string {
	return "xray.transport.internet.SocketConfig"
}

// HappyEyeballsConfig is settings this package reads.
type HappyEyeballsConfig struct {
	PrioritizeIpv6 bool `json:"prioritizeIpv6,omitempty"`
	Interleave uint32 `json:"interleave,omitempty"`
	TryDelayMs uint64 `json:"tryDelayMs,omitempty"`
	MaxConcurrentTry uint32 `json:"maxConcurrentTry,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*HappyEyeballsConfig) SettingsName() string {
	return "xray.transport.internet.HappyEyeballsConfig"
}

// A setting is a plain Go value, and a value on its own does not say what type it
// is, so its JSON form names it. The name is the same one a configuration written
// for the API uses, which is what lets a value be read back as its type.

// MarshalJSON writes the settings of a transport as the named value they are.
func (c *TransportConfig) MarshalJSON() ([]byte, error) {
	type plain TransportConfig
	return json.Marshal(struct {
		ProtocolName string `json:"protocolName,omitempty"`
		Settings     *serial.TypedMessage `json:"settings,omitempty"`
	}{
		ProtocolName: c.ProtocolName,
		Settings:     serial.ToTypedMessage(c.Settings),
	})
}

// UnmarshalJSON reads the settings of a transport as the type they name.
func (c *TransportConfig) UnmarshalJSON(b []byte) error {
	var in struct {
		ProtocolName string `json:"protocolName,omitempty"`
		Settings     *serial.TypedMessage `json:"settings,omitempty"`
	}
	if err := json.Unmarshal(b, &in); err != nil {
		return err
	}
	c.ProtocolName = in.ProtocolName
	settings, err := UnwrapSettings(in.Settings)
	if err != nil {
		return err
	}
	c.Settings = settings
	return nil
}

// MarshalJSON writes a stream setting as the named values they are.
func (c *StreamConfig) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Address           *net.IPOrDomain    `json:"address,omitempty"`
		Port              uint32             `json:"port,omitempty"`
		ProtocolName      string             `json:"protocolName,omitempty"`
		TransportSettings []*TransportConfig `json:"transportSettings,omitempty"`
		SecurityType      string             `json:"securityType,omitempty"`
		SecuritySettings  []*serial.TypedMessage `json:"securitySettings,omitempty"`
		Udpmasks          []*serial.TypedMessage `json:"udpmasks,omitempty"`
		Tcpmasks          []*serial.TypedMessage `json:"tcpmasks,omitempty"`
		QuicParams        *QuicParams        `json:"quicParams,omitempty"`
		SocketSettings    *SocketConfig      `json:"socketSettings,omitempty"`
	}{
		Address:           c.Address,
		Port:              c.Port,
		ProtocolName:      c.ProtocolName,
		TransportSettings: c.TransportSettings,
		SecurityType:      c.SecurityType,
		SecuritySettings:  namedSettings(c.SecuritySettings),
		Udpmasks:          namedSettings(c.Udpmasks),
		Tcpmasks:          namedSettings(c.Tcpmasks),
		QuicParams:        c.QuicParams,
		SocketSettings:    c.SocketSettings,
	})
}

// UnmarshalJSON reads a stream setting as the types they name.
func (c *StreamConfig) UnmarshalJSON(b []byte) error {
	var in struct {
		Address           *net.IPOrDomain    `json:"address,omitempty"`
		Port              uint32             `json:"port,omitempty"`
		ProtocolName      string             `json:"protocolName,omitempty"`
		TransportSettings []*TransportConfig `json:"transportSettings,omitempty"`
		SecurityType      string             `json:"securityType,omitempty"`
		SecuritySettings  []*serial.TypedMessage `json:"securitySettings,omitempty"`
		Udpmasks          []*serial.TypedMessage `json:"udpmasks,omitempty"`
		Tcpmasks          []*serial.TypedMessage `json:"tcpmasks,omitempty"`
		QuicParams        *QuicParams        `json:"quicParams,omitempty"`
		SocketSettings    *SocketConfig      `json:"socketSettings,omitempty"`
	}
	if err := json.Unmarshal(b, &in); err != nil {
		return err
	}
	c.Address = in.Address
	c.Port = in.Port
	c.ProtocolName = in.ProtocolName
	c.TransportSettings = in.TransportSettings
	c.SecurityType = in.SecurityType
	c.QuicParams = in.QuicParams
	c.SocketSettings = in.SocketSettings
	var err error
	if c.SecuritySettings, err = readSettings(in.SecuritySettings); err != nil {
		return err
	}
	if c.Udpmasks, err = readSettings(in.Udpmasks); err != nil {
		return err
	}
	c.Tcpmasks, err = readSettings(in.Tcpmasks)
	return err
}

// namedSettings writes settings as the named values they are.
func namedSettings(list []any) []*serial.TypedMessage {
	out := make([]*serial.TypedMessage, 0, len(list))
	for _, settings := range list {
		if settings == nil {
			continue
		}
		out = append(out, serial.ToTypedMessage(settings))
	}
	return out
}

// readSettings reads settings as the types they name.
func readSettings(list []*serial.TypedMessage) ([]any, error) {
	if len(list) == 0 {
		return nil, nil
	}
	out := make([]any, 0, len(list))
	for _, settings := range list {
		value, err := UnwrapSettings(settings)
		if err != nil {
			return nil, err
		}
		out = append(out, value)
	}
	return out, nil
}
