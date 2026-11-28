// Code that is not generated: the settings of this package are plain Go values,
// and they say the name a configuration written for the API calls them.

package reality

// Config is settings this package reads.
type Config struct {
	Show bool `json:"show,omitempty"`
	Dest string `json:"dest,omitempty"`
	Type string `json:"type,omitempty"`
	Xver uint64 `json:"xver,omitempty"`
	ServerNames []string `json:"serverNames,omitempty"`
	PrivateKey []byte `json:"privateKey,omitempty"`
	MinClientVer []byte `json:"minClientVer,omitempty"`
	MaxClientVer []byte `json:"maxClientVer,omitempty"`
	MaxTimeDiff uint64 `json:"maxTimeDiff,omitempty"`
	ShortIds [][]byte `json:"shortIds,omitempty"`
	Mldsa65Seed []byte `json:"mldsa65Seed,omitempty"`
	LimitFallbackUpload *LimitFallback `json:"limitFallbackUpload,omitempty"`
	LimitFallbackDownload *LimitFallback `json:"limitFallbackDownload,omitempty"`
	Fingerprint string `json:"Fingerprint,omitempty"`
	ServerName string `json:"serverName,omitempty"`
	PublicKey []byte `json:"publicKey,omitempty"`
	ShortId []byte `json:"shortId,omitempty"`
	Mldsa65Verify []byte `json:"mldsa65Verify,omitempty"`
	SpiderX string `json:"spiderX,omitempty"`
	SpiderY []int64 `json:"spiderY,omitempty"`
	MasterKeyLog string `json:"masterKeyLog,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Config) SettingsName() string {
	return "xray.transport.internet.reality.Config"
}

// LimitFallback is settings this package reads.
type LimitFallback struct {
	AfterBytes uint64 `json:"afterBytes,omitempty"`
	BytesPerSec uint64 `json:"bytesPerSec,omitempty"`
	BurstBytesPerSec uint64 `json:"burstBytesPerSec,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*LimitFallback) SettingsName() string {
	return "xray.transport.internet.reality.LimitFallback"
}
