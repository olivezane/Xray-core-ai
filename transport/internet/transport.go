package internet

import (
	"context"
	"sort"

	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/transport/internet/stat"
)

// Transport is one stream transport, described in one place: the protocol name
// used in stream settings, its settings, the function that dials with it and the
// function that listens with it.
//
// A transport does not register itself. Every transport Xray ships is listed in
// transport/internet/transports, and that list is installed here by a single
// call, so reading it tells you which protocols exist, who dials them and who
// listens with them.
type Transport struct {
	// Name is the protocol name, e.g. "tcp".
	Name string
	// Config returns the default settings of this transport, the ones a stream
	// settings entry naming it decodes to when it carries none.
	Config ConfigCreator
	// Dial dials through this transport. It is nil when the transport cannot
	// be used to reach a destination.
	Dial DialFunc
	// Listen accepts connections through this transport. It is nil for
	// transports that cannot listen, e.g. "udp".
	Listen ListenFunc
}

// ConnHandler is called by a transport with every accepted connection.
//
// ctx is the transport-level context of that connection. It is derived from the
// listener context, so it carries the transport's values (HYSTERIA validator,
// per-session state, ...) and is cancelled once the listener stops accepting.
// Handlers must derive their own per-connection context from it instead of
// starting from context.Background().
type ConnHandler func(ctx context.Context, conn stat.Connection)

// DialFunc dials towards dest through one stream transport.
type DialFunc func(ctx context.Context, dest net.Destination, streamSettings *MemoryStreamConfig) (stat.Connection, error)

// ListenFunc accepts connections through one stream transport.
type ListenFunc func(ctx context.Context, address net.Address, port net.Port, settings *MemoryStreamConfig, handler ConnHandler) (Listener, error)

// Listener is what a transport hands back after accepting connections.
type Listener interface {
	Close() error
	Addr() net.Addr
}

var transports = make(map[string]Transport)

// SetTransports installs list as the complete set of stream transports of this
// build. It replaces whatever was installed before, so a transport that is not
// listed is not usable, and it reports what is wrong with the list instead of
// silently ignoring an entry.
func SetTransports(list []Transport) error {
	installed := make(map[string]Transport, len(list))
	for _, t := range list {
		switch {
		case t.Name == "":
			return errors.New("stream transport without a name")
		case t.Config == nil:
			return errors.New("stream transport ", t.Name, " has no config creator")
		case t.Dial == nil && t.Listen == nil:
			return errors.New("stream transport ", t.Name, " neither dials nor listens")
		}
		if _, found := installed[t.Name]; found {
			return errors.New("stream transport ", t.Name, " is listed twice")
		}
		installed[t.Name] = t
	}
	transports = installed
	return nil
}

// TransportNames returns the names of the installed stream transports.
func TransportNames() []string {
	names := make([]string, 0, len(transports))
	for name := range transports {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// TransportByName returns the installed stream transport with this name.
func TransportByName(name string) (Transport, bool) {
	transport, found := transports[name]
	return transport, found
}

func transportFor(name string) Transport {
	return transports[name]
}
