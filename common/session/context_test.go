package session_test

import (
	"context"
	"testing"

	xnet "github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/session"
)

// TestStateHoldsEveryField checks that writing one part of the session keeps the
// other parts: with one struct behind one key, a setter that forgot to copy would
// silently drop whatever was written before it.
func TestStateHoldsEveryField(t *testing.T) {
	inbound := &session.Inbound{}
	content := &session.Content{}
	outbounds := []*session.Outbound{{}}
	sockopt := &session.Sockopt{}

	ctx := context.Background()
	ctx = session.ContextWithInbound(ctx, inbound)
	ctx = session.ContextWithContent(ctx, content)
	ctx = session.ContextWithOutbounds(ctx, outbounds)
	ctx = session.ContextWithSockopt(ctx, sockopt)
	ctx = session.ContextWithIsReverseMux(ctx, true)
	ctx = session.ContextWithTimeoutOnly(ctx, true)
	ctx = session.ContextWithAllowedNetwork(ctx, xnet.Network_UDP)
	ctx = session.ContextWithMitmAlpn11(ctx, true)
	ctx = session.ContextWithMitmServerName(ctx, "example.com")
	ctx = session.ContextWithStreamSettings(ctx, "settings")

	if session.InboundFromContext(ctx) != inbound {
		t.Error("inbound was lost")
	}
	if session.ContentFromContext(ctx) != content {
		t.Error("content was lost")
	}
	if got := session.OutboundsFromContext(ctx); len(got) != 1 || got[0] != outbounds[0] {
		t.Errorf("outbounds were lost: %v", got)
	}
	if session.SockoptFromContext(ctx) != sockopt {
		t.Error("sockopt was lost")
	}
	if !session.IsReverseMuxFromContext(ctx) {
		t.Error("isReverseMux was lost")
	}
	if !session.TimeoutOnlyFromContext(ctx) {
		t.Error("timeoutOnly was lost")
	}
	if session.AllowedNetworkFromContext(ctx) != xnet.Network_UDP {
		t.Error("allowedNetwork was lost")
	}
	if !session.MitmAlpn11FromContext(ctx) {
		t.Error("mitmAlpn11 was lost")
	}
	if session.MitmServerNameFromContext(ctx) != "example.com" {
		t.Error("mitmServerName was lost")
	}
	if session.StreamSettingsFromContext(ctx) != "settings" {
		t.Error("streamSettings were lost")
	}
}

// TestSiblingContextsAreIsolated checks the copy-on-write rule the struct needs:
// two contexts derived from the same parent must not see each other's writes.
func TestSiblingContextsAreIsolated(t *testing.T) {
	parent := session.ContextWithInbound(context.Background(), &session.Inbound{})

	withContent := session.ContextWithContent(parent, &session.Content{})
	withSockopt := session.ContextWithSockopt(parent, &session.Sockopt{})

	if session.ContentFromContext(withSockopt) != nil {
		t.Error("a write for one sibling leaked into another")
	}
	if session.SockoptFromContext(withContent) != nil {
		t.Error("a write for one sibling leaked into another")
	}
	if session.InboundFromContext(withContent) == nil || session.InboundFromContext(withSockopt) == nil {
		t.Error("the parent value was lost in a sibling")
	}
	if session.ContentFromContext(parent) != nil || session.SockoptFromContext(parent) != nil {
		t.Error("a child's write changed its parent")
	}
}

// TestDefaultsWithoutState checks what a context without a session reports: the
// same values the separate keys used to default to.
func TestDefaultsWithoutState(t *testing.T) {
	ctx := context.Background()

	if session.InboundFromContext(ctx) != nil || session.ContentFromContext(ctx) != nil {
		t.Error("a bare context reports a session")
	}
	if session.OutboundsFromContext(ctx) != nil || session.SockoptFromContext(ctx) != nil {
		t.Error("a bare context reports a session")
	}
	if session.DispatcherFromContext(ctx) != nil || session.FullHandlerFromContext(ctx) != nil {
		t.Error("a bare context reports a session")
	}
	if session.IsReverseMuxFromContext(ctx) || session.TimeoutOnlyFromContext(ctx) || session.MitmAlpn11FromContext(ctx) {
		t.Error("a bare context reports a flag")
	}
	if session.AllowedNetworkFromContext(ctx) != xnet.Network_Unknown {
		t.Error("allowedNetwork does not default to unknown")
	}
	if session.MitmServerNameFromContext(ctx) != "" || session.StreamSettingsFromContext(ctx) != nil {
		t.Error("a bare context reports a value")
	}
	if session.SkipDNSResolveFromContext(ctx) || session.GetForcedOutboundTagFromContext(ctx) != "" {
		t.Error("a bare context reports a content value")
	}
}

// TestForcedOutboundTagKeepsContent checks that setting the tag on an existing
// session keeps the Content it was given, since callers mutate that pointer in
// place and compare it.
func TestForcedOutboundTagKeepsContent(t *testing.T) {
	content := &session.Content{}
	ctx := session.ContextWithContent(context.Background(), content)

	ctx = session.SetForcedOutboundTagToContext(ctx, "proxy")

	if session.ContentFromContext(ctx) != content {
		t.Fatal("setting the forced outbound tag replaced the content")
	}
	if got := session.GetForcedOutboundTagFromContext(ctx); got != "proxy" {
		t.Errorf("forced outbound tag = %q, want proxy", got)
	}

	empty := session.SetForcedOutboundTagToContext(context.Background(), "direct")
	if session.ContentFromContext(empty) == nil {
		t.Error("setting the tag on a context without content did not create it")
	}
	if session.GetForcedOutboundTagFromContext(empty) != "direct" {
		t.Error("forced outbound tag was not stored")
	}
}

// TestSubContextFromMuxInbound checks what a mux sub context keeps: a fresh
// outbound chain and a copy of the content, with the inbound untouched.
func TestSubContextFromMuxInbound(t *testing.T) {
	inbound := &session.Inbound{}
	content := &session.Content{}
	parent := session.ContextWithContent(session.ContextWithInbound(context.Background(), inbound), content)

	sub := session.SubContextFromMuxInbound(parent)

	if got := session.OutboundsFromContext(sub); len(got) != 1 {
		t.Errorf("sub context outbounds = %v, want one", got)
	}
	if session.ContentFromContext(sub) == content {
		t.Error("sub context shares the content of its parent")
	}
	if session.InboundFromContext(sub) != inbound {
		t.Error("sub context lost the inbound")
	}
}

type errorTracker struct {
	errs []error
}

func (t *errorTracker) SubmitError(err error) {
	t.errs = append(t.errs, err)
}

// TestOutboundErrorReachesTracker checks the observer path: the tracker written
// into the session receives the outbound error, and a context without one is a
// no-op instead of a panic.
func TestOutboundErrorReachesTracker(t *testing.T) {
	tracker := &errorTracker{}
	ctx := session.TrackedConnectionError(context.Background(), tracker)

	session.SubmitOutboundErrorToOriginator(ctx, context.Canceled)
	if len(tracker.errs) != 1 {
		t.Fatalf("tracker got %d errors, want 1", len(tracker.errs))
	}

	session.SubmitOutboundErrorToOriginator(context.Background(), context.Canceled)
}
