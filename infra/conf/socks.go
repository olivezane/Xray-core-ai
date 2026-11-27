package conf

import (
	"encoding/json"

	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/serial"
	coreconfig "github.com/xtls/xray-core/core/config"
	"github.com/xtls/xray-core/proxy/socks"
	"google.golang.org/protobuf/proto"
)

// SocksAccount is one user of a socks inbound, from the configuration model.
type SocksAccount = coreconfig.SocksAccount

func buildSocksAccount(v *coreconfig.SocksAccount) *socks.Account {
	return &socks.Account{
		Username: v.Username,
		Password: v.Password,
	}
}

const (
	AuthMethodNoAuth   = "noauth"
	AuthMethodUserPass = "password"
)

// SocksServerConfig is the settings of a socks inbound.
type SocksServerConfig struct {
	coreconfig.SocksServer
}

func (v *SocksServerConfig) Build() (proto.Message, error) {
	config := new(socks.ServerConfig)
	switch v.AuthMethod {
	case AuthMethodNoAuth:
		config.AuthType = socks.AuthType_NO_AUTH
	case AuthMethodUserPass:
		config.AuthType = socks.AuthType_PASSWORD
	default:
		config.AuthType = socks.AuthType_NO_AUTH
	}

	if v.Accounts != nil {
		v.Users = v.Accounts
	}
	// TODO: PB
	if len(v.Users) > 0 {
		config.Accounts = make(map[string]string, len(v.Users))
		for _, account := range v.Users {
			config.Accounts[account.Username] = account.Password
		}
	}

	config.UdpEnabled = v.UDP
	if v.Host != nil {
		config.Address = BuildAddress(v.Host)
	}

	config.UserLevel = v.UserLevel
	return config, nil
}

// SocksRemoteConfig is one endpoint of a socks outbound.
type SocksRemoteConfig = coreconfig.SocksRemote

// SocksClientConfig is the settings of a socks outbound.
type SocksClientConfig struct {
	coreconfig.SocksClient
}

func (v *SocksClientConfig) Build() (proto.Message, error) {
	config := new(socks.ClientConfig)
	if v.Address != nil {
		v.Servers = []*SocksRemoteConfig{
			{
				Address: v.Address,
				Port:    v.Port,
			},
		}
		if len(v.Username) > 0 {
			v.Servers[0].Users = []json.RawMessage{{}}
		}
	}
	if len(v.Servers) != 1 {
		return nil, errors.New(`SOCKS settings: "servers" should have one and only one member. Multiple endpoints in "servers" should use multiple SOCKS outbounds and routing balancer instead`)
	}
	serverConfig := v.Servers[0]
	if len(serverConfig.Users) > 1 {
		return nil, errors.New(`SOCKS servers: "users" should have one member at most. Multiple members in "users" should use multiple SOCKS outbounds and routing balancer instead`)
	}
	server := &protocol.ServerEndpoint{
		Address: BuildAddress(serverConfig.Address),
		Port:    uint32(serverConfig.Port),
	}
	if len(serverConfig.Users) == 1 {
		rawUser := serverConfig.Users[0]
		user := new(protocol.User)
		if v.Address != nil {
			user.Level = v.Level
			user.Email = v.Email
		} else {
			if err := json.Unmarshal(rawUser, user); err != nil {
				return nil, errors.New("failed to parse Socks user").Base(err)
			}
		}
		account := new(SocksAccount)
		if v.Address != nil {
			account.Username = v.Username
			account.Password = v.Password
		} else {
			if err := json.Unmarshal(rawUser, account); err != nil {
				return nil, errors.New("failed to parse socks account").Base(err)
			}
		}
		user.Account = serial.ToTypedMessage(buildSocksAccount(account))
		server.User = user
	}
	config.Server = server
	return config, nil
}
