// Code that is not generated: the settings of this package are plain Go values,
// and they say the name a configuration written for the API calls them.

package xdns

import (
	internet "github.com/xtls/xray-core/transport/internet"
	tls "github.com/xtls/xray-core/transport/internet/tls"
)

// DomainProto is settings this package reads.
type DomainProto struct {
	Name string `json:"name,omitempty"`
	LenLimit int32 `json:"lenLimit,omitempty"`
	LabelLimit int32 `json:"labelLimit,omitempty"`
	Types []int32 `json:"types,omitempty"`
	Edns0 int32 `json:"edns0,omitempty"`
	Alphabet string `json:"alphabet,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*DomainProto) SettingsName() string {
	return "xray.transport.internet.finalmask.xdns.DomainProto"
}

// ResolverProto is settings this package reads.
type ResolverProto struct {
	Type string `json:"type,omitempty"`
	Addr string `json:"addr,omitempty"`
	Worker int32 `json:"worker,omitempty"`
	TlsConfig *tls.Config `json:"tlsConfig,omitempty"`
	QuicConfig *internet.QuicParams `json:"quicConfig,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*ResolverProto) SettingsName() string {
	return "xray.transport.internet.finalmask.xdns.ResolverProto"
}

// Config is settings this package reads.
type Config struct {
	Domains []*DomainProto `json:"domains,omitempty"`
	Resolvers []*ResolverProto `json:"resolvers,omitempty"`
	ExtraPoll int32 `json:"extraPoll,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Config) SettingsName() string {
	return "xray.transport.internet.finalmask.xdns.Config"
}
