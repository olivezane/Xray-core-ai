package config

import (
	"encoding/json"

	"github.com/xtls/xray-core/common/errors"
)

// NameServer is one name server of the "dns" section. It may be written as a
// plain address string or as an object with the fields below.
type NameServer struct {
	ID              string     `json:"id"`
	Address         *Address   `json:"address"`
	ClientIP        *Address   `json:"clientIp"`
	Port            uint16     `json:"port"`
	SkipFallback    bool       `json:"skipFallback"`
	Domains         StringList `json:"domains"`
	ExpectedIPs     StringList `json:"expectedIPs"`
	ExpectIPs       StringList `json:"expectIPs"`
	QueryStrategy   string     `json:"queryStrategy"`
	Tag             string     `json:"tag"`
	TimeoutMs       uint64     `json:"timeoutMs"`
	DisableCache    *bool      `json:"disableCache"`
	ServeStale      *bool      `json:"serveStale"`
	ServeExpiredTTL *uint32    `json:"serveExpiredTTL"`
	FinalQuery      bool       `json:"finalQuery"`
	UnexpectedIPs   StringList `json:"unexpectedIPs"`
}

// UnmarshalJSON implements encoding/json.Unmarshaler: a name server may be a plain
// address, which is the shorthand form of a configuration.
func (c *NameServer) UnmarshalJSON(data []byte) error {
	var address Address
	if err := json.Unmarshal(data, &address); err == nil {
		c.Address = &address
		return nil
	}

	var advanced struct {
		ID              string     `json:"id"`
		Address         *Address   `json:"address"`
		ClientIP        *Address   `json:"clientIp"`
		Port            uint16     `json:"port"`
		SkipFallback    bool       `json:"skipFallback"`
		Domains         StringList `json:"domains"`
		ExpectedIPs     StringList `json:"expectedIPs"`
		ExpectIPs       StringList `json:"expectIPs"`
		QueryStrategy   string     `json:"queryStrategy"`
		Tag             string     `json:"tag"`
		TimeoutMs       uint64     `json:"timeoutMs"`
		DisableCache    *bool      `json:"disableCache"`
		ServeStale      *bool      `json:"serveStale"`
		ServeExpiredTTL *uint32    `json:"serveExpiredTTL"`
		FinalQuery      bool       `json:"finalQuery"`
		UnexpectedIPs   StringList `json:"unexpectedIPs"`
	}
	if err := json.Unmarshal(data, &advanced); err != nil {
		return errors.New("failed to parse name server: ", string(data))
	}
	c.ID = advanced.ID
	c.Address = advanced.Address
	c.ClientIP = advanced.ClientIP
	c.Port = advanced.Port
	c.SkipFallback = advanced.SkipFallback
	c.Domains = advanced.Domains
	c.ExpectedIPs = advanced.ExpectedIPs
	c.ExpectIPs = advanced.ExpectIPs
	c.QueryStrategy = advanced.QueryStrategy
	c.Tag = advanced.Tag
	c.TimeoutMs = advanced.TimeoutMs
	c.DisableCache = advanced.DisableCache
	c.ServeStale = advanced.ServeStale
	c.ServeExpiredTTL = advanced.ServeExpiredTTL
	c.FinalQuery = advanced.FinalQuery
	c.UnexpectedIPs = advanced.UnexpectedIPs
	return nil
}

// HostAddress is one host of the "hosts" table: one address or a list of them.
type HostAddress struct {
	addr  *Address
	addrs []*Address
}

// MarshalJSON implements encoding/json.Marshaler.
func (h *HostAddress) MarshalJSON() ([]byte, error) {
	if (h.addr != nil) != (h.addrs != nil) {
		if h.addr != nil {
			return json.Marshal(h.addr)
		}
		return json.Marshal(h.addrs)
	}
	return nil, errors.New("unexpected config state")
}

// UnmarshalJSON implements encoding/json.Unmarshaler.
func (h *HostAddress) UnmarshalJSON(data []byte) error {
	addr := new(Address)
	var addrs []*Address
	switch {
	case json.Unmarshal(data, &addr) == nil:
		h.addr = addr
	case json.Unmarshal(data, &addrs) == nil:
		h.addrs = addrs
	default:
		return errors.New("invalid address")
	}
	return nil
}

// Addrs returns the addresses of the host, whichever form it was written in.
func (h *HostAddress) Addrs() []*Address {
	if h.addr != nil {
		return []*Address{h.addr}
	}
	return h.addrs
}

// Single reports the one address of a host written as one address.
func (h *HostAddress) Single() (*Address, bool) {
	return h.addr, h.addr != nil
}

// Hosts is the "hosts" table of the "dns" section.
type Hosts struct {
	Hosts map[string]*HostAddress
}

// MarshalJSON implements encoding/json.Marshaler.
func (m *Hosts) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.Hosts)
}

// UnmarshalJSON implements encoding/json.Unmarshaler.
func (m *Hosts) UnmarshalJSON(data []byte) error {
	hosts := make(map[string]*HostAddress)
	if err := json.Unmarshal(data, &hosts); err != nil {
		return errors.New("invalid DNS hosts").Base(err)
	}
	m.Hosts = hosts
	return nil
}

// DNS is the "dns" section of a configuration.
type DNS struct {
	Servers                []*NameServer `json:"servers"`
	Script                 string        `json:"script"`
	Hosts                  *Hosts        `json:"hosts"`
	ClientIP               *Address      `json:"clientIp"`
	Tag                    string        `json:"tag"`
	QueryStrategy          string        `json:"queryStrategy"`
	DisableCache           bool          `json:"disableCache"`
	ServeStale             bool          `json:"serveStale"`
	ServeExpiredTTL        uint32        `json:"serveExpiredTTL"`
	DisableFallback        bool          `json:"disableFallback"`
	DisableFallbackIfMatch bool          `json:"disableFallbackIfMatch"`
	EnableParallelQuery    bool          `json:"enableParallelQuery"`
	UseSystemHosts         bool          `json:"useSystemHosts"`
	FakeDNS                *FakeDNS      `json:"fakeDns"`
}
