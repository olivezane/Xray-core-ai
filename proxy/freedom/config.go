package freedom

import (
	"fmt"

	"github.com/xtls/xray-core/common/geodata"
	"github.com/xtls/xray-core/common/log"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/transport/internet"
)

// These are the settings of a freedom outbound. They are plain Go values, so the
// file keeps the name of the message definition it replaced while holding no
// generated code: a configuration is decoded from JSON and crosses the API as
// JSON under the name Config.SettingsName gives.

// RuleAction is what a firewall rule does with a connection.
type RuleAction int32

const (
	RuleAction_Allow RuleAction = 0
	RuleAction_Block RuleAction = 1
)

func (x RuleAction) String() string {
	switch x {
	case RuleAction_Allow:
		return "Allow"
	case RuleAction_Block:
		return "Block"
	default:
		return fmt.Sprint(int32(x))
	}
}

// DestinationOverride sends a connection to a fixed destination.
type DestinationOverride struct {
	Server *protocol.ServerEndpoint `json:"server,omitempty"`
}

// Fragment splits the first writes of a connection.
type Fragment struct {
	PacketsFrom uint64 `json:"packets_from,omitempty"`
	PacketsTo   uint64 `json:"packets_to,omitempty"`
	LengthMin   uint64 `json:"length_min,omitempty"`
	LengthMax   uint64 `json:"length_max,omitempty"`
	IntervalMin uint64 `json:"interval_min,omitempty"`
	IntervalMax uint64 `json:"interval_max,omitempty"`
	MaxSplitMin uint64 `json:"max_split_min,omitempty"`
	MaxSplitMax uint64 `json:"max_split_max,omitempty"`
}

// Noise writes bytes before the payload of a connection.
type Noise struct {
	LengthMin uint64 `json:"length_min,omitempty"`
	LengthMax uint64 `json:"length_max,omitempty"`
	DelayMin  uint64 `json:"delay_min,omitempty"`
	DelayMax  uint64 `json:"delay_max,omitempty"`
	Packet    []byte `json:"packet,omitempty"`
	ApplyTo   string `json:"apply_to,omitempty"`
}

// Range is a range of numbers.
type Range struct {
	Min uint64 `json:"min,omitempty"`
	Max uint64 `json:"max,omitempty"`
}

// Reply is what a blocking rule answers with.
type Reply struct {
	Type               string `json:"type,omitempty"`
	CustomResponseData []byte `json:"custom_response_data,omitempty"`
}

// FirewallConfig is one rule of a freedom outbound.
type FirewallConfig struct {
	Action   RuleAction     `json:"action,omitempty"`
	Networks []net.Network  `json:"networks,omitempty"`
	PortList *net.PortList  `json:"port_list,omitempty"`
	Ip       []*geodata.IPRule `json:"ip,omitempty"`
	Delay    *Range         `json:"delay,omitempty"`
	LogLevel *log.Severity  `json:"log_level,omitempty"`
	Reply    *Reply         `json:"reply,omitempty"`
}

// Config is the settings of a freedom outbound.
type Config struct {
	DomainStrategy      internet.DomainStrategy `json:"domain_strategy,omitempty"`
	DestinationOverride *DestinationOverride    `json:"destination_override,omitempty"`
	UserLevel           uint32                  `json:"user_level,omitempty"`
	Fragment            *Fragment               `json:"fragment,omitempty"`
	Noises              []*Noise                `json:"noises,omitempty"`
	Firewall            []*FirewallConfig       `json:"firewall,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Config) SettingsName() string {
	return "xray.proxy.freedom.Config"
}
