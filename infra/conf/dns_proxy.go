package conf

import (
	"strings"

	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/geodata"
	"github.com/xtls/xray-core/common/net"
	coreconfig "github.com/xtls/xray-core/core/config"
	"github.com/xtls/xray-core/proxy/dns"
	"google.golang.org/protobuf/proto"
)

// DNSOutboundRuleConfig is one rule of a DNS outbound.
type DNSOutboundRuleConfig struct {
	coreconfig.DNSOutboundRule
}

// Build converts the rule to its message.
func (c *DNSOutboundRuleConfig) Build() (*dns.DNSRuleConfig, error) {
	return buildDNSOutboundRule(&c.DNSOutboundRule)
}

func buildDNSOutboundRule(c *coreconfig.DNSOutboundRule) (*dns.DNSRuleConfig, error) {
	rule := &dns.DNSRuleConfig{}

	switch strings.ToLower(c.Action) {
	case "direct":
		rule.Action = dns.RuleAction_Direct
	case "drop":
		rule.Action = dns.RuleAction_Drop
	case "return":
		rule.Action = dns.RuleAction_Return
	case "hijack":
		rule.Action = dns.RuleAction_Hijack
	default:
		return nil, errors.New("unknown action: ", c.Action)
	}

	if c.QType != nil {
		for _, r := range c.QType.Range {
			for qType := r.From; qType <= r.To; qType++ {
				rule.QType = append(rule.QType, int32(qType))
			}
		}
	}

	if c.Domain != nil {
		rules, err := geodata.ParseDomainRules(*c.Domain, geodata.Domain_Substr)
		if err != nil {
			return nil, err
		}
		rule.Domain = rules
	}

	if c.RCode > 65535 {
		return nil, errors.New("rCode out of range: ", c.RCode)
	}
	rule.RCode = c.RCode

	return rule, nil
}

// DNSOutboundConfig is the settings of a DNS outbound.
type DNSOutboundConfig struct {
	coreconfig.DNSOutbound
}

func (c *DNSOutboundConfig) Build() (proto.Message, error) {
	if len(c.Network) > 0 {
		c.RewriteNetwork = c.Network
	}
	if c.Address != nil {
		c.RewriteAddress = c.Address
	}
	if c.Port != 0 {
		c.RewritePort = c.Port
	}
	config := &dns.Config{
		RewriteServer: &net.Endpoint{
			Network: BuildNetwork(c.RewriteNetwork),
			Port:    uint32(c.RewritePort),
		},
		UserLevel: c.UserLevel,
	}
	if c.RewriteAddress != nil {
		config.RewriteServer.Address = BuildAddress(c.RewriteAddress)
	}

	// todo: remove legacy
	if c.NonIPQuery != nil || c.BlockTypes != nil {
		if c.Rules != nil {
			return nil, errors.New("legacy nonIPQuery and blockTypes cannot be mixed with rules")
		}
		errors.PrintDeprecatedFeatureWarning(`"nonIPQuery" and "blockTypes"`, `"rules"`)
		rules, err := c.buildLegacyDNSPolicy()
		if err != nil {
			return nil, err
		}
		config.Rule = rules
		return config, nil
	}

	for _, r := range c.Rules {
		rule, err := buildDNSOutboundRule(r)
		if err != nil {
			return nil, err
		}
		config.Rule = append(config.Rule, rule)
	}

	return config, nil
}

// todo: remove legacy
func (c *DNSOutboundConfig) buildLegacyDNSPolicy() ([]*dns.DNSRuleConfig, error) {
	rules := make([]*dns.DNSRuleConfig, 0, 3)

	mode := "reject"
	if c.NonIPQuery != nil && *c.NonIPQuery != "" {
		mode = *c.NonIPQuery
	}
	switch mode {
	case "", "reject", "drop", "skip":
	default:
		return nil, errors.New("unknown nonIPQuery: ", mode)
	}

	if c.BlockTypes != nil && len(*c.BlockTypes) > 0 {
		rule := &dns.DNSRuleConfig{Action: dns.RuleAction_Drop}
		if mode == "reject" {
			rule.Action = dns.RuleAction_Return
			rule.RCode = 5
		}
		for _, qType := range *c.BlockTypes {
			if qType < 0 || qType > 65535 {
				return nil, errors.New("legacy blockTypes qType out of range: ", qType)
			}
			rule.QType = append(rule.QType, qType)
		}
		rules = append(rules, rule)
	}

	{
		rule := &dns.DNSRuleConfig{Action: dns.RuleAction_Hijack}
		rule.QType = append(rule.QType, 1)
		rule.QType = append(rule.QType, 28)
		rules = append(rules, rule)
	}

	{
		rule := &dns.DNSRuleConfig{Action: dns.RuleAction_Return}
		if mode == "reject" {
			rule.Action = dns.RuleAction_Return
			rule.RCode = 5
		} else if mode == "drop" {
			rule.Action = dns.RuleAction_Drop
		} else if mode == "skip" {
			rule.Action = dns.RuleAction_Direct
		}
		rules = append(rules, rule)
	}

	return rules, nil
}
