// Code that is not generated: the settings of this package are plain Go values,
// and they say the name a configuration written for the API calls them.

package account

// Account is settings this package reads.
type Account struct {
	Auth string `json:"auth,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Account) SettingsName() string {
	return "xray.proxy.hysteria.account.Account"
}
