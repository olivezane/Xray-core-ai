// Package config is the configuration model of Xray in plain Go types.
//
// A configuration is these structs decoded from JSON; protobuf is not the model.
// It survives only at the boundary: the API exchanges protobuf messages, and the
// conversion to them lives where that boundary is crossed (infra/conf), not here.
//
// The types carry units in their names, because that is the one thing a plain
// struct cannot express and the protobuf messages left implicit: a timeout is in
// seconds, a connection buffer is in kilobytes.
package config

// Policy is the policy of one level.
type Policy struct {
	// Handshake is how long a handshake may take, in seconds. Nil means the
	// default of the level.
	Handshake *uint32 `json:"handshake"`
	// ConnectionIdle is how long an idle connection is kept, in seconds.
	ConnectionIdle *uint32 `json:"connIdle"`
	// UplinkOnly is how long a connection stays after its uplink closed, in
	// seconds.
	UplinkOnly *uint32 `json:"uplinkOnly"`
	// DownlinkOnly is how long a connection stays after its downlink closed, in
	// seconds.
	DownlinkOnly *uint32 `json:"downlinkOnly"`
	// StatsUserUplink enables per-user uplink accounting.
	StatsUserUplink bool `json:"statsUserUplink"`
	// StatsUserDownlink enables per-user downlink accounting.
	StatsUserDownlink bool `json:"statsUserDownlink"`
	// StatsUserOnline enables online-user accounting.
	StatsUserOnline bool `json:"statsUserOnline"`
	// BufferSize is the connection buffer of this level, in kilobytes. Nil keeps
	// the default, a negative value disables buffering.
	BufferSize *int32 `json:"bufferSize"`
}

// SystemPolicy is the policy of the instance as a whole.
type SystemPolicy struct {
	StatsInboundUplink    bool `json:"statsInboundUplink"`
	StatsInboundDownlink  bool `json:"statsInboundDownlink"`
	StatsOutboundUplink   bool `json:"statsOutboundUplink"`
	StatsOutboundDownlink bool `json:"statsOutboundDownlink"`
}

// PolicySet is the "policy" section of a configuration.
type PolicySet struct {
	Levels map[uint32]*Policy `json:"levels"`
	System *SystemPolicy      `json:"system"`
}
