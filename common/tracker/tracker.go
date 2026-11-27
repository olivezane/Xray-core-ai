// Package tracker keeps the connections an instance is carrying, so that they can
// be inspected and closed as a whole, per inbound, per outbound, or per user.
package tracker

import (
	"context"
	"sync"
	"time"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/session"
	"github.com/xtls/xray-core/features/inbound"
)

// Direction says which side of the instance a connection belongs to.
type Direction int

const (
	Inbound Direction = iota
	Outbound
)

// Entry is one connection being carried, together with the metadata its owner
// fills in while it works with it.
type Entry struct {
	// Owner is who registered the connection: an inbound handler or an outbound
	// handler. It is what per-owner views filter on, which keeps two untagged
	// handlers apart.
	Owner interface{}
	// Direction tells an accepted connection from a dialed one.
	Direction Direction
	// Inbound is the session metadata of an accepted connection. The proxy owns
	// it and fills it in during processing, so the tracker only reads it, and
	// only when a snapshot is taken: that is how a connection is reported with
	// the user it authenticated as, without every proxy having to report it.
	Inbound *session.Inbound
	// OutboundTag and Destination describe a dialed connection.
	OutboundTag string
	Destination net.Destination
	Network net.Network
	Start   time.Time
	Conn    net.Conn
	// Stream is the identifier of the logical stream inside a multiplexed
	// connection, e.g. a VLESS mux session. Zero means the whole connection.
	Stream uint64
	// Cancel stops the handling of this connection. It may be nil.
	Cancel context.CancelFunc
}

// Session renders the entry as a snapshot for inbound.SessionTracker.
func (e *Entry) Session() inbound.Session {
	s := inbound.Session{
		Direction:   inbound.Direction(e.Direction),
		OutboundTag: e.OutboundTag,
		Destination: e.Destination,
		Network:     e.Network,
		Stream:      e.Stream,
		Start:       e.Start,
	}
	if e.Inbound != nil {
		s.InboundTag = e.Inbound.Tag
		s.Source = e.Inbound.Source
		s.Local = e.Inbound.Local
		if e.Inbound.User != nil {
			s.Email = e.Inbound.User.Email
		}
	}
	return s
}

// Registry is one table of connections. An instance has a single one, shared by
// its inbound and outbound handlers: that is what makes the table global, while
// each handler still sees and closes only its own connections. Closing them is
// what makes a closed instance, handler or removed user actually release its
// connections instead of leaving goroutines and buffers behind.
type Registry struct {
	mu      sync.Mutex
	entries map[*Entry]struct{}
	closed  bool
}

func NewRegistry() *Registry {
	return &Registry{
		entries: make(map[*Entry]struct{}, 16),
	}
}

// Register tracks e. It reports false when the registry is already closed, in
// which case the caller must not serve the connection.
func (r *Registry) Register(e *Entry) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return false
	}
	r.entries[e] = struct{}{}
	return true
}

// Unregister removes an entry. Entries are addressed by identity, not by
// connection, so that several logical streams of one connection can be tracked
// (and dropped) separately.
func (r *Registry) Unregister(e *Entry) {
	if e == nil {
		return
	}
	r.mu.Lock()
	delete(r.entries, e)
	r.mu.Unlock()
}

// Sessions returns a snapshot of every tracked connection of the instance.
func (r *Registry) Sessions() []inbound.Session {
	return r.sessions(nil)
}

// SessionsOf returns a snapshot of the connections owner registered.
func (r *Registry) SessionsOf(owner interface{}) []inbound.Session {
	return r.sessions(func(e *Entry) bool { return e.Owner == owner })
}

func (r *Registry) sessions(keep func(*Entry) bool) []inbound.Session {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]inbound.Session, 0, len(r.entries))
	for e := range r.entries {
		if keep == nil || keep(e) {
			out = append(out, e.Session())
		}
	}
	return out
}

// CloseMatching cancels and closes the tracked connections matching match. The
// connections stay registered until their owner unregisters them, so that a
// single owner stays responsible for each one.
func (r *Registry) CloseMatching(match func(inbound.Session) bool) int {
	return r.closeMatching(nil, match)
}

// CloseMatchingOf closes the connections owner registered that match match.
func (r *Registry) CloseMatchingOf(owner interface{}, match func(inbound.Session) bool) int {
	return r.closeMatching(func(e *Entry) bool { return e.Owner == owner }, match)
}

func (r *Registry) closeMatching(keep func(*Entry) bool, match func(inbound.Session) bool) int {
	r.mu.Lock()
	targets := make([]*Entry, 0, len(r.entries))
	for e := range r.entries {
		if keep != nil && !keep(e) {
			continue
		}
		if match == nil || match(e.Session()) {
			targets = append(targets, e)
		}
	}
	r.mu.Unlock()

	for _, e := range targets {
		if e.Cancel != nil {
			e.Cancel() // let the owner unwind its handling
		}
		common.CloseIfExists(e.Conn) // and unblock its reads and writes
	}
	return len(targets)
}

// CloseAll refuses further registrations and closes every tracked connection.
func (r *Registry) CloseAll() {
	r.mu.Lock()
	r.closed = true
	r.mu.Unlock()

	r.CloseMatching(nil)
}
