package inbound

import (
	"github.com/xtls/xray-core/common/tracker"
	finbound "github.com/xtls/xray-core/features/inbound"
	"github.com/xtls/xray-core/transport/internet/stat"
)

// connection is one connection a worker handed to its inbound proxy, together
// with the metadata the proxy fills in while it processes it.
type connection = tracker.Entry

// connRegistry is the part of the instance-wide connection table that the workers
// of one inbound handler write to. The table itself is shared by every inbound
// and outbound handler of the instance, so a connection can be found and closed
// from anywhere, while a handler only ever sees the entries it registered.
type connRegistry struct {
	*tracker.Registry
	owner interface{}
}

func newConnRegistry() *connRegistry {
	return &connRegistry{Registry: tracker.NewRegistry()}
}

// sharedBy returns the view of registry that owner writes to.
func sharedBy(registry *tracker.Registry, owner interface{}) *connRegistry {
	return &connRegistry{Registry: registry, owner: owner}
}

func (r *connRegistry) register(c *connection) bool {
	c.Owner = r.owner
	return r.Register(c)
}

func (r *connRegistry) unregister(conn stat.Connection) {
	r.Unregister(conn)
}

func (r *connRegistry) sessions() []finbound.Session {
	return r.SessionsOf(r.owner)
}

func (r *connRegistry) closeMatching(match func(finbound.Session) bool) int {
	return r.CloseMatchingOf(r.owner, match)
}

// closeAll closes the connections this owner is serving, and nothing else: the
// table is shared by the whole instance, so closing it here would reach the
// connections of other handlers, including the ones serving the management API.
func (r *connRegistry) closeAll() {
	r.CloseMatchingOf(r.owner, nil)
}
