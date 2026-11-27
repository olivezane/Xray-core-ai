package conf

import (
	"github.com/xtls/xray-core/app/policy"
	coreconfig "github.com/xtls/xray-core/core/config"
)

// The policy section of a configuration is the plain model in core/config. This
// file is the boundary: it converts that model to the protobuf message the
// instance consumes, and it is where the units of the model become the units of
// the message (seconds as Second, kilobytes as bytes).
type (
	Policy       = coreconfig.Policy
	SystemPolicy = coreconfig.SystemPolicy
	PolicyConfig = coreconfig.PolicySet
)

// BuildPolicy converts the policy section to its protobuf message.
func BuildPolicy(c *coreconfig.PolicySet) (*policy.Config, error) {
	if c == nil {
		return nil, nil
	}
	levels := make(map[uint32]*policy.Policy)
	for level, p := range c.Levels {
		if p != nil {
			levels[level] = buildLevelPolicy(p)
		}
	}

	config := &policy.Config{Level: levels}
	if c.System != nil {
		config.System = buildSystemPolicy(c.System)
	}
	return config, nil
}

// BuildLevelPolicy converts one level of the policy section to its protobuf
// message.
func BuildLevelPolicy(t *coreconfig.Policy) *policy.Policy {
	return buildLevelPolicy(t)
}

func buildLevelPolicy(t *coreconfig.Policy) *policy.Policy {
	timeout := new(policy.Policy_Timeout)
	if t.Handshake != nil {
		timeout.Handshake = &policy.Second{Value: *t.Handshake}
	}
	if t.ConnectionIdle != nil {
		timeout.ConnectionIdle = &policy.Second{Value: *t.ConnectionIdle}
	}
	if t.UplinkOnly != nil {
		timeout.UplinkOnly = &policy.Second{Value: *t.UplinkOnly}
	}
	if t.DownlinkOnly != nil {
		timeout.DownlinkOnly = &policy.Second{Value: *t.DownlinkOnly}
	}

	p := &policy.Policy{
		Timeout: timeout,
		Stats: &policy.Policy_Stats{
			UserUplink:   t.StatsUserUplink,
			UserDownlink: t.StatsUserDownlink,
			UserOnline:   t.StatsUserOnline,
		},
	}

	if t.BufferSize != nil {
		connection := int32(-1)
		if *t.BufferSize >= 0 {
			// The model is in kilobytes, the message in bytes.
			connection = (*t.BufferSize) * 1024
		}
		p.Buffer = &policy.Policy_Buffer{Connection: connection}
	}
	return p
}

func buildSystemPolicy(p *coreconfig.SystemPolicy) *policy.SystemPolicy {
	return &policy.SystemPolicy{
		Stats: &policy.SystemPolicy_Stats{
			InboundUplink:    p.StatsInboundUplink,
			InboundDownlink:  p.StatsInboundDownlink,
			OutboundUplink:   p.StatsOutboundUplink,
			OutboundDownlink: p.StatsOutboundDownlink,
		},
	}
}
