// Code that is not generated: the settings of this package are plain Go values,
// and they say the name a configuration written for the API calls them.

package dns

import (
	"fmt"
	geodata "github.com/xtls/xray-core/common/geodata"
	net "github.com/xtls/xray-core/common/net"
)

// QueryStrategy is one of the values these settings take.
type QueryStrategy int32

const (
	QueryStrategy_USE_IP QueryStrategy = 0
	QueryStrategy_USE_IP4 QueryStrategy = 1
	QueryStrategy_USE_IP6 QueryStrategy = 2
	QueryStrategy_USE_SYS QueryStrategy = 3
)

func (x QueryStrategy) String() string {
	switch x {
	case QueryStrategy_USE_IP:
		return "USE_IP"
	case QueryStrategy_USE_IP4:
		return "USE_IP4"
	case QueryStrategy_USE_IP6:
		return "USE_IP6"
	case QueryStrategy_USE_SYS:
		return "USE_SYS"
	default:
		return fmt.Sprint(int32(x))
	}
}

// NameServer is settings this package reads.
type NameServer struct {
	Address *net.Endpoint `json:"address,omitempty"`
	ClientIp []byte `json:"clientIp,omitempty"`
	SkipFallback bool `json:"skipFallback,omitempty"`
	Domain []*geodata.DomainRule `json:"domain,omitempty"`
	ExpectedIp []*geodata.IPRule `json:"expectedIp,omitempty"`
	QueryStrategy QueryStrategy `json:"queryStrategy,omitempty"`
	ActPrior bool `json:"actPrior,omitempty"`
	Tag string `json:"tag,omitempty"`
	TimeoutMs uint64 `json:"timeoutMs,omitempty"`
	DisableCache *bool `json:"disableCache,omitempty"`
	ServeStale *bool `json:"serveStale,omitempty"`
	ServeExpiredTTL *uint32 `json:"serveExpiredTTL,omitempty"`
	FinalQuery bool `json:"finalQuery,omitempty"`
	UnexpectedIp []*geodata.IPRule `json:"unexpectedIp,omitempty"`
	ActUnprior bool `json:"actUnprior,omitempty"`
	PolicyID uint32 `json:"policyID,omitempty"`
	Id string `json:"id,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*NameServer) SettingsName() string {
	return "xray.app.dns.NameServer"
}

// Config is settings this package reads.
type Config struct {
	NameServer []*NameServer `json:"nameServer,omitempty"`
	ClientIp []byte `json:"clientIp,omitempty"`
	StaticHosts []*Config_HostMapping `json:"staticHosts,omitempty"`
	Tag string `json:"tag,omitempty"`
	DisableCache bool `json:"disableCache,omitempty"`
	ServeStale bool `json:"serveStale,omitempty"`
	ServeExpiredTTL uint32 `json:"serveExpiredTTL,omitempty"`
	QueryStrategy QueryStrategy `json:"queryStrategy,omitempty"`
	DisableFallback bool `json:"disableFallback,omitempty"`
	DisableFallbackIfMatch bool `json:"disableFallbackIfMatch,omitempty"`
	EnableParallelQuery bool `json:"enableParallelQuery,omitempty"`
	Script string `json:"script,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Config) SettingsName() string {
	return "xray.app.dns.Config"
}

// Config_HostMapping is settings this package reads.
type Config_HostMapping struct {
	Domain *geodata.DomainRule `json:"domain,omitempty"`
	Ip [][]byte `json:"ip,omitempty"`
	ProxiedDomain string `json:"proxiedDomain,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Config_HostMapping) SettingsName() string {
	return "xray.app.dns.Config_HostMapping"
}
