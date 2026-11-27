package config

import (
	"encoding/json"

	"github.com/xtls/xray-core/transport/internet/finalmask/realm"
)

// TCP is the settings of the tcp transport.
type TCP struct {
	// HeaderConfig is raw JSON, because the header kinds are polymorphic.
	HeaderConfig        json.RawMessage `json:"header"`
	AcceptProxyProtocol bool            `json:"acceptProxyProtocol"`
}

// KCP is the settings of the mkcp transport.
type KCP struct {
	Mtu              *uint32         `json:"mtu"`
	Tti              *uint32         `json:"tti"`
	UpCap            *uint32         `json:"uplinkCapacity"`
	DownCap          *uint32         `json:"downlinkCapacity"`
	CwndMultiplier   *uint32         `json:"cwndMultiplier"`
	MaxSendingWindow *uint32         `json:"maxSendingWindow"`
	HeaderConfig     json.RawMessage `json:"header"`
	Seed             *string         `json:"seed"`
}

// WebSocket is the settings of the websocket transport.
type WebSocket struct {
	Host                string            `json:"host"`
	Path                string            `json:"path"`
	Headers             map[string]string `json:"headers"`
	AcceptProxyProtocol bool              `json:"acceptProxyProtocol"`
	HeartbeatPeriod     uint32            `json:"heartbeatPeriod"`
}

// GRPC is the settings of the grpc transport.
type GRPC struct {
	Authority           string `json:"authority"`
	ServiceName         string `json:"serviceName"`
	MultiMode           bool   `json:"multiMode"`
	IdleTimeout         int32  `json:"idle_timeout"`
	HealthCheckTimeout  int32  `json:"health_check_timeout"`
	PermitWithoutStream bool   `json:"permit_without_stream"`
	InitialWindowsSize  int32  `json:"initial_windows_size"`
	UserAgent           string `json:"user_agent"`
}

// HttpUpgrade is the settings of the httpupgrade transport.
type HttpUpgrade struct {
	Host                string            `json:"host"`
	Path                string            `json:"path"`
	Headers             map[string]string `json:"headers"`
	AcceptProxyProtocol bool              `json:"acceptProxyProtocol"`
}

// Masquerade is what a HYSTERIA inbound answers with when it does not speak the
// protocol.
type Masquerade struct {
	Type string `json:"type"`
	Dir  string `json:"dir"`

	Url         string `json:"url"`
	RewriteHost bool   `json:"rewriteHost"`
	XForwarded  bool   `json:"xForwarded"`
	Insecure    bool   `json:"insecure"`

	Content    string            `json:"content"`
	Headers    map[string]string `json:"headers"`
	StatusCode int32             `json:"statusCode"`
}

// Hysteria is the settings of the hysteria transport.
type Hysteria struct {
	Version        int32      `json:"version"`
	Auth           string     `json:"auth"`
	UdpIdleTimeout int64      `json:"udpIdleTimeout"`
	Masquerade     Masquerade `json:"masquerade"`
}

// MasqueWarp is the WARP account of the masque transport.
type MasqueWarp struct {
	PrivateKey string   `json:"privateKey"`
	PublicKey  string   `json:"publicKey"`
	Address    []string `json:"address"`
}

// Xmux is the multiplex window of an HTTP based transport.
type Xmux struct {
	MaxConcurrency   Int32Range `json:"maxConcurrency"`
	MaxConnections   Int32Range `json:"maxConnections"`
	CMaxReuseTimes   Int32Range `json:"cMaxReuseTimes"`
	HMaxRequestTimes Int32Range `json:"hMaxRequestTimes"`
	HMaxReusableSecs Int32Range `json:"hMaxReusableSecs"`
	HKeepAlivePeriod int64      `json:"hKeepAlivePeriod"`
}

// XDriveS3 is the S3 account of the xdrive transport.
type XDriveS3 struct {
	Endpoint  string `json:"endpoint"`
	Region    string `json:"region"`
	Bucket    string `json:"bucket"`
	PathStyle *bool  `json:"pathStyle"`
}

// XDrive is the settings of the xdrive transport.
type XDrive struct {
	RemoteFolder      string          `json:"remoteFolder"`
	Service           string          `json:"service"`
	Secrets           []string        `json:"secrets"`
	SegmentBytes      uint32          `json:"segmentBytes"`
	FlushIntervalMs   uint32          `json:"flushIntervalMs"`
	PollIntervalMs    uint32          `json:"pollIntervalMs"`
	MaxPollIntervalMs uint32          `json:"maxPollIntervalMs"`
	SessionTTLSeconds uint32          `json:"sessionTtlSeconds"`
	Concurrency       uint32          `json:"concurrency"`
	EagerWindowMs     uint32          `json:"eagerWindowMs"`
	HoleTimeoutMs     uint32          `json:"holeTimeoutMs"`
	Template          json.RawMessage `json:"template"`
	S3                *XDriveS3       `json:"s3"`
	Xmux              Xmux            `json:"xmux"`
}

// LimitFallback limits the bandwidth of a fallback.
type LimitFallback struct {
	AfterBytes       uint64
	BytesPerSec      uint64
	BurstBytesPerSec uint64
}

// Reality is the REALITY security setting.
type Reality struct {
	MasterKeyLog string          `json:"masterKeyLog"`
	Show         bool            `json:"show"`
	Target       json.RawMessage `json:"target"`
	Dest         json.RawMessage `json:"dest"`
	Type         string          `json:"type"`
	Xver         uint64          `json:"xver"`
	ServerNames  []string        `json:"serverNames"`
	PrivateKey   string          `json:"privateKey"`
	MinClientVer string          `json:"minClientVer"`
	MaxClientVer string          `json:"maxClientVer"`
	MaxTimeDiff  uint64          `json:"maxTimeDiff"`
	ShortIds     []string        `json:"shortIds"`
	Mldsa65Seed  string          `json:"mldsa65Seed"`

	LimitFallbackUpload   LimitFallback `json:"limitFallbackUpload"`
	LimitFallbackDownload LimitFallback `json:"limitFallbackDownload"`

	Fingerprint   string `json:"fingerprint"`
	ServerName    string `json:"serverName"`
	Password      string `json:"password"`
	PublicKey     string `json:"publicKey"`
	ShortId       string `json:"shortId"`
	Mldsa65Verify string `json:"mldsa65Verify"`
	SpiderX       string `json:"spiderX"`
}

// TLSCertificate is one certificate of a TLS setting.
type TLSCertificate struct {
	CertFile       string   `json:"certificateFile"`
	CertStr        []string `json:"certificate"`
	KeyFile        string   `json:"keyFile"`
	KeyStr         []string `json:"key"`
	Usage          string   `json:"usage"`
	OcspStapling   uint64   `json:"ocspStapling"`
	OneTimeLoading bool     `json:"oneTimeLoading"`
	BuildChain     bool     `json:"buildChain"`
}

// TLS is the TLS security setting.
type TLS struct {
	AllowInsecure           bool         `json:"allowInsecure"`
	Certs                   []*TLSCertificate `json:"certificates"`
	ServerName              string       `json:"serverName"`
	ALPN                    *StringList  `json:"alpn"`
	EnableSessionResumption bool         `json:"enableSessionResumption"`
	DisableSystemRoot       bool         `json:"disableSystemRoot"`
	MinVersion              string       `json:"minVersion"`
	MaxVersion              string       `json:"maxVersion"`
	CipherSuites            string       `json:"cipherSuites"`
	Fingerprint             string       `json:"fingerprint"`
	RejectUnknownSNI        bool         `json:"rejectUnknownSni"`
	CurvePreferences        *StringList  `json:"curvePreferences"`
	MasterKeyLog            string       `json:"masterKeyLog"`
	PinnedPeerCertSha256    string       `json:"pinnedPeerCertSha256"`
	VerifyPeerCertByName    string       `json:"verifyPeerCertByName"`
	ECHServerKeys           string       `json:"echServerKeys"`
	ECHConfigList           string       `json:"echConfigList"`
	ECHSocketSettings       *Socket       `json:"echSockopt"`
	UseSystemCA             bool         `json:"useSystemCA"`
}

// CustomSockopt is one raw socket option of a configuration.
type CustomSockopt struct {
	System  string `json:"system"`
	Network string `json:"network"`
	Level   string `json:"level"`
	Opt     string `json:"opt"`
	Value   string `json:"value"`
	Type    string `json:"type"`
}

// HappyEyeballs is the dual stack dial settings of a configuration.
type HappyEyeballs struct {
	PrioritizeIPv6   bool   `json:"prioritizeIPv6"`
	TryDelayMs       uint64 `json:"tryDelayMs"`
	Interleave       uint32 `json:"interleave"`
	MaxConcurrentTry uint32 `json:"maxConcurrentTry"`
}

// UnmarshalJSON implements encoding/json.Unmarshaler: a happy eyeballs section
// may leave fields out, which keep their defaults.
func (h *HappyEyeballs) UnmarshalJSON(data []byte) error {
	inner := struct {
		PrioritizeIPv6   bool   `json:"prioritizeIPv6"`
		TryDelayMs       uint64 `json:"tryDelayMs"`
		Interleave       uint32 `json:"interleave"`
		MaxConcurrentTry uint32 `json:"maxConcurrentTry"`
	}{PrioritizeIPv6: false, Interleave: 1, TryDelayMs: 0, MaxConcurrentTry: 4}
	if err := json.Unmarshal(data, &inner); err != nil {
		return err
	}
	h.PrioritizeIPv6 = inner.PrioritizeIPv6
	h.TryDelayMs = inner.TryDelayMs
	h.Interleave = inner.Interleave
	h.MaxConcurrentTry = inner.MaxConcurrentTry
	return nil
}

// Socket is the "sockopt" section: how the sockets of a connection are set up.
type Socket struct {
	Mark                  int32              `json:"mark"`
	// TFO is a boolean or a number, so it decodes into a generic value that the
	// boundary turns into the three states of the setting.
	TFO                   interface{}        `json:"tcpFastOpen"`
	TProxy                string             `json:"tproxy"`
	AcceptProxyProtocol   bool               `json:"acceptProxyProtocol"`
	DomainStrategy        string             `json:"domainStrategy"`
	DialerProxy           string             `json:"dialerProxy"`
	TCPKeepAliveInterval  int32              `json:"tcpKeepAliveInterval"`
	TCPKeepAliveIdle      int32              `json:"tcpKeepAliveIdle"`
	TCPCongestion         string             `json:"tcpCongestion"`
	TCPWindowClamp        int32              `json:"tcpWindowClamp"`
	TCPMaxSeg             int32              `json:"tcpMaxSeg"`
	Penetrate             bool               `json:"penetrate"`
	TCPUserTimeout        int32              `json:"tcpUserTimeout"`
	V6only                bool               `json:"v6only"`
	Interface             string             `json:"interface"`
	TcpMptcp              bool               `json:"tcpMptcp"`
	CustomSockopt         []*CustomSockopt   `json:"customSockopt"`
	AddressPortStrategy   string             `json:"addressPortStrategy"`
	HappyEyeballsSettings *HappyEyeballs     `json:"happyEyeballs"`
	TrustedXForwardedFor  []string           `json:"trustedXForwardedFor"`
}

// ProxyProtocol is the PROXY protocol header settings of a mask.
type ProxyProtocol struct {
	Version uint32 `json:"version"`
}

// Filter blocks or allows source addresses.
type Filter struct {
	BlockedSourceIPs []string `json:"blockedSourceIPs"`
	AllowedSourceIPs []string `json:"allowedSourceIPs"`
}

// MkcpLegacy is the legacy mkcp header setting.
type MkcpLegacy struct {
	Header string `json:"header"`
	Value  string `json:"value"`
}

// Salamander is the salamander mask setting.
type Salamander struct {
	Password   string     `json:"password"`
	PacketSize Int32Range `json:"packetSize"`
}

// HTTPRequest is the request header of an HTTP transport.
type HTTPRequest struct {
	Version string                 `json:"version"`
	Method  string                 `json:"method"`
	Path    StringList             `json:"path"`
	Headers map[string]*StringList `json:"headers"`
}

// HTTPResponse is the response header of an HTTP transport.
type HTTPResponse struct {
	Version string                 `json:"version"`
	Status  string                 `json:"status"`
	Reason  string                 `json:"reason"`
	Headers map[string]*StringList `json:"headers"`
}

// HTTPHeader is the header setting of the tcp transport.
type HTTPHeader struct {
	Request  HTTPRequest  `json:"request"`
	Response HTTPResponse `json:"response"`
}

// Xicmp is the xicmp mask setting.
type Xicmp struct {
	DGRAM bool     `json:"dgram"`
	IPs   []string `json:"ips"`
}

// UDPHop is the udphop mask setting.
type UDPHop struct {
	Mode        string     `json:"mode"`
	Interval    Int32Range `json:"interval"`
	RemoteIPs   []string   `json:"remoteIPs"`
	RemotePorts PortList   `json:"remotePorts"`
}

// FragmentMask is the fragment mask setting.
type FragmentMask struct {
	Packets  string       `json:"packets"`
	Length   Int32Range   `json:"length"`
	Delay    Int32Range   `json:"delay"`
	Lengths  []Int32Range `json:"lengths"`
	Delays   []Int32Range `json:"delays"`
	MaxSplit Int32Range   `json:"maxSplit"`
}

// XDNSDomain is one domain group of the xdns mask.
type XDNSDomain struct {
	Names      []string `json:"names"`
	LenLimit   int32    `json:"lenLimit"`
	LabelLimit int32    `json:"labelLimit"`
	Types      []int32  `json:"types"`
	Edns0      int32    `json:"edns0"`
	Alphabet   string   `json:"alphabet"`
}

// NoiseItem is one noise item of the noise mask.
type NoiseItem struct {
	Rand      Int32Range      `json:"rand"`
	RandRange *Int32Range     `json:"randRange"`
	Type      string          `json:"type"`
	Packet    json.RawMessage `json:"packet"`
	Delay     Int32Range      `json:"delay"`
}

// NoiseMask is the noise mask setting.
type NoiseMask struct {
	Reset  Int32Range  `json:"reset"`
	Noise  []NoiseItem `json:"noise"`
	Script string      `json:"script"`
}

// Sudoku is the sudoku mask setting.
type Sudoku struct {
	Password string `json:"password"`
	ASCII    string `json:"ascii"`

	CustomTable       string   `json:"customTable"`
	LegacyCustomTable string   `json:"custom_table"`
	CustomTables      []string `json:"customTables"`
	LegacyCustomSets  []string `json:"custom_tables"`

	PaddingMin       uint32 `json:"paddingMin"`
	LegacyPaddingMin uint32 `json:"padding_min"`
	PaddingMax       uint32 `json:"paddingMax"`
	LegacyPaddingMax uint32 `json:"padding_max"`
}

// XMCPaddingTurn is one turn of the xmc padding schedule.
type XMCPaddingTurn struct {
	Length    string `json:"length"`
	Direction string `json:"direction"`
	Delay     string `json:"delay,omitempty"`
}

// XMCYggdrasil is the Yggdrasil account of the xmc mask.
type XMCYggdrasil struct {
	API      string `json:"api"`
	Username string `json:"username"`
	Password string `json:"password"`
	Player   string `json:"player"`
}

// XMCProfile is one Minecraft profile of the xmc mask.
type XMCProfile struct {
	Username          string `json:"username"`
	UUID              string `json:"uuid"`
	TexturesValue     string `json:"texturesValue"`
	TexturesSignature string `json:"texturesSignature"`
}

// XMC is the xmc mask setting.
type XMC struct {
	Hostname  string           `json:"hostname"`
	Profiles  []XMCProfile     `json:"profiles"`
	Password  string           `json:"password"`
	Paddings  []XMCPaddingTurn `json:"paddings,omitempty"`
	Yggdrasil *XMCYggdrasil    `json:"yggdrasil,omitempty"`
	NoPadding bool             `json:"noPadding,omitempty"`
}

// Realm is the realm mask setting.
type Realm struct {
	Url         string            `json:"url"`
	StunServers []string          `json:"stunServers"`
	TlsConfig   *TLS              `json:"tlsConfig"`
	IPMode      string            `json:"ipMode"`
	PortMapping *realm.PortMapping `json:"portMapping"`
}

// Bandwidth is a bandwidth written as a string with a unit, e.g. "60 mbps". The
// parse of the unit is part of the configuration boundary, not of the model.
type Bandwidth string

// QuicParams is the QUIC setting of the transports that speak QUIC.
type QuicParams struct {
	Congestion                    string `json:"congestion"`
	Debug                         bool   `json:"debug"`
	BbrProfile                    string `json:"bbrProfile"`
	BrutalUp                      Bandwidth `json:"brutalUp"`
	BrutalDown                    Bandwidth `json:"brutalDown"`
	BrutalDisableLossCompensation bool   `json:"brutalDisableLossCompensation"`
	InitStreamReceiveWindow       uint64 `json:"initStreamReceiveWindow"`
	MaxStreamReceiveWindow        uint64 `json:"maxStreamReceiveWindow"`
	InitConnectionReceiveWindow   uint64 `json:"initConnectionReceiveWindow"`
	MaxConnectionReceiveWindow    uint64 `json:"maxConnectionReceiveWindow"`
	MaxIdleTimeout                int64  `json:"maxIdleTimeout"`
	KeepAlivePeriod               int64  `json:"keepAlivePeriod"`
	DisablePathMTUDiscovery       bool   `json:"disablePathMTUDiscovery"`
	DisableChromeParrot           bool   `json:"disableChromeParrot"`
	DisableGSO                    bool   `json:"disableGSO"`
	MaxIncomingStreams            int64  `json:"maxIncomingStreams"`
	DisableStatelessReset         bool   `json:"disableStatelessReset"`
}

// XDNSResolver is one resolver of the xdns mask.
type XDNSResolver struct {
	Addrs      []string    `json:"addrs"`
	Worker     int32       `json:"worker"`
	TlsConfig  *TLS        `json:"tlsConfig"`
	QuicConfig *QuicParams `json:"quicConfig"`
}

// XDNS is the xdns mask setting.
type XDNS struct {
	Domains   []XDNSDomain   `json:"domains"`
	Resolvers []XDNSResolver `json:"resolvers"`
	ExtraPoll int32          `json:"extraPoll"`
}

// TCPItem is one item of the header-custom mask for TCP.
type TCPItem struct {
	Delay     Int32Range       `json:"delay"`
	Rand      int32            `json:"rand"`
	RandRange *Int32Range      `json:"randRange"`
	Capture   string           `json:"capture"`
	Type      string           `json:"type"`
	Reuse     string           `json:"reuse"`
	Transform *CustomTransform `json:"transform"`
	Packet    json.RawMessage  `json:"packet"`
}

// CustomTransform is one transform of a header-custom item.
type CustomTransform struct {
	Op   string               `json:"op"`
	Args []CustomTransformArg `json:"args"`
}

// CustomTransformArg is one argument of a transform.
type CustomTransformArg struct {
	Type      string           `json:"type"`
	Bytes     json.RawMessage  `json:"bytes"`
	U64       *uint64          `json:"u64"`
	Reuse     string           `json:"reuse"`
	Metadata  string           `json:"metadata"`
	Transform *CustomTransform `json:"transform"`
}

// UDPItem is one item of the header-custom mask for UDP.
type UDPItem struct {
	Rand      int32            `json:"rand"`
	RandRange *Int32Range      `json:"randRange"`
	Capture   string           `json:"capture"`
	Type      string           `json:"type"`
	Reuse     string           `json:"reuse"`
	Transform *CustomTransform `json:"transform"`
	Packet    json.RawMessage  `json:"packet"`
}

// HeaderCustomTCP is the header-custom mask setting for TCP.
type HeaderCustomTCP struct {
	Clients [][]TCPItem `json:"clients"`
	Servers [][]TCPItem `json:"servers"`
	Errors  [][]TCPItem `json:"errors"`
}

// HeaderCustomUDP is the header-custom mask setting for UDP.
type HeaderCustomUDP struct {
	Mode   string    `json:"mode"`
	Client []UDPItem `json:"client"`
	Server []UDPItem `json:"server"`
}

// Masque is the settings of the masque transport.
type Masque struct {
	Host    string            `json:"host"`
	Path    string            `json:"path"`
	User    string            `json:"user"`
	Pass    string            `json:"pass"`
	Headers map[string]string `json:"headers"`
	Warp    *MasqueWarp       `json:"warp"`
	Xmux    Xmux              `json:"xmux"`
}

// SplitHTTP is the settings of the splithttp transport.
type SplitHTTP struct {
	Host                 string            `json:"host"`
	Path                 string            `json:"path"`
	Mode                 string            `json:"mode"`
	Headers              map[string]string `json:"headers"`
	XPaddingBytes        Int32Range        `json:"xPaddingBytes"`
	XPaddingObfsMode     bool              `json:"xPaddingObfsMode"`
	XPaddingKey          string            `json:"xPaddingKey"`
	XPaddingHeader       string            `json:"xPaddingHeader"`
	XPaddingPlacement    string            `json:"xPaddingPlacement"`
	XPaddingMethod       string            `json:"xPaddingMethod"`
	UplinkHTTPMethod     string            `json:"uplinkHTTPMethod"`
	SessionIDPlacement   string            `json:"sessionIDPlacement"`
	SessionIDKey         string            `json:"sessionIDKey"`
	SessionIDTable       string            `json:"sessionIDTable"`
	SessionIDLength      Int32Range        `json:"sessionIDLength"`
	SeqPlacement         string            `json:"seqPlacement"`
	SeqKey               string            `json:"seqKey"`
	UplinkDataPlacement  string            `json:"uplinkDataPlacement"`
	UplinkDataKey        string            `json:"uplinkDataKey"`
	UplinkChunkSize      Int32Range        `json:"uplinkChunkSize"`
	NoGRPCHeader         bool              `json:"noGRPCHeader"`
	NoSSEHeader          bool              `json:"noSSEHeader"`
	ScMaxEachPostBytes   Int32Range        `json:"scMaxEachPostBytes"`
	ScMinPostsIntervalMs Int32Range        `json:"scMinPostsIntervalMs"`
	ScMaxBufferedPosts   int64             `json:"scMaxBufferedPosts"`
	ScStreamUpServerSecs Int32Range        `json:"scStreamUpServerSecs"`
	ServerMaxHeaderBytes int32             `json:"serverMaxHeaderBytes"`
	Xmux                 Xmux              `json:"xmux"`
	DownloadSettings     *Stream           `json:"downloadSettings"`
	H2Params             json.RawMessage   `json:"h2Params"`
	Extra                json.RawMessage   `json:"extra"`
}

// Stream is the "streamSettings" section: the transport, its security and its
// socket settings.
type Stream struct {
	Address             *Address        `json:"address"`
	Port                uint16          `json:"port"`
	Method              *TransportProtocol `json:"method"`
	Network             *TransportProtocol `json:"network"`
	Security            string          `json:"security"`
	FinalMask           *FinalMask      `json:"finalmask"`
	TLSSettings         *TLS            `json:"tlsSettings"`
	REALITYSettings     *Reality        `json:"realitySettings"`
	RAWSettings         *TCP            `json:"rawSettings"`
	TCPSettings         *TCP            `json:"tcpSettings"`
	XHTTPSettings       *SplitHTTP      `json:"xhttpSettings"`
	SplitHTTPSettings   *SplitHTTP      `json:"splithttpSettings"`
	KCPSettings         *KCP            `json:"kcpSettings"`
	GRPCSettings        *GRPC           `json:"grpcSettings"`
	WSSettings          *WebSocket      `json:"wsSettings"`
	HTTPUPGRADESettings *HttpUpgrade    `json:"httpupgradeSettings"`
	HysteriaSettings    *Hysteria       `json:"hysteriaSettings"`
	MASQUESettings      *Masque         `json:"masqueSettings"`
	XDRIVESettings      *XDrive         `json:"xdriveSettings"`
	SocketSettings      *Socket         `json:"sockopt"`
}

// TransportProtocol is a transport name as a configuration writes it: "raw",
// "tcp", "xhttp", "ws", and so on. The boundary turns it into the protocol name
// of the transport.
type TransportProtocol string

// Mask is one entry of a mask list: its type and the raw settings of that type.
type Mask struct {
	Type     string           `json:"type"`
	Settings *json.RawMessage `json:"settings"`
}

// FinalMask is the "finalmask" section of stream settings.
type FinalMask struct {
	Tcp        []Mask      `json:"tcp"`
	Udp        []Mask      `json:"udp"`
	QuicParams *QuicParams `json:"quicParams"`
}
