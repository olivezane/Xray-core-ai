package inbound

import (
	"github.com/xtls/xray-core/common/tracker"
	finbound "github.com/xtls/xray-core/features/inbound"
	"github.com/xtls/xray-core/transport/internet/stat"
)

// connection is one connection a worker handed to its inbound proxy, together
// with the metadata the proxy fills in while it processes it.
type connection = tracker.Entry

// connRegistry tracks the connections the workers of one inbound handler are
// serving. It belongs to the handler, not to a worker, so that closing the
// handler closes every connection of every port at once.
type connRegistry struct {
	*tracker.Registry
}

func newConnRegistry() *connRegistry {
	return &connRegistry{Registry: tracker.NewRegistry()}
}

func (r *connRegistry) register(c *connection) bool {
	return r.Register(c)
}

func (r *connRegistry) unregister(conn stat.Connection) {
	r.Unregister(conn)
}

func (r *connRegistry) sessions() []finbound.Session {
	return r.Sessions()
}

func (r *connRegistry) closeMatching(match func(finbound.Session) bool) int {
	return r.CloseMatching(match)
}

func (r *connRegistry) closeAll() {
	r.CloseAll()
}
