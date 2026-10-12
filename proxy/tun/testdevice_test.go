package tun

import (
	"errors"
	"sync/atomic"
	"time"
)

// fakeTun is a pure OS-device fake: a Tun with no packets of its own behind
// it. Tests embed it when they only need the interface, and give it a packet
// device of their own when they need one.
type fakeTun struct {
	startErr error
	indexErr error

	closed bool
	reads  int32
}

func (f *fakeTun) Start() error          { return f.startErr }
func (f *fakeTun) Close() error          { f.closed = true; return nil }
func (f *fakeTun) Name() (string, error) { return "fake", nil }
func (f *fakeTun) Index() (int, error)   { return 10, f.indexErr }

func (f *fakeTun) ReadPacket([]byte) ([]byte, error) {
	atomic.AddInt32(&f.reads, 1)
	time.Sleep(time.Millisecond)
	return nil, errors.New("fake tun has no packets")
}

func (f *fakeTun) WritePacket(header, payload []byte) error { return nil }
