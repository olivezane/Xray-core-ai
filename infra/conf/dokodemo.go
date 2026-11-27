package conf

import (
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
	coreconfig "github.com/xtls/xray-core/core/config"
	"github.com/xtls/xray-core/proxy/dokodemo"
	"google.golang.org/protobuf/proto"
)

// DokodemoConfig is the settings of a dokodemo-door inbound.
type DokodemoConfig struct {
	coreconfig.Dokodemo
}

func (v *DokodemoConfig) Build() (proto.Message, error) {
	if v.Network != nil {
		v.AllowedNetwork = v.Network
	}
	if v.Address != nil {
		v.RewriteAddress = v.Address
	}
	if v.Port != 0 {
		v.RewritePort = v.Port
	}
	config := new(dokodemo.Config)
	config.AllowedNetworks = BuildNetworkList(v.AllowedNetwork)
	if v.RewriteAddress != nil {
		config.RewriteAddress = BuildAddress(v.RewriteAddress)
	}
	config.RewritePort = uint32(v.RewritePort)
	config.PortMap = v.PortMap
	for _, v := range config.PortMap {
		if _, _, err := net.SplitHostPort(v); err != nil {
			return nil, errors.New("invalid portMap: ", v).Base(err)
		}
	}
	config.FollowRedirect = v.FollowRedirect
	config.UserLevel = v.UserLevel
	return config, nil
}
