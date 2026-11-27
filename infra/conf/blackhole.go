package conf

import (
	"encoding/base64"
	"strings"

	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/log"
	coreconfig "github.com/xtls/xray-core/core/config"
	"github.com/xtls/xray-core/proxy/freedom"
	"google.golang.org/protobuf/proto"
)

// BlackholeConfig is the settings of a blackhole outbound.
type BlackholeConfig struct {
	coreconfig.Blackhole
}

// ResponseConfig is what a blackhole outbound answers with.
type ResponseConfig = coreconfig.Response

// Build converts the settings of a blackhole outbound to its message.
func (v *BlackholeConfig) Build() (proto.Message, error) {
	errors.PrintDeprecatedFeatureWarning(`"blackhole" outbound`, `"freedom" outbound with a blocking "firewall" rule`)
	rule := &freedom.FirewallConfig{
		Action:   freedom.RuleAction_Block,
		Delay:    &freedom.Range{},
		LogLevel: log.Severity_Unknown.Enum(),
	}
	if v.Response != nil {
		responseName := strings.ToLower(v.Response.Type)
		switch responseName {
		case "none", "":
			rule.Reply = &freedom.Reply{Type: "none"}
		case "http":
			rule.Reply = &freedom.Reply{Type: "http"}
		case "custom":
			data, err := base64.StdEncoding.DecodeString(v.Response.CustomResponseData)
			if err != nil {
				return nil, errors.New("failed to decode custom response data: " + err.Error())
			}
			rule.Reply = &freedom.Reply{Type: "custom", CustomResponseData: data}
		default:
			return nil, errors.New("unknown blackhole response: " + responseName)
		}
	}

	return &freedom.Config{Firewall: []*freedom.FirewallConfig{rule}}, nil
}
