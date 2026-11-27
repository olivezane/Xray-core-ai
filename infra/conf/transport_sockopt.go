package conf

import (
	"math"
	"strings"

	"github.com/xtls/xray-core/common/errors"
	coreconfig "github.com/xtls/xray-core/core/config"
	"github.com/xtls/xray-core/transport/internet"
)

// CustomSockoptConfig is one raw socket option of the configuration model.
type CustomSockoptConfig = coreconfig.CustomSockopt

// HappyEyeballsConfig is the dual stack dial settings of the configuration model.
type HappyEyeballsConfig = coreconfig.HappyEyeballs

// SocketConfig is the "sockopt" section of the configuration. It carries the
// model type and adds the conversion to the message, where the three states of
// TFO and the strategy names become their enum values.
type SocketConfig struct {
	coreconfig.Socket
}

// Build implements Buildable.
func (c *SocketConfig) Build() (*internet.SocketConfig, error) {
	tfo := int32(0) // don't invoke setsockopt() for TFO
	if c.TFO != nil {
		switch v := c.TFO.(type) {
		case bool:
			if v {
				tfo = 256
			} else {
				tfo = -1 // TFO need to be disabled
			}
		case float64:
			tfo = int32(math.Min(v, math.MaxInt32))
		default:
			return nil, errors.New("tcpFastOpen: only boolean and integer value is acceptable")
		}
	}
	var tproxy internet.SocketConfig_TProxyMode
	switch strings.ToLower(c.TProxy) {
	case "tproxy":
		tproxy = internet.SocketConfig_TProxy
	case "redirect":
		tproxy = internet.SocketConfig_Redirect
	default:
		tproxy = internet.SocketConfig_Off
	}

	dStrategy := internet.DomainStrategy_AS_IS
	switch strings.ToLower(c.DomainStrategy) {
	case "asis", "":
		dStrategy = internet.DomainStrategy_AS_IS
	case "useip":
		dStrategy = internet.DomainStrategy_USE_IP
	case "useipv4":
		dStrategy = internet.DomainStrategy_USE_IP4
	case "useipv6":
		dStrategy = internet.DomainStrategy_USE_IP6
	case "useipv4v6":
		dStrategy = internet.DomainStrategy_USE_IP46
	case "useipv6v4":
		dStrategy = internet.DomainStrategy_USE_IP64
	case "forceip":
		dStrategy = internet.DomainStrategy_FORCE_IP
	case "forceipv4":
		dStrategy = internet.DomainStrategy_FORCE_IP4
	case "forceipv6":
		dStrategy = internet.DomainStrategy_FORCE_IP6
	case "forceipv4v6":
		dStrategy = internet.DomainStrategy_FORCE_IP46
	case "forceipv6v4":
		dStrategy = internet.DomainStrategy_FORCE_IP64
	default:
		return nil, errors.New("unsupported domain strategy: ", c.DomainStrategy)
	}

	var customSockopts []*internet.CustomSockopt

	for _, copt := range c.CustomSockopt {
		customSockopt := &internet.CustomSockopt{
			System:  copt.System,
			Network: copt.Network,
			Level:   copt.Level,
			Opt:     copt.Opt,
			Value:   copt.Value,
			Type:    copt.Type,
		}
		customSockopts = append(customSockopts, customSockopt)
	}

	addressPortStrategy := internet.AddressPortStrategy_None
	switch strings.ToLower(c.AddressPortStrategy) {
	case "none", "":
		addressPortStrategy = internet.AddressPortStrategy_None
	case "srvportonly":
		addressPortStrategy = internet.AddressPortStrategy_SrvPortOnly
	case "srvaddressonly":
		addressPortStrategy = internet.AddressPortStrategy_SrvAddressOnly
	case "srvportandaddress":
		addressPortStrategy = internet.AddressPortStrategy_SrvPortAndAddress
	case "txtportonly":
		addressPortStrategy = internet.AddressPortStrategy_TxtPortOnly
	case "txtaddressonly":
		addressPortStrategy = internet.AddressPortStrategy_TxtAddressOnly
	case "txtportandaddress":
		addressPortStrategy = internet.AddressPortStrategy_TxtPortAndAddress
	default:
		return nil, errors.New("unsupported address and port strategy: ", c.AddressPortStrategy)
	}

	happyEyeballs := &internet.HappyEyeballsConfig{Interleave: 1, PrioritizeIpv6: false, TryDelayMs: 0, MaxConcurrentTry: 4}
	if c.HappyEyeballsSettings != nil {
		happyEyeballs.PrioritizeIpv6 = c.HappyEyeballsSettings.PrioritizeIPv6
		happyEyeballs.Interleave = c.HappyEyeballsSettings.Interleave
		happyEyeballs.TryDelayMs = c.HappyEyeballsSettings.TryDelayMs
		happyEyeballs.MaxConcurrentTry = c.HappyEyeballsSettings.MaxConcurrentTry
	}

	return &internet.SocketConfig{
		Mark:                 c.Mark,
		Tfo:                  tfo,
		Tproxy:               tproxy,
		DomainStrategy:       dStrategy,
		AcceptProxyProtocol:  c.AcceptProxyProtocol,
		DialerProxy:          c.DialerProxy,
		TcpKeepAliveInterval: c.TCPKeepAliveInterval,
		TcpKeepAliveIdle:     c.TCPKeepAliveIdle,
		TcpCongestion:        c.TCPCongestion,
		TcpWindowClamp:       c.TCPWindowClamp,
		TcpMaxSeg:            c.TCPMaxSeg,
		Penetrate:            c.Penetrate,
		TcpUserTimeout:       c.TCPUserTimeout,
		V6Only:               c.V6only,
		Interface:            c.Interface,
		TcpMptcp:             c.TcpMptcp,
		CustomSockopt:        customSockopts,
		AddressPortStrategy:  addressPortStrategy,
		HappyEyeballs:        happyEyeballs,
		TrustedXForwardedFor: c.TrustedXForwardedFor,
	}, nil
}
