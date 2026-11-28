// Code that is not generated: the settings of this package are plain Go values,
// and they say the name a configuration written for the API calls them.

package command

// Config is the settings of the log service. It carries nothing today; it is kept
// because a configuration names it.
type Config struct{}

// SettingsName is what a configuration written for the API calls these settings.
func (*Config) SettingsName() string {
	return "xray.app.log.command.Config"
}
