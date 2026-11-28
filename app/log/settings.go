// Code that is not generated: the settings of this package are plain Go values,
// and they say the name a configuration written for the API calls them.

package log

import (
	"fmt"
	log "github.com/xtls/xray-core/common/log"
)

// LogType is one of the values these settings take.
type LogType int32

const (
	LogType_None LogType = 0
	LogType_Console LogType = 1
	LogType_File LogType = 2
	LogType_Event LogType = 3
)

func (x LogType) String() string {
	switch x {
	case LogType_None:
		return "None"
	case LogType_Console:
		return "Console"
	case LogType_File:
		return "File"
	case LogType_Event:
		return "Event"
	default:
		return fmt.Sprint(int32(x))
	}
}

// Config is settings this package reads.
type Config struct {
	ErrorLogType LogType `json:"errorLogType,omitempty"`
	ErrorLogLevel log.Severity `json:"errorLogLevel,omitempty"`
	ErrorLogPath string `json:"errorLogPath,omitempty"`
	AccessLogType LogType `json:"accessLogType,omitempty"`
	AccessLogPath string `json:"accessLogPath,omitempty"`
	EnableDnsLog bool `json:"enableDnsLog,omitempty"`
	MaskAddress string `json:"maskAddress,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Config) SettingsName() string {
	return "xray.app.log.Config"
}
