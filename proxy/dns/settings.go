// Code that is not generated: the settings of this package are plain Go values,
// and they say the name a configuration written for the API calls them.

package dns

import (
	"fmt"
	geodata "github.com/xtls/xray-core/common/geodata"
	net "github.com/xtls/xray-core/common/net"
)

// RuleAction is one of the values these settings take.
type RuleAction int32

const (
	RuleAction_Direct RuleAction = 0
	RuleAction_Drop RuleAction = 1
	RuleAction_Return RuleAction = 2
	RuleAction_Hijack RuleAction = 3
)

func (x RuleAction) String() string {
	switch x {
	case RuleAction_Direct:
		return "Direct"
	case RuleAction_Drop:
		return "Drop"
	case RuleAction_Return:
		return "Return"
	case RuleAction_Hijack:
		return "Hijack"
	default:
		return fmt.Sprint(int32(x))
	}
}

// DNSRuleConfig is settings this package reads.
type DNSRuleConfig struct {
	Action RuleAction `json:"action,omitempty"`
	QType []int32 `json:"qType,omitempty"`
	Domain []*geodata.DomainRule `json:"domain,omitempty"`
	RCode uint32 `json:"rCode,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*DNSRuleConfig) SettingsName() string {
	return "xray.proxy.dns.DNSRuleConfig"
}

// Config is settings this package reads.
type Config struct {
	UserLevel uint32 `json:"userLevel,omitempty"`
	Rule []*DNSRuleConfig `json:"rule,omitempty"`
	RewriteServer *net.Endpoint `json:"rewriteServer,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Config) SettingsName() string {
	return "xray.proxy.dns.Config"
}
