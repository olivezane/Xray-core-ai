package inbound

// GetDefaultValue returns default settings of DefaultConfig.
func (c *Config) GetDefaultValue() *DefaultConfig {
	if c.Default == nil {
		return &DefaultConfig{}
	}
	return c.Default
}
