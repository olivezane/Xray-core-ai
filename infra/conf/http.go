package conf

import (
	"encoding/json"

	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/serial"
	coreconfig "github.com/xtls/xray-core/core/config"
	"github.com/xtls/xray-core/proxy/http"
	"google.golang.org/protobuf/proto"
)

// HTTPAccount is one user of an HTTP inbound, from the configuration model.
type HTTPAccount = coreconfig.HTTPAccount

func buildHTTPAccount(v *coreconfig.HTTPAccount) *http.Account {
	return &http.Account{
		Username: v.Username,
		Password: v.Password,
	}
}

// HTTPServerConfig is the settings of an HTTP inbound.
type HTTPServerConfig struct {
	coreconfig.HTTPServer
}

func (c *HTTPServerConfig) Build() (proto.Message, error) {
	config := &http.ServerConfig{
		AllowTransparent: c.Transparent,
		UserLevel:        c.UserLevel,
	}

	if c.Accounts != nil {
		c.Users = c.Accounts
	}
	// TODO: PB
	if len(c.Users) > 0 {
		config.Accounts = make(map[string]string)
		for _, account := range c.Users {
			config.Accounts[account.Username] = account.Password
		}
	}

	return config, nil
}

// HTTPRemoteConfig is one endpoint of an HTTP outbound.
type HTTPRemoteConfig = coreconfig.HTTPRemote

// HTTPClientConfig is the settings of an HTTP outbound.
type HTTPClientConfig struct {
	coreconfig.HTTPClient
}

func (v *HTTPClientConfig) Build() (proto.Message, error) {
	config := new(http.ClientConfig)
	if v.Address != nil {
		v.Servers = []*HTTPRemoteConfig{
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
		return nil, errors.New(`HTTP settings: "servers" should have one and only one member. Multiple endpoints in "servers" should use multiple HTTP outbounds and routing balancer instead`)
	}
	serverConfig := v.Servers[0]
	if len(serverConfig.Users) > 1 {
		return nil, errors.New(`HTTP servers: "users" should have one member at most. Multiple members in "users" should use multiple HTTP outbounds and routing balancer instead`)
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
				return nil, errors.New("failed to parse HTTP user").Base(err)
			}
		}
		account := new(HTTPAccount)
		if v.Address != nil {
			account.Username = v.Username
			account.Password = v.Password
		} else {
			if err := json.Unmarshal(rawUser, account); err != nil {
				return nil, errors.New("failed to parse HTTP account").Base(err)
			}
		}
		user.Account = serial.ToTypedMessage(buildHTTPAccount(account))
		server.User = user
	}
	config.Server = server
	config.Header = make([]*http.Header, 0, 32)
	for key, value := range v.Headers {
		config.Header = append(config.Header, &http.Header{
			Key:   key,
			Value: value,
		})
	}
	return config, nil
}
