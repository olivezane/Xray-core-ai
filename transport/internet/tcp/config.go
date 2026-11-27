package tcp

import (
)

// NewConfig returns the settings a stream settings entry naming this transport
// decodes to when it carries no settings for it.
func NewConfig() *Config {
	return new(Config)
}
