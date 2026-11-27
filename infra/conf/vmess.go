package conf

import (
	"encoding/json"
	"strings"

	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/serial"
	"github.com/xtls/xray-core/common/task"
	"github.com/xtls/xray-core/common/uuid"
	coreconfig "github.com/xtls/xray-core/core/config"
	"github.com/xtls/xray-core/proxy/vmess"
	"github.com/xtls/xray-core/proxy/vmess/inbound"
	"github.com/xtls/xray-core/proxy/vmess/outbound"
	"google.golang.org/protobuf/proto"
)

// VMessAccount is one user of a VMess inbound or outbound.
type VMessAccount = coreconfig.VMessAccount

// Build implements Buildable
func buildVMessAccount(a *coreconfig.VMessAccount) *vmess.Account {
	var st protocol.SecurityType
	switch strings.ToLower(a.Security) {
	case "aes-128-gcm":
		st = protocol.SecurityType_AES128_GCM
	case "chacha20-poly1305":
		st = protocol.SecurityType_CHACHA20_POLY1305
	case "auto":
		st = protocol.SecurityType_AUTO
	default:
		st = protocol.SecurityType_AUTO
	}
	return &vmess.Account{
		Id: a.ID,
		SecuritySettings: &protocol.SecurityConfig{
			Type: st,
		},
		TestsEnabled: a.Experiments,
	}
}

// VMessDefaultConfig is the default settings of a VMess inbound.
type VMessDefaultConfig = coreconfig.VMessDefault

// Build implements Buildable
func buildVMessDefault(c *coreconfig.VMessDefault) *inbound.DefaultConfig {
	config := new(inbound.DefaultConfig)
	config.Level = uint32(c.Level)
	return config
}

// VMessInboundConfig is the settings of a VMess inbound.
type VMessInboundConfig struct {
	coreconfig.VMessInbound
}

// Build implements Buildable
func (c *VMessInboundConfig) Build() (proto.Message, error) {
	errors.PrintNonRemovalDeprecatedFeatureWarning("VMess (with no Forward Secrecy, etc.)", "VLESS Encryption")

	config := &inbound.Config{}

	if c.Defaults != nil {
		config.Default = buildVMessDefault(c.Defaults)
	}

	if c.Clients != nil {
		c.Users = c.Clients
	}
	config.User = make([]*protocol.User, len(c.Users))
	processUser := func(idx int) error {
		rawData := c.Users[idx]
		user := new(protocol.User)
		if err := json.Unmarshal(rawData, user); err != nil {
			return errors.New("invalid VMess user").Base(err)
		}
		account := new(VMessAccount)
		if err := json.Unmarshal(rawData, account); err != nil {
			return errors.New("invalid VMess user").Base(err)
		}

		u, err := uuid.ParseString(account.ID)
		if err != nil {
			return err
		}
		account.ID = u.String()

		user.Account = serial.ToTypedMessage(buildVMessAccount(account))
		config.User[idx] = user
		return nil
	}
	if err := task.ParallelForN(len(c.Users), processUser); err != nil {
		return nil, err
	}

	return config, nil
}

// VMessOutboundTarget is one endpoint of a VMess outbound.
type VMessOutboundTarget = coreconfig.VMessOutboundTarget

// VMessOutboundConfig is the settings of a VMess outbound.
type VMessOutboundConfig struct {
	coreconfig.VMessOutbound
}

// Build implements Buildable
func (c *VMessOutboundConfig) Build() (proto.Message, error) {
	errors.PrintNonRemovalDeprecatedFeatureWarning("VMess (with no Forward Secrecy, etc.)", "VLESS Encryption")

	multiplex, err := BuildMultiplex(c.Multiplex)
	if err != nil {
		return nil, err
	}
	config := &outbound.Config{Multiplex: multiplex}
	if c.Address != nil {
		c.Receivers = []*VMessOutboundTarget{
			{
				Address: c.Address,
				Port:    c.Port,
				Users:   []json.RawMessage{{}},
			},
		}
	}
	if len(c.Receivers) != 1 {
		return nil, errors.New(`VMess settings: "vnext" should have one and only one member. Multiple endpoints in "vnext" should use multiple VMess outbounds and routing balancer instead`)
	}
	rec := c.Receivers[0]
	if len(rec.Users) != 1 {
		return nil, errors.New(`VMess vnext: "users" should have one and only one member. Multiple members in "users" should use multiple VMess outbounds and routing balancer instead`)
	}
	if rec.Address == nil {
		return nil, errors.New(`VMess vnext: "address" is not set`)
	}
	spec := &protocol.ServerEndpoint{
		Address: BuildAddress(rec.Address),
		Port:    uint32(rec.Port),
	}
	rawUser := rec.Users[0]
	user := new(protocol.User)
	if c.Address != nil {
		user.Level = c.Level
		user.Email = c.Email
	} else {
		if err := json.Unmarshal(rawUser, user); err != nil {
			return nil, errors.New("invalid VMess user").Base(err)
		}
	}
	account := new(VMessAccount)
	if c.Address != nil {
		account.ID = c.ID
		account.Security = c.Security
		account.Experiments = c.Experiments
	} else {
		if err := json.Unmarshal(rawUser, account); err != nil {
			return nil, errors.New("invalid VMess user").Base(err)
		}
	}

	u, err := uuid.ParseString(account.ID)
	if err != nil {
		return nil, err
	}
	account.ID = u.String()

	user.Account = serial.ToTypedMessage(buildVMessAccount(account))
	spec.User = user
	config.Receiver = spec
	return config, nil
}
