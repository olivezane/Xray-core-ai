// Code that is not generated: the settings of this package are plain Go values,
// and they say the name a configuration written for the API calls them.

package policy

// Second is settings this package reads.
type Second struct {
	Value uint32 `json:"value,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Second) SettingsName() string {
	return "xray.app.policy.Second"
}

// Policy is settings this package reads.
type Policy struct {
	Timeout *Policy_Timeout `json:"timeout,omitempty"`
	Stats *Policy_Stats `json:"stats,omitempty"`
	Buffer *Policy_Buffer `json:"buffer,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Policy) SettingsName() string {
	return "xray.app.policy.Policy"
}

// SystemPolicy is settings this package reads.
type SystemPolicy struct {
	Stats *SystemPolicy_Stats `json:"stats,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*SystemPolicy) SettingsName() string {
	return "xray.app.policy.SystemPolicy"
}

// Config is settings this package reads.
type Config struct {
	Level map[uint32]*Policy `json:"level,omitempty"`
	System *SystemPolicy `json:"system,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Config) SettingsName() string {
	return "xray.app.policy.Config"
}

// Policy_Timeout is settings this package reads.
type Policy_Timeout struct {
	Handshake *Second `json:"handshake,omitempty"`
	ConnectionIdle *Second `json:"connectionIdle,omitempty"`
	UplinkOnly *Second `json:"uplinkOnly,omitempty"`
	DownlinkOnly *Second `json:"downlinkOnly,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Policy_Timeout) SettingsName() string {
	return "xray.app.policy.Policy_Timeout"
}

// Policy_Stats is settings this package reads.
type Policy_Stats struct {
	UserUplink bool `json:"userUplink,omitempty"`
	UserDownlink bool `json:"userDownlink,omitempty"`
	UserOnline bool `json:"userOnline,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Policy_Stats) SettingsName() string {
	return "xray.app.policy.Policy_Stats"
}

// Policy_Buffer is settings this package reads.
type Policy_Buffer struct {
	Connection int32 `json:"connection,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Policy_Buffer) SettingsName() string {
	return "xray.app.policy.Policy_Buffer"
}

// SystemPolicy_Stats is settings this package reads.
type SystemPolicy_Stats struct {
	InboundUplink bool `json:"inboundUplink,omitempty"`
	InboundDownlink bool `json:"inboundDownlink,omitempty"`
	OutboundUplink bool `json:"outboundUplink,omitempty"`
	OutboundDownlink bool `json:"outboundDownlink,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*SystemPolicy_Stats) SettingsName() string {
	return "xray.app.policy.SystemPolicy_Stats"
}
