package masque

import (
	"time"

)

const protocolName = "masque"

const DefaultPath = "/.well-known/masque/ip/*/*/"

func (c *Config) keepAlivePeriod() time.Duration {
	if c.Xmux == nil {
		return 0
	}
	return time.Duration(c.Xmux.HKeepAlivePeriod) * time.Second
}

// NewConfig returns the settings a stream settings entry naming this transport
// decodes to when it carries no settings for it.
func NewConfig() *Config {
	return &Config{
		Path: DefaultPath,
	}
}
