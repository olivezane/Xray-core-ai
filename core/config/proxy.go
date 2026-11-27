package config

// Sniffing is the "sniffing" section of an inbound: which protocols to detect on
// the connection and which destinations to skip.
type Sniffing struct {
	Enabled         bool       `json:"enabled"`
	DestOverride    StringList `json:"destOverride"`
	DomainsExcluded StringList `json:"domainsExcluded"`
	IPsExcluded     StringList `json:"ipsExcluded"`
	MetadataOnly    bool       `json:"metadataOnly"`
	RouteOnly       bool       `json:"routeOnly"`
}

// Blackhole is the settings of a "blackhole" outbound.
type Blackhole struct {
	Response *Response `json:"response"`
}

// Response is what a blackhole outbound answers with.
type Response struct {
	// Type is "none", "http" or "custom".
	Type string `json:"type"`
	// CustomResponseData is the base64 payload of a custom response.
	CustomResponseData string `json:"customResponseData"`
}

// Loopback is the settings of a "loopback" inbound.
type Loopback struct {
	InboundTag string    `json:"inboundTag"`
	Sniffing   *Sniffing `json:"sniffing"`
}

// Dokodemo is the settings of a "dokodemo-door" inbound.
type Dokodemo struct {
	AllowedNetwork *NetworkList      `json:"allowedNetwork"`
	RewriteAddress *Address          `json:"rewriteAddress"`
	RewritePort    uint16            `json:"rewritePort"`
	Network        *NetworkList      `json:"network"`
	Address        *Address          `json:"address"`
	Port           uint16            `json:"port"`
	PortMap        map[string]string `json:"portMap"`
	FollowRedirect bool              `json:"followRedirect"`
	UserLevel      uint32            `json:"userLevel"`
}
