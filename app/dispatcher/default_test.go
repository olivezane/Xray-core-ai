package dispatcher

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/buf"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/session"
	"github.com/xtls/xray-core/features/dns"
	"github.com/xtls/xray-core/features/outbound"
	"github.com/xtls/xray-core/transport"
)

// fakeDNSEngine is a minimal FakeDNSEngineRev0 for tests.
type fakeDNSEngine struct {
	pool func(net.Address) bool
}

func (*fakeDNSEngine) Type() interface{}                                    { return (*dns.FakeDNSEngine)(nil) }
func (*fakeDNSEngine) Start() error                                         { return nil }
func (*fakeDNSEngine) Close() error                                         { return nil }
func (*fakeDNSEngine) GetFakeIPForDomain(string) []net.Address              { return nil }
func (*fakeDNSEngine) GetFakeIPForDomain3(string, bool, bool) []net.Address { return nil }
func (*fakeDNSEngine) GetDomainFromFakeDNS(net.Address) string              { return "" }
func (f *fakeDNSEngine) IsIPInIPPool(addr net.Address) bool {
	return f.pool != nil && f.pool(addr)
}

// fakeOutboundManager always yields no handler, so routedDispatch closes the
// link and returns after the routing decision is made.
type fakeOutboundManager struct{}

func (*fakeOutboundManager) Type() interface{}                                  { return outbound.ManagerType() }
func (*fakeOutboundManager) Start() error                                       { return nil }
func (*fakeOutboundManager) Close() error                                       { return nil }
func (*fakeOutboundManager) GetHandler(string) outbound.Handler                 { return nil }
func (*fakeOutboundManager) GetDefaultHandler() outbound.Handler                { return nil }
func (*fakeOutboundManager) AddHandler(context.Context, outbound.Handler) error { return nil }
func (*fakeOutboundManager) RemoveHandler(context.Context, string) error        { return nil }
func (*fakeOutboundManager) ListHandlers(context.Context) []outbound.Handler    { return nil }

// fakeTimeoutReader is a buf.TimeoutReader for tests. It returns payload on
// every read; with timeoutErr set it instead sleeps out the whole deadline
// and fails with buf.ErrReadTimeout; with sleep set it delays each read by
// min(deadline, sleep).
type fakeTimeoutReader struct {
	payload    []byte
	sleep      time.Duration
	timeoutErr bool
}

func (r *fakeTimeoutReader) ReadMultiBuffer() (buf.MultiBuffer, error) {
	return r.ReadMultiBufferTimeout(0)
}

func (r *fakeTimeoutReader) ReadMultiBufferTimeout(d time.Duration) (buf.MultiBuffer, error) {
	if r.timeoutErr {
		time.Sleep(d)
		return nil, buf.ErrReadTimeout
	}
	if r.sleep > 0 && d > 0 {
		time.Sleep(min(d, r.sleep))
	}
	if len(r.payload) == 0 {
		return nil, io.EOF
	}
	return buf.MultiBuffer{buf.FromBytes(r.payload)}, nil
}

// fakePlainResult is a plain (non-composite, non-fake-DNS) SnifferResult.
type fakePlainResult struct {
	protocol string
	domain   string
}

func (r fakePlainResult) Protocol() string                { return r.protocol }
func (r fakePlainResult) Domain() string                  { return r.domain }
func (r fakePlainResult) ProtocolForDomainResult() string { return r.protocol }
func (fakePlainResult) IsProtoSubsetOf(string) bool       { return false }
func (fakePlainResult) IsFakeDNS() bool                   { return false }

func newFakeSniffer(result SnifferResult, err error) *Sniffer {
	return newFakeSnifferFn(func([]byte) (SnifferResult, error) { return result, err })
}

func newFakeSnifferFn(fn func([]byte) (SnifferResult, error)) *Sniffer {
	return &Sniffer{sniffer: []protocolSnifferWithMetadata{{
		protocolSniffer: func(context.Context, []byte) (SnifferResult, error) { return fn(nil) },
		network:         net.Network_TCP,
	}}}
}

const fakeIP = "198.18.0.1"

func testDestination() net.Destination {
	return net.Destination{Address: net.ParseAddress(fakeIP), Port: 443, Network: net.Network_TCP}
}

// runSniffAndDispatch drives sniffAndDispatch with a fake sniffer that
// returns the given result, and returns the outbound whose target/route
// decision is made. outbounds and the outbound link are injected into the
// context directly; no core instance is fabricated.
func runSniffAndDispatch(t *testing.T, d *DefaultDispatcher, request session.SniffingRequest, result SnifferResult) *session.Outbound {
	t.Helper()
	destination := testDestination()
	ob := &session.Outbound{Target: destination}
	ctx := session.ContextWithOutbounds(context.Background(), []*session.Outbound{ob})
	outbound := &transport.Link{Reader: &fakeTimeoutReader{payload: []byte("x")}}
	d.sniffAndDispatch(ctx, newFakeSniffer(result, nil), outbound, destination, request)
	return ob
}

func TestSniffAndDispatchComposite(t *testing.T) {
	d := &DefaultDispatcher{fdns: &fakeDNSEngine{}, ohm: &fakeOutboundManager{}}
	content := new(session.Content)
	destination := testDestination()
	ob := &session.Outbound{Target: destination}
	ctx := session.ContextWithOutbounds(session.ContextWithContent(context.Background(), content), []*session.Outbound{ob})
	outbound := &transport.Link{Reader: &fakeTimeoutReader{payload: []byte("x")}}

	// Metadata says fake DNS (domain part), content says tls: the composite
	// must be matched by the domain-side protocol "fakedns" and the target
	// replaced by the sniffed domain.
	result := CompositeResult(&fakeDNSSniffResult{domainName: "example.com"}, fakePlainResult{protocol: "tls"})
	d.sniffAndDispatch(ctx, newFakeSniffer(result, nil), outbound, destination, session.SniffingRequest{
		Enabled:                        true,
		OverrideDestinationForProtocol: []string{"fakedns"},
	})

	if got := ob.Target.Address.String(); got != "example.com" {
		t.Fatalf("Target.Address = %q, want %q", got, "example.com")
	}
	if ob.RouteTarget.Address != nil {
		t.Fatalf("RouteTarget = %q, want unset", ob.RouteTarget.Address)
	}
	if content.Protocol != "tls" {
		t.Fatalf("Content.Protocol = %q, want %q", content.Protocol, "tls")
	}
}

func TestSniffAndDispatchSubset(t *testing.T) {
	d := &DefaultDispatcher{fdns: &fakeDNSEngine{}, ohm: &fakeOutboundManager{}}

	// "fakedns+others" overrides only via subset matching: the override
	// protocol "http1" is a prefix of the embedded original protocol.
	ob := runSniffAndDispatch(t, d, session.SniffingRequest{
		Enabled:                        true,
		OverrideDestinationForProtocol: []string{"http1"},
	}, DNSThenOthersSniffResult{domainName: "example.com", protocolOriginalName: "http1"})

	if got := ob.Target.Address.String(); got != "example.com" {
		t.Fatalf("Target.Address = %q, want %q", got, "example.com")
	}

	// An unrelated override protocol must not match the subset result.
	ob = runSniffAndDispatch(t, d, session.SniffingRequest{
		Enabled:                        true,
		OverrideDestinationForProtocol: []string{"tls"},
	}, DNSThenOthersSniffResult{domainName: "example.com", protocolOriginalName: "http1"})

	if got := ob.Target.Address.String(); got != fakeIP {
		t.Fatalf("Target.Address = %q, want unchanged %q", got, fakeIP)
	}
	if ob.RouteTarget.Address != nil {
		t.Fatalf("RouteTarget = %q, want unset", ob.RouteTarget.Address)
	}
}

func TestSniffAndDispatchFakeDNSResult(t *testing.T) {
	d := &DefaultDispatcher{fdns: &fakeDNSEngine{}, ohm: &fakeOutboundManager{}}

	// A fake DNS result must replace the target even under RouteOnly: the
	// fake IP cannot be connected to directly.
	ob := runSniffAndDispatch(t, d, session.SniffingRequest{
		Enabled:                        true,
		RouteOnly:                      true,
		OverrideDestinationForProtocol: []string{"fakedns"},
	}, &fakeDNSSniffResult{domainName: "example.com"})

	if got := ob.Target.Address.String(); got != "example.com" {
		t.Fatalf("Target.Address = %q, want %q", got, "example.com")
	}
	if ob.RouteTarget.Address != nil {
		t.Fatalf("RouteTarget = %q, want unset", ob.RouteTarget.Address)
	}
}

func TestSniffAndDispatchFakeDNSMissed(t *testing.T) {
	// The target is in the fake DNS pool but the sniffer only identified a
	// plain protocol: "fakedns" override must still hit via the pool check.
	d := &DefaultDispatcher{
		fdns: &fakeDNSEngine{pool: func(net.Address) bool { return true }},
		ohm:  &fakeOutboundManager{},
	}
	ob := runSniffAndDispatch(t, d, session.SniffingRequest{
		Enabled:                        true,
		OverrideDestinationForProtocol: []string{"fakedns"},
	}, fakePlainResult{protocol: "tls", domain: "example.com"})

	if got := ob.Target.Address.String(); got != "example.com" {
		t.Fatalf("Target.Address = %q, want %q", got, "example.com")
	}

	// Same result outside the pool: no override, target unchanged.
	d.fdns = &fakeDNSEngine{}
	ob = runSniffAndDispatch(t, d, session.SniffingRequest{
		Enabled:                        true,
		OverrideDestinationForProtocol: []string{"fakedns"},
	}, fakePlainResult{protocol: "tls", domain: "example.com"})

	if got := ob.Target.Address.String(); got != fakeIP {
		t.Fatalf("Target.Address = %q, want unchanged %q", got, fakeIP)
	}
}

func TestSniffAndDispatchRouteOnly(t *testing.T) {
	destination := testDestination()

	// Real target: RouteOnly keeps the target and routes by the domain.
	d := &DefaultDispatcher{fdns: &fakeDNSEngine{}, ohm: &fakeOutboundManager{}}
	ob := runSniffAndDispatch(t, d, session.SniffingRequest{
		Enabled:                        true,
		RouteOnly:                      true,
		OverrideDestinationForProtocol: []string{"tls"},
	}, fakePlainResult{protocol: "tls", domain: "example.com"})

	if ob.RouteTarget.Address == nil || ob.RouteTarget.Address.String() != "example.com" {
		t.Fatalf("RouteTarget = %v, want %q", ob.RouteTarget.Address, "example.com")
	}
	if got := ob.Target.Address.String(); got != destination.Address.String() {
		t.Fatalf("Target.Address = %q, want unchanged %q", got, destination.Address.String())
	}

	// Fake target: the fake IP is unreachable, so the target must be
	// replaced by the sniffed domain even under RouteOnly.
	d = &DefaultDispatcher{
		fdns: &fakeDNSEngine{pool: func(net.Address) bool { return true }},
		ohm:  &fakeOutboundManager{},
	}
	ob = runSniffAndDispatch(t, d, session.SniffingRequest{
		Enabled:                        true,
		RouteOnly:                      true,
		OverrideDestinationForProtocol: []string{"tls"},
	}, fakePlainResult{protocol: "tls", domain: "example.com"})

	if got := ob.Target.Address.String(); got != "example.com" {
		t.Fatalf("Target.Address = %q, want %q", got, "example.com")
	}
	if ob.RouteTarget.Address != nil {
		t.Fatalf("RouteTarget = %q, want unset", ob.RouteTarget.Address)
	}
}

func TestSnifferTimeoutOnNoClue(t *testing.T) {
	// ErrNoClue counts as an attempt: two attempts exhaust the loop.
	s := newFakeSnifferFn(func([]byte) (SnifferResult, error) { return nil, common.ErrNoClue })
	cr := &cachedReader{reader: &fakeTimeoutReader{payload: []byte("x")}}
	result, err := sniffer(context.Background(), s, cr, false, net.Network_TCP)
	if err != errSniffingTimeout {
		t.Fatalf("err = %v, want %v", err, errSniffingTimeout)
	}
	if result != nil {
		t.Fatalf("result = %v, want nil", result)
	}
}

func TestSnifferTimeoutOnReadDeadline(t *testing.T) {
	// The reader sleeps out the whole 200ms cache deadline and fails:
	// the loop must exit with the reader's error.
	s := newFakeSnifferFn(func([]byte) (SnifferResult, error) { return nil, common.ErrNoClue })
	cr := &cachedReader{reader: &fakeTimeoutReader{timeoutErr: true}}
	start := time.Now()
	_, err := sniffer(context.Background(), s, cr, false, net.Network_TCP)
	if err != buf.ErrReadTimeout {
		t.Fatalf("err = %v, want %v", err, buf.ErrReadTimeout)
	}
	if elapsed := time.Since(start); elapsed < 200*time.Millisecond {
		t.Fatalf("returned after %v, want the full cache deadline", elapsed)
	}
}

func TestSnifferNeedMoreDataWaitsForDeadline(t *testing.T) {
	// ErrProtoNeedMoreData does not count as an attempt: the loop must
	// keep reading until the cache deadline is exhausted.
	var calls int
	s := newFakeSnifferFn(func([]byte) (SnifferResult, error) {
		calls++
		return nil, protocol.ErrProtoNeedMoreData
	})
	cr := &cachedReader{reader: &fakeTimeoutReader{payload: []byte("x"), sleep: 150 * time.Millisecond}}
	start := time.Now()
	_, err := sniffer(context.Background(), s, cr, false, net.Network_TCP)
	if err != errSniffingTimeout {
		t.Fatalf("err = %v, want %v", err, errSniffingTimeout)
	}
	if calls < 2 {
		t.Fatalf("sniffer called %d times, want at least 2 reads before the deadline exit", calls)
	}
	if elapsed := time.Since(start); elapsed < 200*time.Millisecond {
		t.Fatalf("returned after %v, want the full cache deadline", elapsed)
	}
}

func TestSnifferCtxCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := newFakeSniffer(nil, common.ErrNoClue)
	cr := &cachedReader{reader: &fakeTimeoutReader{}}
	_, err := sniffer(ctx, s, cr, false, net.Network_TCP)
	if err != context.Canceled {
		t.Fatalf("err = %v, want %v", err, context.Canceled)
	}
}

func TestSnifferMetadataOnly(t *testing.T) {
	s := &Sniffer{sniffer: []protocolSnifferWithMetadata{{
		protocolSniffer: func(context.Context, []byte) (SnifferResult, error) {
			return &fakeDNSSniffResult{domainName: "example.com"}, nil
		},
		metadataSniffer: true,
	}}}
	cr := &cachedReader{reader: &fakeTimeoutReader{}}
	result, err := sniffer(context.Background(), s, cr, true, net.Network_TCP)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if result.Domain() != "example.com" {
		t.Fatalf("Domain() = %q, want %q", result.Domain(), "example.com")
	}
}
