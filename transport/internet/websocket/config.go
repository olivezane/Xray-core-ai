package websocket

import (
	"net/http"

	"github.com/xtls/xray-core/common/utils")

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

func (c *Config) GetRequestHeader() http.Header {
	header := http.Header{}
	for k, v := range c.Header {
		header.Add(k, v)
	}
	utils.TryDefaultHeadersWith(header, "ws")
	return header
}

// NewConfig returns the settings a stream settings entry naming this transport
// decodes to when it carries no settings for it.
func NewConfig() *Config {
	return new(Config)
}
