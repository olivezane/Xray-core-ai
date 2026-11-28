package core

// Config is a configuration of an instance: the apps it starts, the inbounds it
// accepts on and the outbounds it dials with.
//
// The settings of an app, an inbound or an outbound are the plain Go values the
// configuration model carries (core/config). Protobuf is not the shape of a
// configuration: it survives only where a value crosses the API, and the
// conversion happens at that boundary.
type Config struct {
	Inbound   []*InboundHandlerConfig
	Outbound  []*OutboundHandlerConfig
	App       []any
	Extension []any
}

// InboundHandlerConfig is one inbound of an instance.
type InboundHandlerConfig struct {
	Tag string
	// ReceiverSettings is the receiver: where it listens and with what stream
	// settings, as the configuration model describes it.
	ReceiverSettings any
	// ProxySettings is the protocol of the inbound, as the configuration model
	// describes it.
	ProxySettings any
}

// OutboundHandlerConfig is one outbound of an instance.
type OutboundHandlerConfig struct {
	Tag string
	// SenderSettings is the sender: how it reaches the server and with what stream
	// settings, as the configuration model describes it.
	SenderSettings any
	// ProxySettings is the protocol of the outbound, as the configuration model
	// describes it.
	ProxySettings any
	// Expire is the moment this outbound stops being used, in seconds.
	Expire int64
	// Comment is what the user wrote about this outbound.
	Comment string
}
