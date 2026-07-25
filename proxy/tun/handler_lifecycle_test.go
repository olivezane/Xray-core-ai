package tun

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xtls/xray-core/features/policy"
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

func (f *fakeTun) SetUpdater(*InterfaceUpdater) {}
func (f *fakeTun) Start() error                 { return f.startErr }
func (f *fakeTun) Close() error                 { f.closed = true; return nil }
func (f *fakeTun) Name() (string, error)        { return "fake", nil }
func (f *fakeTun) Index() (int, error)          { return 10, f.indexErr }

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

func newTestHandler(fake Tun) *Handler {
	return &Handler{
		ctx:           context.Background(),
		config:        &Config{},
		policyManager: policy.DefaultManager{},
		newTun: func(*Config) (Tun, error) {
			return fake, nil
		},
	}
}

// assertDispatchLoopStopped fails when the stack's packet dispatch loop is
// still reading from the device: a properly closed stack detaches its
// endpoint, which cancels the loop.
func assertDispatchLoopStopped(t *testing.T, fake *gvisorFakeTun) {
	t.Helper()
	time.Sleep(30 * time.Millisecond)
	after := atomic.LoadInt32(&fake.reads)
	time.Sleep(30 * time.Millisecond)
	if got := atomic.LoadInt32(&fake.reads); got != after {
		t.Fatalf("dispatch loop still reading from the device (%d -> %d); stack was not closed", after, got)
	}
}

func TestStartFailsWhenDeviceCannotBeCreated(t *testing.T) {
	h := &Handler{
		ctx:           context.Background(),
		config:        &Config{},
		policyManager: policy.DefaultManager{},
		newTun: func(*Config) (Tun, error) {
			return nil, errors.New("cannot create tun")
		},
	}
	if err := h.Start(); err == nil {
		t.Fatal("expected error")
	}
	if h.tun != nil || h.stack != nil {
		t.Fatal("handler must not hold resources after a failed Start")
	}
}

func TestStartClosesDeviceWhenIndexFails(t *testing.T) {
	fake := &fakeTun{indexErr: errors.New("cannot resolve index")}
	h := newTestHandler(fake)
	h.config.AutoOutboundsInterface = "auto"

	if err := h.Start(); err == nil {
		t.Fatal("expected error")
	}
	if !fake.closed {
		t.Fatal("device must be closed when Index fails")
	}
	if h.tun != nil || h.stack != nil {
		t.Fatal("handler must not hold resources after a failed Start")
	}
}

func TestStartClosesDeviceWhenStackConstructionFails(t *testing.T) {
	fake := &fakeTun{}
	h := newTestHandler(fake)
	h.newStack = func(context.Context, StackOptions, *Handler) (*stackGVisor, error) {
		return nil, errors.New("cannot build stack")
	}

	if err := h.Start(); err == nil {
		t.Fatal("expected error")
	}
	if !fake.closed {
		t.Fatal("device must be closed when stack construction fails")
	}
}

func TestStartClosesDeviceWhenStackStartFails(t *testing.T) {
	// the fake implements no endpoint capability, so stack.Start errors
	fake := &fakeTun{}
	h := newTestHandler(fake)

	if err := h.Start(); err == nil {
		t.Fatal("expected error")
	}
	if !fake.closed {
		t.Fatal("device must be closed when stack.Start fails")
	}
}

func TestStartUnwindsStackAndDeviceWhenDeviceStartFails(t *testing.T) {
	fake := &gvisorFakeTun{fakeTun{startErr: errors.New("device start failed")}}
	h := newTestHandler(fake)

	if err := h.Start(); err == nil {
		t.Fatal("expected error")
	}
	if !fake.closed {
		t.Fatal("device must be closed when its Start fails")
	}
	if h.tun != nil || h.stack != nil {
		t.Fatal("handler must not hold resources after a failed Start")
	}
	// the stack must have been closed too: its dispatch loop reads from the
	// device in a loop and must stop once the stack is closed
	assertDispatchLoopStopped(t, fake)
}

func TestStartAndCloseLifecycle(t *testing.T) {
	fake := &gvisorFakeTun{}
	h := newTestHandler(fake)

	if err := h.Start(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.tun != fake || h.stack == nil {
		t.Fatal("handler must hold the device and stack after successful Start")
	}

	if err := h.Close(); err != nil {
		t.Fatalf("unexpected close error: %v", err)
	}
	if !fake.closed {
		t.Fatal("device must be closed by Handler.Close")
	}
	assertDispatchLoopStopped(t, fake)
}
