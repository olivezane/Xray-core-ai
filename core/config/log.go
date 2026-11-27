package config

// Log is the "log" section: where the two logs of an instance go and how much of
// the error log is kept. The strings are the user's, resolved to the log types
// and severities of the instance at the boundary.
type Log struct {
	// AccessLog is "none", a file path, or empty for the default.
	AccessLog string `json:"access"`
	// ErrorLog is "none", a file path, or empty for the default.
	ErrorLog string `json:"error"`
	// LogLevel is "debug", "info", "warning", "error" or "none".
	LogLevel string `json:"logLevel"`
	// DNSLog enables logging of DNS queries.
	DNSLog bool `json:"dnsLog"`
	// MaskAddress replaces the address of an access log entry.
	MaskAddress string `json:"maskAddress"`
}

// Version is the "version" section: the versions of the core this configuration
// accepts.
type Version struct {
	MinVersion string `json:"min"`
	MaxVersion string `json:"max"`
}
