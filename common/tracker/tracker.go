// Package tracker keeps the connections an inbound is serving, so that they can
// be inspected and closed as a whole, per inbound, or per user.
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

// Entry is one connection being served, together with the metadata its proxy
// fills in while it processes it.
type Entry struct {
	// Inbound is the session metadata of the connection. The proxy owns it and
	// fills it in during processing, so the tracker only reads it, and only when
	// a snapshot is taken: that is how a connection is reported with the user it
	// authenticated as, without every proxy having to report it.
	Inbound *session.Inbound
	Network net.Network
	Start   time.Time
	Conn    net.Conn
	// Cancel stops the handling of this connection. It may be nil.
	Cancel context.CancelFunc
}

// Session renders the entry as a snapshot for inbound.SessionTracker.
func (e *Entry) Session() inbound.Session {
	s := inbound.Session{
		Network: e.Network,
		Start:   e.Start,
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

// Registry tracks the connections one inbound is serving. Closing them is what
// makes a closed handler or a removed user actually release its connections
// instead of leaving their goroutines and buffers behind.
type Registry struct {
	mu     sync.Mutex
	conns  map[net.Conn]*Entry
	closed bool
}

func NewRegistry() *Registry {
	return &Registry{
		conns: make(map[net.Conn]*Entry, 16),
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
	r.conns[e.Conn] = e
	return true
}

func (r *Registry) Unregister(conn net.Conn) {
	r.mu.Lock()
	delete(r.conns, conn)
	r.mu.Unlock()
}

// Sessions returns a snapshot of the tracked connections.
func (r *Registry) Sessions() []inbound.Session {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]inbound.Session, 0, len(r.conns))
	for _, e := range r.conns {
		out = append(out, e.Session())
	}
	return out
}

// CloseMatching cancels and closes the tracked connections matching match. The
// connections stay registered until their owner unregisters them, so that a
// single owner stays responsible for each one.
func (r *Registry) CloseMatching(match func(inbound.Session) bool) int {
	r.mu.Lock()
	targets := make([]*Entry, 0, len(r.conns))
	for _, e := range r.conns {
		if match == nil || match(e.Session()) {
			targets = append(targets, e)
		}
	}
	r.mu.Unlock()

	for _, e := range targets {
		if e.Cancel != nil {
			e.Cancel() // let the proxy unwind its handling
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
