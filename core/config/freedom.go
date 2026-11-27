package config

// Freedom is the settings of a "freedom" outbound.
type Freedom struct {
	TargetStrategy string    `json:"targetStrategy"`
	DomainStrategy string    `json:"domainStrategy"`
	Redirect       string    `json:"redirect"`
	UserLevel      uint32    `json:"userLevel"`
	Fragment       *Fragment `json:"fragment"`
	Noise          *Noise    `json:"noise"`
	Noises         []*Noise  `json:"noises"`
	ProxyProtocol  uint32    `json:"proxyProtocol"`
	IPsBlocked     *StringList `json:"ipsBlocked"`
	// Firewall and FinalRules block or shape traffic by destination.
	Firewall   []*FreedomFirewall `json:"firewall"`
	FinalRules []*FreedomFirewall `json:"finalRules"`
}

// Fragment splits the stream of a freedom outbound into pieces.
type Fragment struct {
	Packets  string      `json:"packets"`
	Length   *Int32Range `json:"length"`
	Interval *Int32Range `json:"interval"`
	MaxSplit *Int32Range `json:"maxSplit"`
}

// Noise adds noise to the traffic of a freedom outbound.
type Noise struct {
	Type    string      `json:"type"`
	Packet  string      `json:"packet"`
	Delay   *Int32Range `json:"delay"`
	ApplyTo string      `json:"applyTo"`
}

// FreedomFirewall is one firewall rule of a freedom outbound.
type FreedomFirewall struct {
	Action     string        `json:"action"`
	Network    *NetworkList  `json:"network"`
	Port       *PortList     `json:"port"`
	IP         *StringList   `json:"ip"`
	Delay      *Int32Range   `json:"delay"`
	LogLevel   string        `json:"logLevel"`
	Reply      *FreedomReply `json:"reply"`
	BlockDelay *Int32Range   `json:"blockDelay"`
}

// FreedomReply is what a blocking rule answers with.
type FreedomReply struct {
	Type               string `json:"type"`
	CustomResponseData string `json:"customResponseData"`
}
