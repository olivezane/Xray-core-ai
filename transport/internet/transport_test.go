package internet_test

import (
	"context"
	"strings"
	"testing"

	xnet "github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/transport/internet"
	"github.com/xtls/xray-core/transport/internet/stat"
)

// TestDialReportsUnwiredTransport checks what a stream settings naming a
// protocol that is not in the transport list tells the user: the name, and
// where the transports of this build are listed.
func TestDialReportsUnwiredTransport(t *testing.T) {
	settings := &internet.MemoryStreamConfig{ProtocolName: "no-such-transport"}

	_, err := internet.Dial(context.Background(), xnet.TCPDestination(xnet.LocalHostIP, 1), settings)
	if err == nil {
		t.Fatal("dialing an unwired transport succeeded")
	}
	message := err.Error()
	if !strings.Contains(message, "no-such-transport") || !strings.Contains(message, "transport/internet/transports") {
		t.Fatalf("error does not name the unwired transport and where transports are listed: %v", err)
	}
}

// TestListenReportsUnwiredTransport is the listener side of the same check.
func TestListenReportsUnwiredTransport(t *testing.T) {
	settings := &internet.MemoryStreamConfig{ProtocolName: "no-such-transport"}
	handler := func(context.Context, stat.Connection) {}

	_, err := internet.ListenTCP(context.Background(), xnet.LocalHostIP, 1, settings, handler)
	if err == nil {
		t.Fatal("listening on an unwired transport succeeded")
	}
	message := err.Error()
	if !strings.Contains(message, "no-such-transport") || !strings.Contains(message, "transport/internet/transports") {
		t.Fatalf("error does not name the unwired transport and where transports are listed: %v", err)
	}
}

// TestSetTransportsRejectsBrokenList checks that a mistake in the list is
// reported where it is written instead of turning into "not wired" at runtime.
func TestSetTransportsRejectsBrokenList(t *testing.T) {
	dial := func(context.Context, xnet.Destination, *internet.MemoryStreamConfig) (stat.Connection, error) {
		return nil, nil
	}
	for _, test := range []struct {
		name   string
		list   []internet.Transport
		report string
	}{
		{name: "unnamed", list: []internet.Transport{{Dial: dial}}, report: "without a name"},
		{name: "idle", list: []internet.Transport{{Name: "idle"}}, report: "neither dials nor listens"},
		{name: "twice", list: []internet.Transport{{Name: "a", Dial: dial}, {Name: "a", Dial: dial}}, report: "listed twice"},
	} {
		err := internet.SetTransports(test.list)
		if err == nil {
			t.Fatalf("%s: broken transport list was accepted", test.name)
		}
		if !strings.Contains(err.Error(), test.report) {
			t.Errorf("%s: error %q does not say %q", test.name, err, test.report)
		}
	}
	// The rejected lists must not have replaced what was installed.
	if _, found := internet.TransportByName("a"); found {
		t.Error("a rejected transport list was installed anyway")
	}
}
