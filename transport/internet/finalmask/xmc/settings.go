// Code that is not generated: the settings of this package are plain Go values,
// and they say the name a configuration written for the API calls them.

package xmc

// Profile is settings this package reads.
type Profile struct {
	Username string `json:"username,omitempty"`
	Uuid []byte `json:"uuid,omitempty"`
	TexturesValue string `json:"texturesValue,omitempty"`
	TexturesSignature string `json:"texturesSignature,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Profile) SettingsName() string {
	return "xray.transport.internet.finalmask.xmc.Profile"
}

// Yggdrasil is settings this package reads.
type Yggdrasil struct {
	Api string `json:"api,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Player string `json:"player,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Yggdrasil) SettingsName() string {
	return "xray.transport.internet.finalmask.xmc.Yggdrasil"
}

// Config is settings this package reads.
type Config struct {
	Password string `json:"password,omitempty"`
	RsaPrivateKey []byte `json:"rsaPrivateKey,omitempty"`
	RsaPublicKey []byte `json:"rsaPublicKey,omitempty"`
	Hostname string `json:"hostname,omitempty"`
	Profiles []*Profile `json:"profiles,omitempty"`
	Paddings []*Padding `json:"paddings,omitempty"`
	Yggdrasil *Yggdrasil `json:"yggdrasil,omitempty"`
	NoPadding bool `json:"noPadding,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Config) SettingsName() string {
	return "xray.transport.internet.finalmask.xmc.Config"
}

// Padding is settings this package reads.
type Padding struct {
	LengthMin int64 `json:"lengthMin,omitempty"`
	LengthMax int64 `json:"lengthMax,omitempty"`
	Direction int32 `json:"direction,omitempty"`
	DelayMin int64 `json:"delayMin,omitempty"`
	DelayMax int64 `json:"delayMax,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Padding) SettingsName() string {
	return "xray.transport.internet.finalmask.xmc.Padding"
}
