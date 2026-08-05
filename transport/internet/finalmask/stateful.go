package finalmask

import (
	"net"
	"sync"
	"time"
)

// The stateful-mask deadline arbitration: a mask whose connections must
// complete a handshake before proxy data flows. The handshake is lazy (it runs
// at most once, on the first data read or write) and its deadline is arbitrated
// by HandshakeDeadlines so that caller-set deadlines always win.

// statefulHandshakeTimeout bounds a mask handshake that would otherwise block
// the first read or write forever.
const statefulHandshakeTimeout = 2 * time.Minute

// HandshakeDeadlines arbitrates between the mask's internal handshake deadline
// and caller-set deadlines while a stateful mask connection completes its
// handshake: the earlier of the two always applies, and the handshake deadline
// disappears once the handshake is done.
type HandshakeDeadlines struct {
	mu sync.Mutex
	c  net.Conn

	read      time.Time
	write     time.Time
	handshake time.Time
}

func NewHandshakeDeadlines(c net.Conn) *HandshakeDeadlines {
	return &HandshakeDeadlines{c: c}
}

func (d *HandshakeDeadlines) BeginHandshake() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.handshake = time.Now().Add(statefulHandshakeTimeout)
	if err := d.applyLocked(); err != nil {
		d.handshake = time.Time{}
		_ = d.applyLocked()
		return err
	}
	return nil
}

func (d *HandshakeDeadlines) EndHandshake() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.handshake = time.Time{}
	return d.applyLocked()
}

func (d *HandshakeDeadlines) SetDeadline(t time.Time) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.read = t
	d.write = t
	return d.applyLocked()
}

func (d *HandshakeDeadlines) SetReadDeadline(t time.Time) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.read = t
	return d.c.SetReadDeadline(earlierDeadline(d.read, d.handshake))
}

func (d *HandshakeDeadlines) SetWriteDeadline(t time.Time) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.write = t
	return d.c.SetWriteDeadline(earlierDeadline(d.write, d.handshake))
}

func (d *HandshakeDeadlines) applyLocked() error {
	if err := d.c.SetReadDeadline(earlierDeadline(d.read, d.handshake)); err != nil {
		return err
	}
	return d.c.SetWriteDeadline(earlierDeadline(d.write, d.handshake))
}

func earlierDeadline(user, internal time.Time) time.Time {
	if internal.IsZero() {
		return user
	}
	if user.IsZero() || internal.Before(user) {
		return internal
	}
	return user
}
