// Code that is not generated: the settings of this package are plain Go values,
// and they say the name a configuration written for the API calls them.

package http

// Header is settings this package reads.
type Header struct {
	Name string `json:"name,omitempty"`
	Value []string `json:"value,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Header) SettingsName() string {
	return "xray.transport.internet.headers.http.Header"
}

// Version is settings this package reads.
type Version struct {
	Value string `json:"value,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Version) SettingsName() string {
	return "xray.transport.internet.headers.http.Version"
}

// Method is settings this package reads.
type Method struct {
	Value string `json:"value,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Method) SettingsName() string {
	return "xray.transport.internet.headers.http.Method"
}

// RequestConfig is settings this package reads.
type RequestConfig struct {
	Version *Version `json:"version,omitempty"`
	Method *Method `json:"method,omitempty"`
	Uri []string `json:"uri,omitempty"`
	Header []*Header `json:"header,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*RequestConfig) SettingsName() string {
	return "xray.transport.internet.headers.http.RequestConfig"
}

// Status is settings this package reads.
type Status struct {
	Code string `json:"code,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Status) SettingsName() string {
	return "xray.transport.internet.headers.http.Status"
}

// ResponseConfig is settings this package reads.
type ResponseConfig struct {
	Version *Version `json:"version,omitempty"`
	Status *Status `json:"status,omitempty"`
	Header []*Header `json:"header,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*ResponseConfig) SettingsName() string {
	return "xray.transport.internet.headers.http.ResponseConfig"
}

// Config is settings this package reads.
type Config struct {
	Request *RequestConfig `json:"request,omitempty"`
	Response *ResponseConfig `json:"response,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Config) SettingsName() string {
	return "xray.transport.internet.headers.http.Config"
}
