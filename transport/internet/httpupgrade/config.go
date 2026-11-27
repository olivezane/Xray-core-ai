package httpupgrade


func (c *Config) GetNormalizedPath() string {
	path := c.Path
	if path == "" {
		return "/"
	}
	if path[0] != '/' {
		return "/" + path
	}
	return path
}

// NewConfig returns the settings a stream settings entry naming this transport
// decodes to when it carries no settings for it.
func NewConfig() *Config {
	return new(Config)
}
