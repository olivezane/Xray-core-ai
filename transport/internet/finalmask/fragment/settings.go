// Code that is not generated: the settings of this package are plain Go values,
// and they say the name a configuration written for the API calls them.

package fragment

// Config is settings this package reads.
type Config struct {
	PacketsFrom int64 `json:"packetsFrom,omitempty"`
	PacketsTo int64 `json:"packetsTo,omitempty"`
	MaxSplitMin int64 `json:"maxSplitMin,omitempty"`
	MaxSplitMax int64 `json:"maxSplitMax,omitempty"`
	LengthsMin []int64 `json:"lengthsMin,omitempty"`
	LengthsMax []int64 `json:"lengthsMax,omitempty"`
	DelaysMin []int64 `json:"delaysMin,omitempty"`
	DelaysMax []int64 `json:"delaysMax,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Config) SettingsName() string {
	return "xray.transport.internet.finalmask.fragment.Config"
}
