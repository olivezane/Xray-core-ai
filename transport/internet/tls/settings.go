// Code that is not generated: the settings of this package are plain Go values,
// and they say the name a configuration written for the API calls them.

package tls

import (
	"fmt"
	internet "github.com/xtls/xray-core/transport/internet"
)

// Certificate_Usage is one of the values these settings take.
type Certificate_Usage int32

const (
	Certificate_ENCIPHERMENT Certificate_Usage = 0
	Certificate_AUTHORITY_VERIFY Certificate_Usage = 1
	Certificate_AUTHORITY_ISSUE Certificate_Usage = 2
)

func (x Certificate_Usage) String() string {
	switch x {
	case Certificate_ENCIPHERMENT:
		return "Certificate_ENCIPHERMENT"
	case Certificate_AUTHORITY_VERIFY:
		return "Certificate_AUTHORITY_VERIFY"
	case Certificate_AUTHORITY_ISSUE:
		return "Certificate_AUTHORITY_ISSUE"
	default:
		return fmt.Sprint(int32(x))
	}
}

// Certificate is settings this package reads.
type Certificate struct {
	Certificate []byte `json:"certificate,omitempty"`
	Key []byte `json:"key,omitempty"`
	Usage Certificate_Usage `json:"usage,omitempty"`
	OcspStapling uint64 `json:"ocspStapling,omitempty"`
	CertificatePath string `json:"certificatePath,omitempty"`
	KeyPath string `json:"keyPath,omitempty"`
	OneTimeLoading bool `json:"OneTimeLoading,omitempty"`
	BuildChain bool `json:"buildChain,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Certificate) SettingsName() string {
	return "xray.transport.internet.tls.Certificate"
}

// Config is settings this package reads.
type Config struct {
	Certificate []*Certificate `json:"certificate,omitempty"`
	ServerName string `json:"serverName,omitempty"`
	NextProtocol []string `json:"nextProtocol,omitempty"`
	EnableSessionResumption bool `json:"enableSessionResumption,omitempty"`
	DisableSystemRoot bool `json:"disableSystemRoot,omitempty"`
	MinVersion string `json:"minVersion,omitempty"`
	MaxVersion string `json:"maxVersion,omitempty"`
	CipherSuites string `json:"cipherSuites,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
	RejectUnknownSni bool `json:"rejectUnknownSni,omitempty"`
	MasterKeyLog string `json:"masterKeyLog,omitempty"`
	CurvePreferences []string `json:"curvePreferences,omitempty"`
	VerifyPeerCertByName []string `json:"verifyPeerCertByName,omitempty"`
	EchServerKeys []byte `json:"echServerKeys,omitempty"`
	EchConfigList string `json:"echConfigList,omitempty"`
	EchSocketSettings *internet.SocketConfig `json:"echSocketSettings,omitempty"`
	PinnedPeerCertSha256 [][]byte `json:"pinnedPeerCertSha256,omitempty"`
	UseSystemCa bool `json:"useSystemCa,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Config) SettingsName() string {
	return "xray.transport.internet.tls.Config"
}
