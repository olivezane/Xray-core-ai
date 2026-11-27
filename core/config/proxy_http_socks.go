package config

import "encoding/json"

// SocksAccount is one user of a socks inbound.
type SocksAccount struct {
	Username string `json:"user"`
	Password string `json:"pass"`
}

// SocksServer is the settings of a socks inbound.
type SocksServer struct {
	AuthMethod string          `json:"auth"`
	Users      []*SocksAccount `json:"users"`
	Accounts   []*SocksAccount `json:"accounts"`
	UDP        bool            `json:"udp"`
	Host       *Address        `json:"ip"`
	UserLevel  uint32          `json:"userLevel"`
}

// SocksRemote is one endpoint of a socks outbound.
type SocksRemote struct {
	Address *Address          `json:"address"`
	Port    uint16            `json:"port"`
	Users   []json.RawMessage `json:"users"`
}

// SocksClient is the settings of a socks outbound.
type SocksClient struct {
	Address  *Address       `json:"address"`
	Port     uint16         `json:"port"`
	Level    uint32         `json:"level"`
	Email    string         `json:"email"`
	Username string         `json:"user"`
	Password string         `json:"pass"`
	Servers  []*SocksRemote `json:"servers"`
}

// HTTPAccount is one user of an HTTP inbound.
type HTTPAccount struct {
	Username string `json:"user"`
	Password string `json:"pass"`
}

// HTTPServer is the settings of an HTTP inbound.
type HTTPServer struct {
	Users       []*HTTPAccount `json:"users"`
	Accounts    []*HTTPAccount `json:"accounts"`
	Transparent bool           `json:"allowTransparent"`
	UserLevel   uint32         `json:"userLevel"`
}

// HTTPRemote is one endpoint of an HTTP outbound.
type HTTPRemote struct {
	Address *Address          `json:"address"`
	Port    uint16            `json:"port"`
	Users   []json.RawMessage `json:"users"`
}

// HTTPClient is the settings of an HTTP outbound.
type HTTPClient struct {
	Address  *Address          `json:"address"`
	Port     uint16            `json:"port"`
	Level    uint32            `json:"level"`
	Email    string            `json:"email"`
	Username string            `json:"user"`
	Password string            `json:"pass"`
	Servers  []*HTTPRemote     `json:"servers"`
	Headers  map[string]string `json:"headers"`
}

// TrojanTarget is one server of a trojan outbound.
type TrojanTarget struct {
	Address  *Address `json:"address"`
	Port     uint16   `json:"port"`
	Level    byte     `json:"level"`
	Email    string   `json:"email"`
	Password string   `json:"password"`
	Flow     string   `json:"flow"`
}

// TrojanClient is the settings of a trojan outbound.
type TrojanClient struct {
	Address  *Address        `json:"address"`
	Port     uint16          `json:"port"`
	Level    byte            `json:"level"`
	Email    string          `json:"email"`
	Password string          `json:"password"`
	Flow     string          `json:"flow"`
	Servers  []*TrojanTarget `json:"servers"`
}

// TrojanFallback is one fallback of a trojan inbound.
type TrojanFallback struct {
	Name string          `json:"name"`
	Alpn string          `json:"alpn"`
	Path string          `json:"path"`
	Type string          `json:"type"`
	Dest json.RawMessage `json:"dest"`
	Xver uint64          `json:"xver"`
}

// TrojanUser is one user of a trojan inbound.
type TrojanUser struct {
	Password string `json:"password"`
	Level    byte   `json:"level"`
	Email    string `json:"email"`
	Flow     string `json:"flow"`
}

// TrojanServer is the settings of a trojan inbound.
type TrojanServer struct {
	Users     []*TrojanUser     `json:"users"`
	Clients   []*TrojanUser     `json:"clients"`
	Fallbacks []*TrojanFallback `json:"fallbacks"`
}

// VLessFallback is one fallback of a VLESS inbound.
type VLessFallback struct {
	Name string          `json:"name"`
	Alpn string          `json:"alpn"`
	Path string          `json:"path"`
	Type string          `json:"type"`
	Dest json.RawMessage `json:"dest"`
	Xver uint64          `json:"xver"`
}

// VLessInbound is the settings of a VLESS inbound.
type VLessInbound struct {
	Users      []json.RawMessage `json:"users"`
	Clients    []json.RawMessage `json:"clients"`
	Decryption string            `json:"decryption"`
	Fallbacks  []*VLessFallback  `json:"fallbacks"`
	Flow       string            `json:"flow"`
	Testseed   []uint32          `json:"testseed"`
}

// VLessReverse is the reverse setting of a VLESS outbound.
type VLessReverse struct {
	Tag      string    `json:"tag"`
	Sniffing *Sniffing `json:"sniffing"`
}

// Multiplex is the multiplex setting of a VLESS or VMess outbound.
type Multiplex struct {
	Concurrency     int16  `json:"concurrency"`
	XudpConcurrency int16  `json:"xudpConcurrency"`
	XudpProxyUDP443 string `json:"xudpProxyUDP443"`
}

// VLessVnext is one endpoint of a VLESS outbound.
type VLessVnext struct {
	Address *Address          `json:"address"`
	Port    uint16            `json:"port"`
	Users   []json.RawMessage `json:"users"`
}

// VLessOutbound is the settings of a VLESS outbound.
type VLessOutbound struct {
	Address    *Address        `json:"address"`
	Port       uint16          `json:"port"`
	Level      uint32          `json:"level"`
	Email      string          `json:"email"`
	Id         string          `json:"id"`
	Flow       string          `json:"flow"`
	Seed       string          `json:"seed"`
	Encryption string          `json:"encryption"`
	Multiplex  *Multiplex      `json:"multiplex"`
	Reverse    *VLessReverse   `json:"reverse"`
	Testpre    uint32          `json:"testpre"`
	Testseed   []uint32        `json:"testseed"`
	Vnext      []*VLessVnext   `json:"vnext"`
}

// VMessAccount is one user of a VMess inbound or outbound.
type VMessAccount struct {
	ID          string `json:"id"`
	Security    string `json:"security"`
	Experiments string `json:"experiments"`
}

// VMessDefault is the default settings of a VMess inbound.
type VMessDefault struct {
	Level byte `json:"level"`
}

// VMessInbound is the settings of a VMess inbound.
type VMessInbound struct {
	Users    []json.RawMessage `json:"users"`
	Clients  []json.RawMessage `json:"clients"`
	Defaults *VMessDefault     `json:"default"`
}

// VMessOutboundTarget is one endpoint of a VMess outbound.
type VMessOutboundTarget struct {
	Address *Address          `json:"address"`
	Port    uint16            `json:"port"`
	Users   []json.RawMessage `json:"users"`
}

// VMessOutbound is the settings of a VMess outbound.
type VMessOutbound struct {
	Address     *Address               `json:"address"`
	Port        uint16                 `json:"port"`
	Level       uint32                 `json:"level"`
	Email       string                 `json:"email"`
	ID          string                 `json:"id"`
	Security    string                 `json:"security"`
	Experiments string                 `json:"experiments"`
	Receivers   []*VMessOutboundTarget `json:"vnext"`
	Multiplex   *Multiplex             `json:"multiplex"`
}

// MasqueClient is the settings of a MASQUE outbound.
type MasqueClient struct {
	Address   *Address `json:"address"`
	Port      uint16   `json:"port"`
	RemoteDNS []string `json:"remoteDNS"`
}

// MasqueUser is one user of a MASQUE inbound.
type MasqueUser struct {
	Pass  string `json:"pass"`
	Level uint32 `json:"level"`
	Email string `json:"email"`
}

// MasqueServer is the settings of a MASQUE inbound.
type MasqueServer struct {
	Users   []*MasqueUser `json:"users"`
	Clients []*MasqueUser `json:"clients"`
	Address []string      `json:"address"`
	MTU     uint32        `json:"mtu"`
}

// HysteriaClient is the settings of a HYSTERIA outbound.
type HysteriaClient struct {
	Version int32    `json:"version"`
	Address *Address `json:"address"`
	Port    uint16   `json:"port"`
}

// HysteriaUser is one user of a HYSTERIA inbound.
type HysteriaUser struct {
	Auth  string `json:"auth"`
	Level uint32 `json:"level"`
	Email string `json:"email"`
}

// HysteriaServer is the settings of a HYSTERIA inbound.
type HysteriaServer struct {
	Version int32           `json:"version"`
	Users   []*HysteriaUser `json:"users"`
	Clients []*HysteriaUser `json:"clients"`
}

// WireGuardPeer is one peer of a WireGuard tunnel.
type WireGuardPeer struct {
	PublicKey    string   `json:"publicKey"`
	PreSharedKey string   `json:"preSharedKey"`
	Endpoint     string   `json:"endpoint"`
	KeepAlive    uint32   `json:"keepAlive"`
	AllowedIPs   []string `json:"allowedIPs,omitempty"`
	Level        uint32   `json:"level"`
	Email        string   `json:"email"`
}

// WireGuard is the settings of a WireGuard tunnel.
type WireGuard struct {
	IsClient bool `json:""`
	// NoKernelTun keeps the tunnel in user space.
	NoKernelTun bool             `json:"noKernelTun"`
	SecretKey   string           `json:"secretKey"`
	Address     []string         `json:"address"`
	Peers       []*WireGuardPeer `json:"peers"`
	MTU         int32            `json:"mtu"`
	Reserved    []byte           `json:"reserved"`
	DNS         []string         `json:"remoteDNS"`
}

// ShadowsocksUser is one user of a shadowsocks inbound.
type ShadowsocksUser struct {
	Cipher   string   `json:"method"`
	Password string   `json:"password"`
	Level    byte     `json:"level"`
	Email    string   `json:"email"`
	Address  *Address `json:"address"`
	Port     uint16   `json:"port"`
}

// ShadowsocksServer is the settings of a shadowsocks inbound.
type ShadowsocksServer struct {
	Cipher      string             `json:"method"`
	Password    string             `json:"password"`
	Level       byte               `json:"level"`
	Email       string             `json:"email"`
	Users       []*ShadowsocksUser `json:"users"`
	Clients     []*ShadowsocksUser `json:"clients"`
	NetworkList *NetworkList       `json:"network"`
}

// ShadowsocksTarget is one server of a shadowsocks outbound.
type ShadowsocksTarget struct {
	Address  *Address `json:"address"`
	Port     uint16   `json:"port"`
	Level    byte     `json:"level"`
	Email    string   `json:"email"`
	Cipher   string   `json:"method"`
	Password string   `json:"password"`
}

// ShadowsocksClient is the settings of a shadowsocks outbound.
type ShadowsocksClient struct {
	Address  *Address             `json:"address"`
	Port     uint16               `json:"port"`
	Level    byte                 `json:"level"`
	Email    string               `json:"email"`
	Cipher   string               `json:"method"`
	Password string               `json:"password"`
	Servers  []*ShadowsocksTarget `json:"servers"`
}

// Tun is the settings of a Tun inbound.
type Tun struct {
	Name                   string   `json:"name"`
	Desc                   string   `json:"desc"`
	GUID                   string   `json:"guid"`
	MTU                    uint32   `json:"mtu"`
	Gateway                []string `json:"gateway"`
	DNS                    []string `json:"dns"`
	UserLevel              uint32   `json:"userLevel"`
	AutoSystemRoutingTable []string `json:"autoSystemRoutingTable"`
	AutoOutboundsInterface *string  `json:"autoOutboundsInterface"`
	AutoSystemDnsToGateway bool     `json:"autoSystemDnsToGateway"`
	AutoSystemWfpBlockLeak []string `json:"autoSystemWfpBlockLeak"`
}

// DNSOutboundRule is one rule of a DNS outbound.
type DNSOutboundRule struct {
	Action string      `json:"action"`
	QType  *PortList   `json:"qType"`
	Domain *StringList `json:"domain"`
	RCode  uint32      `json:"rCode"`
}

// DNSOutbound is the settings of a DNS outbound.
type DNSOutbound struct {
	RewriteNetwork Network             `json:"rewriteNetwork"`
	RewriteAddress *Address            `json:"rewriteAddress"`
	RewritePort    uint16              `json:"rewritePort"`
	Network        Network             `json:"network"`
	Address        *Address            `json:"address"`
	Port           uint16              `json:"port"`
	UserLevel      uint32              `json:"userLevel"`
	Rules          []*DNSOutboundRule  `json:"rules"`
	NonIPQuery     *string             `json:"nonIPQuery"`
	BlockTypes     *[]int32            `json:"blockTypes"`
}
