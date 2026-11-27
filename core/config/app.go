package config

import (
	"encoding/json"

	"github.com/xtls/xray-core/app/router"
	"github.com/xtls/xray-core/infra/conf/cfgcommon/duration"
)

// API is the management block of a configuration: one port that serves the gRPC
// control service and the HTTP /debug endpoints.
type API struct {
	Tag      string   `json:"tag"`
	Listen   string   `json:"listen"`
	Services []string `json:"services"`
	// Observatory and BurstObservatory configure outbound health checks.
	Observatory      *Observatory      `json:"observatory"`
	BurstObservatory *BurstObservatory `json:"burstObservatory"`
}

// Observatory probes outbounds on a fixed interval.
type Observatory struct {
	SubjectSelector   []string          `json:"subjectSelector"`
	ProbeURL          string            `json:"probeURL"`
	ProbeInterval     duration.Duration `json:"probeInterval"`
	EnableConcurrency bool              `json:"enableConcurrency"`
}

// BurstObservatory probes outbounds in bursts.
type BurstObservatory struct {
	SubjectSelector []string `json:"subjectSelector"`
	// HealthCheck is the pingConfig section.
	HealthCheck *HealthCheck `json:"pingConfig,omitempty"`
}

// HealthCheck is the health check settings used by a burst observatory.
type HealthCheck struct {
	Destination   string            `json:"destination"`
	Connectivity  string            `json:"connectivity"`
	Interval      duration.Duration `json:"interval"`
	SamplingCount int               `json:"sampling"`
	Timeout       duration.Duration `json:"timeout"`
	HttpMethod    string            `json:"httpMethod"`
}

// Geodata is the "geodata" section: which rule assets to keep up to date.
type Geodata struct {
	Cron     *string  `json:"cron"`
	Outbound string   `json:"outbound"`
	Assets   []*Asset `json:"assets"`
}

// Asset is one geodata asset to download or to read from a file.
type Asset struct {
	URL  string `json:"url"`
	File string `json:"file"`
}

// LeastLoadStrategy is the "leastload" balancer strategy: how to price a node,
// which latency baselines to expect, and how many nodes to select.
type LeastLoadStrategy struct {
	// Costs are the weights of the strategy.
	Costs []*router.StrategyWeight `json:"costs,omitempty"`
	// Baselines are the expected latencies, in the unit of the configuration.
	Baselines []duration.Duration `json:"baselines,omitempty"`
	// Expected is how many nodes to select, 0 for the default.
	Expected int32 `json:"expected,omitempty"`
	// MaxRTT filters away nodes slower than this.
	MaxRTT duration.Duration `json:"maxRTT,omitempty"`
	// Tolerance is the acceptable failure rate, between 0 and 1.
	Tolerance float64 `json:"tolerance,omitempty"`
}

// StrategyConfig is the strategy of a balancer: its type and the raw settings that
// the type defines.
type StrategyConfig struct {
	Type     string           `json:"type"`
	Settings *json.RawMessage `json:"settings"`
}

// Balancer is one balancing rule of the "routing" section.
type Balancer struct {
	Tag         string        `json:"tag"`
	Selectors   StringList    `json:"selector"`
	Strategy    StrategyConfig `json:"strategy"`
	FallbackTag string        `json:"fallbackTag"`
}

// Router is the "routing" section: the rules, the domain strategy and the
// balancers of an instance.
type Router struct {
	// RuleList holds one raw JSON rule each, because a rule is polymorphic and
	// is parsed by the boundary.
	RuleList       []json.RawMessage `json:"rules"`
	DomainStrategy *string           `json:"domainStrategy"`
	Balancers      []*Balancer       `json:"balancers"`
	// Script is a routing script, which cannot be combined with rules or a
	// domain strategy.
	Script string `json:"script"`
}

// Inbound is one entry of the "inbounds" list.
type Inbound struct {
	Protocol       string           `json:"protocol"`
	PortList       *PortList        `json:"port"`
	ListenOn       *Address         `json:"listen"`
	Settings       *json.RawMessage `json:"settings"`
	Tag            string           `json:"tag"`
	StreamSetting  *Stream          `json:"streamSettings"`
	SniffingConfig *Sniffing        `json:"sniffing"`
}

// Outbound is one entry of the "outbounds" list.
type Outbound struct {
	Protocol       string           `json:"protocol"`
	SendThrough    *string          `json:"sendThrough"`
	Tag            string           `json:"tag"`
	Settings       *json.RawMessage `json:"settings"`
	StreamSetting  *Stream          `json:"streamSettings"`
	ProxySettings  *json.RawMessage `json:"proxySettings"`
	LegacyMux      json.RawMessage  `json:"mux,omitempty"`
	TargetStrategy string           `json:"targetStrategy"`
}

// Env is the "env" section: environment variables a configuration sets.
type Env map[string]string

// Root is a whole configuration file.
type Root struct {
	// Transport is deprecated: the global transport config moved to
	// streamSettings, and its presence is an error.
	Transport json.RawMessage `json:"transport"`

	Env             Env      `json:"env"`
	LogConfig       *Log     `json:"log"`
	RouterConfig    *Router  `json:"routing"`
	DNSConfig       *DNS     `json:"dns"`
	InboundConfigs  []Inbound  `json:"inbounds"`
	OutboundConfigs []Outbound `json:"outbounds"`
	Policy          *PolicySet `json:"policy"`
	API             *API     `json:"api"`
	// Deprecated: removed blocks, kept only to warn.
	Metrics          json.RawMessage `json:"metrics"`
	Stats            json.RawMessage `json:"stats"`
	Observatory      json.RawMessage `json:"observatory"`
	BurstObservatory json.RawMessage `json:"burstObservatory"`
	FakeDNS          json.RawMessage `json:"fakeDns"`
	Reverse          json.RawMessage `json:"reverse"`
	Version          *Version        `json:"version"`
	Geodata          *Geodata        `json:"geodata"`
}
