// Code that is not generated: the settings of this package are plain Go values,
// and they say the name a configuration written for the API calls them.

package splithttp

import (
	internet "github.com/xtls/xray-core/transport/internet"
)

// RangeConfig is settings this package reads.
type RangeConfig struct {
	From int32 `json:"from,omitempty"`
	To int32 `json:"to,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*RangeConfig) SettingsName() string {
	return "xray.transport.internet.splithttp.RangeConfig"
}

// XmuxConfig is settings this package reads.
type XmuxConfig struct {
	MaxConcurrency *RangeConfig `json:"maxConcurrency,omitempty"`
	MaxConnections *RangeConfig `json:"maxConnections,omitempty"`
	CMaxReuseTimes *RangeConfig `json:"cMaxReuseTimes,omitempty"`
	HMaxRequestTimes *RangeConfig `json:"hMaxRequestTimes,omitempty"`
	HMaxReusableSecs *RangeConfig `json:"hMaxReusableSecs,omitempty"`
	HKeepAlivePeriod int64 `json:"hKeepAlivePeriod,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*XmuxConfig) SettingsName() string {
	return "xray.transport.internet.splithttp.XmuxConfig"
}

// Config is settings this package reads.
type Config struct {
	Host string `json:"host,omitempty"`
	Path string `json:"path,omitempty"`
	Mode string `json:"mode,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	XPaddingBytes *RangeConfig `json:"xPaddingBytes,omitempty"`
	NoGRPCHeader bool `json:"noGRPCHeader,omitempty"`
	NoSSEHeader bool `json:"noSSEHeader,omitempty"`
	ScMaxEachPostBytes *RangeConfig `json:"scMaxEachPostBytes,omitempty"`
	ScMinPostsIntervalMs *RangeConfig `json:"scMinPostsIntervalMs,omitempty"`
	ScMaxBufferedPosts int64 `json:"scMaxBufferedPosts,omitempty"`
	ScStreamUpServerSecs *RangeConfig `json:"scStreamUpServerSecs,omitempty"`
	Xmux *XmuxConfig `json:"xmux,omitempty"`
	DownloadSettings *internet.StreamConfig `json:"downloadSettings,omitempty"`
	XPaddingObfsMode bool `json:"xPaddingObfsMode,omitempty"`
	XPaddingKey string `json:"xPaddingKey,omitempty"`
	XPaddingHeader string `json:"xPaddingHeader,omitempty"`
	XPaddingPlacement string `json:"xPaddingPlacement,omitempty"`
	XPaddingMethod string `json:"xPaddingMethod,omitempty"`
	UplinkHTTPMethod string `json:"uplinkHTTPMethod,omitempty"`
	SessionIDPlacement string `json:"sessionIDPlacement,omitempty"`
	SessionIDKey string `json:"sessionIDKey,omitempty"`
	SeqPlacement string `json:"seqPlacement,omitempty"`
	SeqKey string `json:"seqKey,omitempty"`
	UplinkDataPlacement string `json:"uplinkDataPlacement,omitempty"`
	UplinkDataKey string `json:"uplinkDataKey,omitempty"`
	UplinkChunkSize *RangeConfig `json:"uplinkChunkSize,omitempty"`
	ServerMaxHeaderBytes int32 `json:"serverMaxHeaderBytes,omitempty"`
	SessionIDTable string `json:"sessionIDTable,omitempty"`
	SessionIDLength *RangeConfig `json:"sessionIDLength,omitempty"`
	H2Params []byte `json:"h2Params,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Config) SettingsName() string {
	return "xray.transport.internet.splithttp.Config"
}
