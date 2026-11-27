package conf

import (
	"strings"

	coreconfig "github.com/xtls/xray-core/core/config"

	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/protocol"
)

// The types below are the configuration model of core/config. This file keeps the
// names the configuration loader and its tests use, and adds the conversion of
// each of them to the protobuf message the instance consumes.
type (
	StringList  = coreconfig.StringList
	Address     = coreconfig.Address
	Network     = coreconfig.Network
	NetworkList = coreconfig.NetworkList
	PortRange   = coreconfig.PortRange
	PortList    = coreconfig.PortList
	User        = coreconfig.User
	Int32Range  = coreconfig.Int32Range
)

// NewStringList returns a StringList holding raw.
func NewStringList(raw []string) *StringList {
	return coreconfig.NewStringList(raw)
}

// BuildAddress converts an address of the model to its message.
func BuildAddress(v *coreconfig.Address) *net.IPOrDomain {
	if v == nil {
		return nil
	}
	return net.NewIPOrDomain(v.Address)
}

// BuildNetwork converts a network name of the model to its message value.
func BuildNetwork(v coreconfig.Network) net.Network {
	switch strings.ToLower(string(v)) {
	case "tcp":
		return net.Network_TCP
	case "udp":
		return net.Network_UDP
	case "unix":
		return net.Network_UNIX
	default:
		return net.Network_Unknown
	}
}

// BuildNetworkList converts a network list of the model to its message value.
func BuildNetworkList(v *coreconfig.NetworkList) []net.Network {
	if v == nil {
		return []net.Network{net.Network_TCP}
	}
	list := make([]net.Network, 0, len(*v))
	for _, network := range *v {
		list = append(list, BuildNetwork(network))
	}
	return list
}

// BuildPortRange converts a port range of the model to its message.
func BuildPortRange(v *coreconfig.PortRange) *net.PortRange {
	return &net.PortRange{From: v.From, To: v.To}
}

// BuildPortList converts a port list of the model to its message.
func BuildPortList(list *coreconfig.PortList) *net.PortList {
	portList := new(net.PortList)
	for _, r := range list.Range {
		portList.Range = append(portList.Range, BuildPortRange(&r))
	}
	return portList
}

// BuildUser converts a user of the model to its message.
func BuildUser(v *coreconfig.User) *protocol.User {
	return &protocol.User{
		Email: v.EmailString,
		Level: uint32(v.LevelByte),
	}
}

// parse helpers kept for the configuration loader.
var (
	parseIntPort       = coreconfig.ParseIntPort
	parseStringPort    = coreconfig.ParseStringPort
	parseJSONStringPort = coreconfig.ParseJSONStringPort
	ParseRangeString   = coreconfig.ParseRangeString
	splitFromSecondDash = coreconfig.SplitFromSecondDash
)
