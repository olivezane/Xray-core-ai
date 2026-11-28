// Code that is not generated: the settings of this package are plain Go values,
// and they say the name a configuration written for the API calls them.

package vless

import "github.com/xtls/xray-core/app/proxyman"

// Reverse is the reverse proxy part of an account.
type Reverse struct {
	Tag      string                   `json:"tag,omitempty"`
	Sniffing *proxyman.SniffingConfig `json:"sniffing,omitempty"`
}

// Account is the identity of a vless user.
type Account struct {
	// ID of the account, in the form of a UUID, e.g., "66ad4540-b58c-4ad2-9926-ea63445a9b57".
	Id string `json:"id,omitempty"`
	// Flow settings. May be "xtls-rprx-vision".
	Flow string `json:"flow,omitempty"`

	Encryption string `json:"encryption,omitempty"`
	XorMode    uint32 `json:"xorMode,omitempty"`
	Seconds    uint32 `json:"seconds,omitempty"`
	Padding    string `json:"padding,omitempty"`

	Reverse *Reverse `json:"reverse,omitempty"`

	Testpre  uint32   `json:"testpre,omitempty"`
	Testseed []uint32 `json:"testseed,omitempty"`
}

// Multiplex is the multiplexing part of an outbound.
type Multiplex struct {
	Concurrency     int32  `json:"concurrency,omitempty"`
	XudpConcurrency int32  `json:"xudpConcurrency,omitempty"`
	XudpProxyUDP443 string `json:"xudpProxyUDP443,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Account) SettingsName() string {
	return "xray.proxy.vless.Account"
}

func (*Multiplex) SettingsName() string {
	return "xray.proxy.vless.Multiplex"
}
