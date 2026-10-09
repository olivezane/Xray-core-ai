package tun

import (
	"errors"
	"sync/atomic"
	"time"

	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

// fakeTun is a pure OS-device fake: no gVisor types on the Tun interface.
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
func (f *fakeTun) newEndpoint() (stack.LinkEndpoint, error) {
	return nil, errors.New("fake tun builds no endpoint")
}

// gvisorFakeTun adds GVisorDevice capability on top of fakeTun, so the real
// gVisor stack can attach to it in tests.
type gvisorFakeTun struct {
	fakeTun
}

func (f *gvisorFakeTun) WritePacket(*stack.PacketBuffer) tcpip.Error { return nil }
func (f *gvisorFakeTun) ReadPacket() (byte, *stack.PacketBuffer, error) {
	atomic.AddInt32(&f.reads, 1)
	return 0, nil, ErrQueueEmpty
}
func (f *gvisorFakeTun) Wait() { time.Sleep(time.Millisecond) }
