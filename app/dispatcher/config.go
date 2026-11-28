package dispatcher

// SessionConfig is the settings of a session. It carries nothing today; it is
// kept because a configuration names it.
type SessionConfig struct{}

// Config is the settings of the dispatcher.
type Config struct {
	Settings *SessionConfig `json:"settings,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Config) SettingsName() string {
	return "xray.app.dispatcher.Config"
}
