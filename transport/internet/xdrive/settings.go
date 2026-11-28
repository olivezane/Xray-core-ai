// Code that is not generated: the settings of this package are plain Go values,
// and they say the name a configuration written for the API calls them.

package xdrive

import (
	splithttp "github.com/xtls/xray-core/transport/internet/splithttp"
)

// Config is settings this package reads.
type Config struct {
	RemoteFolder string `json:"remoteFolder,omitempty"`
	Service string `json:"service,omitempty"`
	Secrets []string `json:"secrets,omitempty"`
	SegmentBytes uint32 `json:"segmentBytes,omitempty"`
	FlushIntervalMs uint32 `json:"flushIntervalMs,omitempty"`
	PollIntervalMs uint32 `json:"pollIntervalMs,omitempty"`
	MaxPollIntervalMs uint32 `json:"maxPollIntervalMs,omitempty"`
	SessionTtlSeconds uint32 `json:"sessionTtlSeconds,omitempty"`
	Concurrency uint32 `json:"concurrency,omitempty"`
	EagerWindowMs uint32 `json:"eagerWindowMs,omitempty"`
	HoleTimeoutMs uint32 `json:"holeTimeoutMs,omitempty"`
	Template string `json:"template,omitempty"`
	Xmux *splithttp.XmuxConfig `json:"xmux,omitempty"`
	S3 *S3Config `json:"s3,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Config) SettingsName() string {
	return "xray.transport.internet.xdrive.Config"
}

// S3Config is settings this package reads.
type S3Config struct {
	Endpoint string `json:"endpoint,omitempty"`
	Region string `json:"region,omitempty"`
	Bucket string `json:"bucket,omitempty"`
	PathStyle bool `json:"pathStyle,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*S3Config) SettingsName() string {
	return "xray.transport.internet.xdrive.S3Config"
}
