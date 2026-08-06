package finalmask

import (
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeStream is a committed stream: serves reads from data and records Stop.
type fakeStream struct {
	data    []byte
	stopped atomic.Bool
}

func (s *fakeStream) Read(p []byte) (int, error) {
	if len(s.data) == 0 {
		return 0, io.EOF
	}
	n := copy(p, s.data)
	s.data = s.data[n:]
	return n, nil
}

func (s *fakeStream) Write(p []byte) (int, error) { return len(p), nil }

func (s *fakeStream) Stop() { s.stopped.Store(true) }

func TestStatefulConnLazyHandshakeOnFirstRead(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	var calls atomic.Int32
	stream := &fakeStream{data: []byte("payload")}
	sc := NewStatefulConn(client, client, func(h *Handshake) error {
		calls.Add(1)
		return h.Commit(stream)
	})

	buf := make([]byte, 16)
	n, err := sc.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if string(buf[:n]) != "payload" {
		t.Fatalf("read %q, want payload", buf[:n])
	}
	if calls.Load() != 1 {
		t.Fatalf("handshake calls = %d, want 1", calls.Load())
	}
	if _, err := sc.Read(buf); !errors.Is(err, io.EOF) {
		t.Fatalf("second read err = %v, want EOF", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("handshake calls after second read = %d, want 1", calls.Load())
	}
}

func TestStatefulConnLazyHandshakeOnFirstWrite(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	var calls atomic.Int32
	sc := NewStatefulConn(client, client, func(h *Handshake) error {
		calls.Add(1)
		return h.Commit(&fakeStream{})
	})
	if _, err := sc.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatalf("handshake calls = %d, want 1", calls.Load())
	}
}

func TestStatefulConnConcurrentFirstIOTriggersHandshakeOnce(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	var calls atomic.Int32
	entered := make(chan struct{})
	release := make(chan struct{})
	stream := &fakeStream{data: []byte("payload")}
	sc := NewStatefulConn(client, client, func(h *Handshake) error {
		calls.Add(1)
		close(entered)
		<-release
		return h.Commit(stream)
	})

	var wg sync.WaitGroup
	wg.Add(2)
	var readErr, writeErr error
	go func() {
		defer wg.Done()
		_, readErr = sc.Read(make([]byte, 16))
	}()
	go func() {
		defer wg.Done()
		_, writeErr = sc.Write([]byte("x"))
	}()
	<-entered
	close(release)
	wg.Wait()

	if calls.Load() != 1 {
		t.Fatalf("handshake calls = %d, want 1", calls.Load())
	}
	if readErr != nil {
		t.Fatalf("read err = %v", readErr)
	}
	if writeErr != nil {
		t.Fatalf("write err = %v", writeErr)
	}
}

func TestStatefulConnCloseDuringHandshakeStopsStream(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	entered := make(chan struct{})
	release := make(chan struct{})
	stream := &fakeStream{}
	sc := NewStatefulConn(client, client, func(h *Handshake) error {
		close(entered)
		<-release
		return h.Commit(stream)
	})

	readDone := make(chan error, 1)
	go func() {
		_, err := sc.Read(make([]byte, 16))
		readDone <- err
	}()
	<-entered
	if err := sc.Close(); err != nil {
		t.Fatal(err)
	}
	close(release)

	if err := <-readDone; !errors.Is(err, net.ErrClosed) {
		t.Fatalf("read err = %v, want ErrClosed", err)
	}
	if !stream.stopped.Load() {
		t.Fatal("committed stream was not stopped")
	}
}

func TestStatefulConnHandshakeFailureSurfacesOnTriggeringCall(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	sentinel := errors.New("boom")
	sc := NewStatefulConn(client, client, func(h *Handshake) error {
		return sentinel
	})

	_, err := sc.Read(make([]byte, 16))
	if !errors.Is(err, sentinel) {
		t.Fatalf("read err = %v, want sentinel", err)
	}
	if !strings.Contains(err.Error(), "handshake:") {
		t.Fatalf("read err = %q, want wrapped with handshake:", err)
	}
}

func TestStatefulConnBracketsHandshakeDeadlines(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	recording := &deadlineRecordingConn{Conn: client}
	entered := make(chan struct{})
	release := make(chan struct{})
	sc := NewStatefulConn(recording, recording, func(h *Handshake) error {
		close(entered)
		<-release
		return h.Commit(&fakeStream{data: []byte("x")})
	})
	callerDeadline := time.Now().Add(10 * time.Minute)
	if err := sc.SetDeadline(callerDeadline); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := sc.Read(make([]byte, 16))
		done <- err
	}()
	<-entered
	read, write := recording.currentDeadlines()
	if read.IsZero() || write.IsZero() {
		t.Fatalf("deadlines not applied during handshake: %s/%s", read, write)
	}
	if !read.Before(callerDeadline) || !write.Before(callerDeadline) {
		t.Fatalf("handshake deadlines = %s/%s, want before caller %s", read, write, callerDeadline)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	read, write = recording.currentDeadlines()
	if !read.Equal(callerDeadline) || !write.Equal(callerDeadline) {
		t.Fatalf("restored deadlines = %s/%s, want %s", read, write, callerDeadline)
	}
}
