package conf

import (
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/serial"
	"github.com/xtls/xray-core/common/task"
	coreconfig "github.com/xtls/xray-core/core/config"
	"github.com/xtls/xray-core/proxy/hysteria"
	"github.com/xtls/xray-core/proxy/hysteria/account"
)

// HysteriaClientConfig is the settings of a HYSTERIA outbound.
type HysteriaClientConfig struct {
	coreconfig.HysteriaClient
}

func (c *HysteriaClientConfig) Build() (any, error) {
	if c.Version != 2 {
		return nil, errors.New("version != 2")
	}

	config := &hysteria.ClientConfig{}
	config.Server = &protocol.ServerEndpoint{
		Address: BuildAddress(c.Address),
		Port:    uint32(c.Port),
	}

	return config, nil
}

// HysteriaUserConfig is one user of a HYSTERIA inbound.
type HysteriaUserConfig = coreconfig.HysteriaUser

// HysteriaServerConfig is the settings of a HYSTERIA inbound.
type HysteriaServerConfig struct {
	coreconfig.HysteriaServer
}

func (c *HysteriaServerConfig) Build() (any, error) {
	if c.Version != 2 {
		return nil, errors.New("version != 2")
	}

	config := new(hysteria.ServerConfig)

	if c.Clients != nil {
		c.Users = c.Clients
	}
	if len(c.Users) > 0 {
		config.Users = make([]*protocol.User, len(c.Users))
		processUser := func(idx int) error {
			user := c.Users[idx]
			acc := &account.Account{
				Auth: user.Auth,
			}
			config.Users[idx] = &protocol.User{
				Email:   user.Email,
				Level:   user.Level,
				Account: serial.ToTypedMessage(acc),
			}
			return nil
		}
		if err := task.ParallelForN(len(c.Users), processUser); err != nil {
			return nil, err
		}
	}

	return config, nil
}
