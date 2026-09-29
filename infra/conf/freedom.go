package conf

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"net"
	"strings"

	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/geodata"
	xnet "github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/proxy/freedom"
	"github.com/xtls/xray-core/transport/internet"
	"google.golang.org/protobuf/proto"
)

type FreedomConfig struct {
	TargetStrategy string                    `json:"targetStrategy"`
	DomainStrategy string                    `json:"domainStrategy"`
	Redirect       string                    `json:"redirect"`
	UserLevel      uint32                    `json:"userLevel"`
	Fragment       *Fragment                 `json:"fragment"`
	Noise          *Noise                    `json:"noise"`
	Noises         []*Noise                  `json:"noises"`
	ProxyProtocol  uint32                    `json:"proxyProtocol"`
	IPsBlocked     *StringList               `json:"ipsBlocked"`
	FinalRules     []*FreedomFinalRuleConfig `json:"finalRules"`
}

type Fragment struct {
	Packets  string      `json:"packets"`
	Length   *Int32Range `json:"length"`
	Interval *Int32Range `json:"interval"`
	MaxSplit *Int32Range `json:"maxSplit"`
}

type Noise struct {
	Type    string      `json:"type"`
	Packet  string      `json:"packet"`
	Delay   *Int32Range `json:"delay"`
	ApplyTo string      `json:"applyTo"`
}

type FreedomFinalRuleConfig struct {
	Action     string       `json:"action"`
	Network    *NetworkList `json:"network"`
	Port       *PortList    `json:"port"`
	IP         *StringList  `json:"ip"`
	BlockDelay *Int32Range  `json:"blockDelay"`
}

// Build implements Buildable
func (c *FreedomConfig) Build() (proto.Message, error) {
	if c.IPsBlocked != nil {
		// todo: remove legacy
		errors.LogWarning(context.Background(), `The feature "ipsBlocked" has been removed and migrated to "finalRules". Please update your config(s) according to release note and documentation.`)
	}

	config := new(freedom.Config)
	targetStrategy := c.TargetStrategy
	if targetStrategy == "" {
		targetStrategy = c.DomainStrategy
	}
	switch strings.ToLower(targetStrategy) {
	case "asis", "":
		config.DomainStrategy = internet.DomainStrategy_AS_IS
	case "useip":
		config.DomainStrategy = internet.DomainStrategy_USE_IP
	case "useipv4":
		config.DomainStrategy = internet.DomainStrategy_USE_IP4
	case "useipv6":
		config.DomainStrategy = internet.DomainStrategy_USE_IP6
	case "useipv4v6":
		config.DomainStrategy = internet.DomainStrategy_USE_IP46
	case "useipv6v4":
		config.DomainStrategy = internet.DomainStrategy_USE_IP64
	case "forceip":
		config.DomainStrategy = internet.DomainStrategy_FORCE_IP
	case "forceipv4":
		config.DomainStrategy = internet.DomainStrategy_FORCE_IP4
	case "forceipv6":
		config.DomainStrategy = internet.DomainStrategy_FORCE_IP6
	case "forceipv4v6":
		config.DomainStrategy = internet.DomainStrategy_FORCE_IP46
	case "forceipv6v4":
		config.DomainStrategy = internet.DomainStrategy_FORCE_IP64
	default:
		return nil, errors.New("unsupported domain strategy: ", targetStrategy)
	}

	if c.Fragment != nil {
		config.Fragment = new(freedom.Fragment)

		switch strings.ToLower(c.Fragment.Packets) {
		case "tlshello":
			// TLS Hello Fragmentation (into multiple handshake messages)
			config.Fragment.PacketsFrom = 0
			config.Fragment.PacketsTo = 1
		case "":
			// TCP Segmentation (all packets)
			config.Fragment.PacketsFrom = 0
			config.Fragment.PacketsTo = 0
		default:
			// TCP Segmentation (range)
			from, to, err := ParseRangeString(c.Fragment.Packets)
			if err != nil {
				return nil, errors.New("Invalid PacketsFrom").Base(err)
			}
			if config.Fragment.PacketsFrom, config.Fragment.PacketsTo, err = ToUint64Pair(from, to, "Packets"); err != nil {
				return nil, err
			}
			if config.Fragment.PacketsFrom == 0 {
				return nil, errors.New("PacketsFrom can't be 0")
			}
		}

		{
			if c.Fragment.Length == nil {
				return nil, errors.New("Length can't be empty")
			}
			lengthMin, lengthMax, err := ToUint64Pair(c.Fragment.Length.From, c.Fragment.Length.To, "Length")
			if err != nil {
				return nil, err
			}
			config.Fragment.LengthMin = lengthMin
			config.Fragment.LengthMax = lengthMax
			if config.Fragment.LengthMin == 0 {
				return nil, errors.New("LengthMin can't be 0")
			}
		}

		{
			if c.Fragment.Interval == nil {
				return nil, errors.New("Interval can't be empty")
			}
			intervalMin, intervalMax, err := ToUint64Pair(c.Fragment.Interval.From, c.Fragment.Interval.To, "Interval")
			if err != nil {
				return nil, err
			}
			config.Fragment.IntervalMin = intervalMin
			config.Fragment.IntervalMax = intervalMax
		}

		{
			if c.Fragment.MaxSplit != nil {
				maxSplitMin, maxSplitMax, err := ToUint64Pair(c.Fragment.MaxSplit.From, c.Fragment.MaxSplit.To, "MaxSplit")
				if err != nil {
					return nil, err
				}
				config.Fragment.MaxSplitMin = maxSplitMin
				config.Fragment.MaxSplitMax = maxSplitMax
			}
		}
	}

	if c.Noise != nil {
		return nil, errors.PrintRemovedFeatureError("noise = { ... }", "noises = [ { ... } ]")
	}

	if c.Noises != nil {
		for _, n := range c.Noises {
			NConfig, err := ParseNoise(n)
			if err != nil {
				return nil, err
			}
			config.Noises = append(config.Noises, NConfig)
		}
	}

	config.UserLevel = c.UserLevel

	if len(c.Redirect) > 0 {
		host, portStr, err := net.SplitHostPort(c.Redirect)
		if err != nil {
			return nil, errors.New("invalid redirect address: ", c.Redirect, ": ", err).Base(err)
		}
		port, err := xnet.PortFromString(portStr)
		if err != nil {
			return nil, errors.New("invalid redirect port: ", c.Redirect, ": ", err).Base(err)
		}
		config.DestinationOverride = &freedom.DestinationOverride{
			Server: &protocol.ServerEndpoint{
				Port: uint32(port),
			},
		}

		if len(host) > 0 {
			config.DestinationOverride.Server.Address = xnet.NewIPOrDomain(xnet.ParseAddress(host))
		}
	}

	if c.ProxyProtocol > 0 && c.ProxyProtocol <= 2 {
		config.ProxyProtocol = c.ProxyProtocol
	}

	for _, r := range c.FinalRules {
		rule, err := r.Build()
		if err != nil {
			return nil, err
		}
		config.FinalRules = append(config.FinalRules, rule)
	}

	return config, nil
}

func ParseNoise(noise *Noise) (*freedom.Noise, error) {
	var err error
	NConfig := new(freedom.Noise)
	noise.Packet = strings.TrimSpace(noise.Packet)

	switch noise.Type {
	case "rand":
		minVal, maxVal, err := ParseRangeString(noise.Packet)
		if err != nil {
			return nil, errors.New("invalid value for rand Length").Base(err)
		}
		if NConfig.LengthMin, NConfig.LengthMax, err = ToUint64Pair(minVal, maxVal, "rand length"); err != nil {
			return nil, err
		}
		if NConfig.LengthMin == 0 {
			return nil, errors.New("rand lengthMin or lengthMax cannot be 0")
		}

	case "str":
		// user input string
		NConfig.Packet = []byte(noise.Packet)

	case "hex":
		// user input hex
		NConfig.Packet, err = hex.DecodeString(noise.Packet)
		if err != nil {
			return nil, errors.New("Invalid hex string").Base(err)
		}

	case "base64":
		// user input base64
		NConfig.Packet, err = base64.RawURLEncoding.DecodeString(strings.NewReplacer("+", "-", "/", "_", "=", "").Replace(noise.Packet))
		if err != nil {
			return nil, errors.New("Invalid base64 string").Base(err)
		}

	default:
		return nil, errors.New("Invalid packet, only rand/str/hex/base64 are supported")
	}

	if noise.Delay != nil {
		if NConfig.DelayMin, NConfig.DelayMax, err = ToUint64Pair(noise.Delay.From, noise.Delay.To, "Delay"); err != nil {
			return nil, err
		}
	}
	switch strings.ToLower(noise.ApplyTo) {
	case "", "ip", "all":
		NConfig.ApplyTo = "ip"
	case "ipv4":
		NConfig.ApplyTo = "ipv4"
	case "ipv6":
		NConfig.ApplyTo = "ipv6"
	default:
		return nil, errors.New("Invalid applyTo, only ip/ipv4/ipv6 are supported")
	}
	return NConfig, nil
}

func (c *FreedomFinalRuleConfig) Build() (*freedom.FinalRuleConfig, error) {
	rule := &freedom.FinalRuleConfig{}

	switch strings.ToLower(c.Action) {
	case "allow":
		rule.Action = freedom.RuleAction_Allow
	case "block":
		rule.Action = freedom.RuleAction_Block
	default:
		return nil, errors.New("unknown action: ", c.Action)
	}

	if c.Network != nil {
		rule.Networks = c.Network.Build()
	}

	if c.Port != nil {
		rule.PortList = c.Port.Build()
	}

	if c.IP != nil {
		rules, err := geodata.ParseIPRules(*c.IP)
		if err != nil {
			return nil, err
		}
		rule.Ip = rules
	}

	if c.BlockDelay != nil {
		minVal, maxVal, err := ToUint64Pair(c.BlockDelay.From, c.BlockDelay.To, "BlockDelay")
		if err != nil {
			return nil, err
		}
		rule.BlockDelay = &freedom.Range{
			Min: minVal,
			Max: maxVal,
		}
	}

	return rule, nil
}
