// Code that is not generated: the settings of this package are plain Go values,
// and they say the name a configuration written for the API calls them.

package router

import (
	"encoding/json"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/geodata"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/serial"
)

// Config_DomainStrategy is how a routing decision resolves a domain.
type Config_DomainStrategy int32

const (
	Config_AsIs      Config_DomainStrategy = 0
	Config_UseIp     Config_DomainStrategy = 1
	Config_IpIfNonMatch Config_DomainStrategy = 2
	Config_IpOnDemand Config_DomainStrategy = 3
)

// Config is the settings of the router.
type Config struct {
	DomainStrategy Config_DomainStrategy `json:"domainStrategy,omitempty"`
	Rule           []*RoutingRule       `json:"rule,omitempty"`
	BalancingRule  []*BalancingRule     `json:"balancingRule,omitempty"`
	Script         string               `json:"script,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Config) SettingsName() string {
	return "xray.app.router.Config"
}

// RoutingRule is one rule of the router: where a matching connection goes.
type RoutingRule struct {
	// TargetTag is where a matching connection goes: exactly one of the variants
	// below.
	TargetTag isRoutingRule_TargetTag `json:"-"`

	RuleTag   string `json:"ruleTag,omitempty"`
	// List of domains for target domain matching.
	Domain []*geodata.DomainRule `json:"domain,omitempty"`
	// List of IPs for target IP address matching.
	Ip []*geodata.IPRule `json:"ip,omitempty"`
	// List of ports for target port matching.
	PortList *net.PortList `json:"portList,omitempty"`
	// List of networks for matching.
	Networks []net.Network `json:"networks,omitempty"`
	// List of IPs for source IP address matching.
	SourceIp []*geodata.IPRule `json:"sourceIp,omitempty"`
	// List of ports for source port matching.
	SourcePortList *net.PortList `json:"sourcePortList,omitempty"`
	UserEmail      []string      `json:"userEmail,omitempty"`
	InboundTag     []string      `json:"inboundTag,omitempty"`
	Protocol       []string      `json:"protocol,omitempty"`
	Attributes     map[string]string `json:"attributes,omitempty"`
	// List of IPs for local IP address matching.
	LocalIp []*geodata.IPRule `json:"localIp,omitempty"`
	// List of ports for local port matching.
	LocalPortList  *net.PortList `json:"localPortList,omitempty"`
	VlessRouteList *net.PortList `json:"vlessRouteList,omitempty"`
	Process        []string      `json:"process,omitempty"`
	Webhook        *WebhookConfig `json:"webhook,omitempty"`
	// List of operating systems for matching the one Xray itself is running on.
	LocalOs []string `json:"localOs,omitempty"`
}

type isRoutingRule_TargetTag interface {
	isRoutingRule_TargetTag()
}

// The variants of RoutingRule.TargetTag.
type (
	RoutingRule_Tag struct {
		// Tag of outbound that this rule is pointing to.
		Tag string `json:"tag,omitempty"`
	}
	RoutingRule_BalancingTag struct {
		// Tag of routing balancer.
		BalancingTag string `json:"balancingTag,omitempty"`
	}
)

func (*RoutingRule_Tag) isRoutingRule_TargetTag()          {}
func (*RoutingRule_BalancingTag) isRoutingRule_TargetTag() {}

// MarshalJSON writes a rule the way a configuration writes it: the variant of
// TargetTag becomes the field it names, because JSON cannot hold an interface.
func (x *RoutingRule) MarshalJSON() ([]byte, error) {
	type plain RoutingRule
	out := struct {
		*plain
		Tag          string `json:"tag,omitempty"`
		BalancingTag string `json:"balancingTag,omitempty"`
	}{plain: (*plain)(x)}
	switch v := x.TargetTag.(type) {
	case *RoutingRule_Tag:
		out.Tag = v.Tag
	case *RoutingRule_BalancingTag:
		out.BalancingTag = v.BalancingTag
	}
	return json.Marshal(out)
}

// UnmarshalJSON reads a rule the way a configuration writes it.
func (x *RoutingRule) UnmarshalJSON(b []byte) error {
	type plain RoutingRule
	fields := struct {
		*plain
		Tag          string `json:"tag"`
		BalancingTag string `json:"balancingTag"`
	}{plain: (*plain)(x)}
	if err := json.Unmarshal(b, &fields); err != nil {
		return err
	}
	switch {
	case fields.BalancingTag != "":
		x.TargetTag = &RoutingRule_BalancingTag{BalancingTag: fields.BalancingTag}
	default:
		x.TargetTag = &RoutingRule_Tag{Tag: fields.Tag}
	}
	return nil
}

// GetTag is the outbound this rule points to, or an empty string.
func (x *RoutingRule) GetTag() string {
	if x == nil {
		return ""
	}
	if v, ok := x.TargetTag.(*RoutingRule_Tag); ok {
		return v.Tag
	}
	return ""
}

// GetBalancingTag is the balancer this rule points to, or an empty string.
func (x *RoutingRule) GetBalancingTag() string {
	if x == nil {
		return ""
	}
	if v, ok := x.TargetTag.(*RoutingRule_BalancingTag); ok {
		return v.BalancingTag
	}
	return ""
}

// GetRuleTag is the tag of this rule, or an empty string.
func (x *RoutingRule) GetRuleTag() string {
	if x == nil {
		return ""
	}
	return x.RuleTag
}

// GetWebhook is where this rule reports a matching connection.
func (x *RoutingRule) GetWebhook() *WebhookConfig {
	if x == nil {
		return nil
	}
	return x.Webhook
}

// WebhookConfig is where a rule reports a matching connection.
type WebhookConfig struct {
	Url           string            `json:"url,omitempty"`
	Deduplication uint32            `json:"deduplication,omitempty"`
	Headers       map[string]string `json:"headers,omitempty"`
}

// BalancingRule is one balancer of the router.
type BalancingRule struct {
	Tag              string               `json:"tag,omitempty"`
	OutboundSelector []string             `json:"outboundSelector,omitempty"`
	Strategy         string               `json:"strategy,omitempty"`
	StrategySettings any `json:"-"`
	FallbackTag      string               `json:"fallbackTag,omitempty"`
}

// StrategyWeight is one weighted match of the least-load strategy.
type StrategyWeight struct {
	Regexp bool    `json:"regexp,omitempty"`
	Match  string  `json:"match,omitempty"`
	Value  float32 `json:"value,omitempty"`
}

// StrategyLeastLoadConfig is the settings of the least-load strategy.
type StrategyLeastLoadConfig struct {
	// weight settings
	Costs []*StrategyWeight `json:"costs,omitempty"`
	// RTT baselines for selecting, int64 values of time.Duration
	Baselines []int64 `json:"baselines,omitempty"`
	// expected nodes count to select
	Expected int32 `json:"expected,omitempty"`
	// max acceptable rtt, filter away high delay nodes. default 0
	MaxRTT int64 `json:"maxRTT,omitempty"`
	// acceptable failure rate
	Tolerance float32 `json:"tolerance,omitempty"`
}

// MarshalJSON writes a balancing rule with its strategy settings named, because a
// value on its own does not say what type it is.
func (x *BalancingRule) MarshalJSON() ([]byte, error) {
	type plain BalancingRule
	return json.Marshal(struct {
		*plain
		StrategySettings *serial.TypedMessage `json:"strategySettings,omitempty"`
	}{plain: (*plain)(x), StrategySettings: serial.ToTypedMessage(x.StrategySettings)})
}

// UnmarshalJSON reads a balancing rule with its strategy settings read as the type
// they name.
func (x *BalancingRule) UnmarshalJSON(b []byte) error {
	type plain BalancingRule
	in := struct {
		*plain
		StrategySettings *serial.TypedMessage `json:"strategySettings,omitempty"`
	}{plain: (*plain)(x)}
	if err := json.Unmarshal(b, &in); err != nil {
		return err
	}
	settings := common.UnwrapSettings(in.StrategySettings)
	x.StrategySettings = settings
	return nil
}

// SettingsName is what a configuration written for the API calls these settings.
func (*StrategyLeastLoadConfig) SettingsName() string {
	return "xray.app.router.StrategyLeastLoadConfig"
}
