package session

import (
	"context"

	"github.com/xtls/xray-core/common/ctx"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/features/outbound"
	"github.com/xtls/xray-core/features/routing"
)

// stateKey is the one context key this package uses.
const stateKey ctx.SessionKey = 1

// state is everything Xray carries in a context for one connection or request.
//
// It used to be thirteen independent context keys, so a function signature could
// not tell you what the session carries, and every new field meant a new key. Now
// there is one struct and one key: this is the whole session, and the accessors
// below are thin wrappers over it.
//
// The state is written copy-on-write. Deriving a context copies the state and
// changes one field, so two contexts derived from the same parent never see each
// other's writes, while pointers reached through a field stay shared exactly as
// before (Content, for instance, is mutated in place).
type state struct {
	// inbound is the inbound handler serving the connection.
	inbound *Inbound
	// outbounds is the outbound chain of the request; it grows when traffic is
	// redirected into another outbound.
	outbounds []*Outbound
	// content is what the request is for: destination, protocol, sniffing, ...
	content *Content
	// isReverseMux marks traffic that belongs to a reverse mux.
	isReverseMux bool
	// sockopt is used by dokodemo to only receive sockopt.Mark.
	sockopt *Sockopt
	// trackedError is used by an observer to get the outbound error.
	trackedError TrackedRequestErrorFeedback
	// dispatcher is used by ss2022 inbounds to get the dispatcher.
	dispatcher routing.Dispatcher
	// timeoutOnly makes a mux context's child contexts cancel only when their own
	// traffic times out.
	timeoutOnly bool
	// allowedNetwork is how a muxcool server constrains incoming TCP/UDP requests.
	allowedNetwork net.Network
	// fullHandler is the handler the outbound gets, when it needs more than the
	// picker.
	fullHandler outbound.Handler
	// mitmAlpn11 and mitmServerName are used by the TLS dialer.
	mitmAlpn11     bool
	mitmServerName string
	// streamSettings are the dialer stream settings of the request.
	streamSettings any
}

// stateFromContext returns the session state of ctx, or nil when the context
// carries none.
func stateFromContext(ctx context.Context) *state {
	if state, ok := ctx.Value(stateKey).(*state); ok {
		return state
	}
	return nil
}

// contextWithChange returns a context whose state is the state of ctx with change
// applied. Neither ctx nor its state is modified, so a context is never changed
// under the code already using it.
func contextWithChange(ctx context.Context, change func(*state)) context.Context {
	var next state
	if current := stateFromContext(ctx); current != nil {
		next = *current
	}
	change(&next)
	return context.WithValue(ctx, stateKey, &next)
}

func ContextWithInbound(ctx context.Context, inbound *Inbound) context.Context {
	return contextWithChange(ctx, func(s *state) { s.inbound = inbound })
}

func InboundFromContext(ctx context.Context) *Inbound {
	if state := stateFromContext(ctx); state != nil {
		return state.inbound
	}
	return nil
}

func ContextWithOutbounds(ctx context.Context, outbounds []*Outbound) context.Context {
	return contextWithChange(ctx, func(s *state) { s.outbounds = outbounds })
}

func SubContextFromMuxInbound(ctx context.Context) context.Context {
	newOutbounds := []*Outbound{{}}

	content := ContentFromContext(ctx)
	newContent := Content{}
	if content != nil {
		newContent = *content
		if content.Attributes != nil {
			panic("content.Attributes != nil")
		}
	}
	return ContextWithContent(ContextWithOutbounds(ctx, newOutbounds), &newContent)
}

func OutboundsFromContext(ctx context.Context) []*Outbound {
	if state := stateFromContext(ctx); state != nil {
		return state.outbounds
	}
	return nil
}

func ContextWithContent(ctx context.Context, content *Content) context.Context {
	return contextWithChange(ctx, func(s *state) { s.content = content })
}

func ContentFromContext(ctx context.Context) *Content {
	if state := stateFromContext(ctx); state != nil {
		return state.content
	}
	return nil
}

func SkipDNSResolveFromContext(ctx context.Context) bool {
	content := ContentFromContext(ctx)
	return content != nil && content.SkipDNSResolve
}

func ContextWithIsReverseMux(ctx context.Context, isReverseMux bool) context.Context {
	return contextWithChange(ctx, func(s *state) { s.isReverseMux = isReverseMux })
}

func IsReverseMuxFromContext(ctx context.Context) bool {
	if state := stateFromContext(ctx); state != nil {
		return state.isReverseMux
	}
	return false
}

func ContextWithSockopt(ctx context.Context, sockopt *Sockopt) context.Context {
	return contextWithChange(ctx, func(s *state) { s.sockopt = sockopt })
}

func SockoptFromContext(ctx context.Context) *Sockopt {
	if state := stateFromContext(ctx); state != nil {
		return state.sockopt
	}
	return nil
}

func GetForcedOutboundTagFromContext(ctx context.Context) string {
	if ContentFromContext(ctx) == nil {
		return ""
	}
	return ContentFromContext(ctx).Attribute("forcedOutboundTag")
}

func SetForcedOutboundTagToContext(ctx context.Context, tag string) context.Context {
	if contentFromContext := ContentFromContext(ctx); contentFromContext == nil {
		ctx = ContextWithContent(ctx, &Content{})
	}
	ContentFromContext(ctx).SetAttribute("forcedOutboundTag", tag)
	return ctx
}

type TrackedRequestErrorFeedback interface {
	SubmitError(err error)
}

func SubmitOutboundErrorToOriginator(ctx context.Context, err error) {
	if state := stateFromContext(ctx); state != nil && state.trackedError != nil {
		state.trackedError.SubmitError(err)
	}
}

func TrackedConnectionError(ctx context.Context, tracker TrackedRequestErrorFeedback) context.Context {
	return contextWithChange(ctx, func(s *state) { s.trackedError = tracker })
}

func ContextWithDispatcher(ctx context.Context, dispatcher routing.Dispatcher) context.Context {
	return contextWithChange(ctx, func(s *state) { s.dispatcher = dispatcher })
}

func DispatcherFromContext(ctx context.Context) routing.Dispatcher {
	if state := stateFromContext(ctx); state != nil {
		return state.dispatcher
	}
	return nil
}

func ContextWithTimeoutOnly(ctx context.Context, only bool) context.Context {
	return contextWithChange(ctx, func(s *state) { s.timeoutOnly = only })
}

func TimeoutOnlyFromContext(ctx context.Context) bool {
	if state := stateFromContext(ctx); state != nil {
		return state.timeoutOnly
	}
	return false
}

func ContextWithAllowedNetwork(ctx context.Context, network net.Network) context.Context {
	return contextWithChange(ctx, func(s *state) { s.allowedNetwork = network })
}

func AllowedNetworkFromContext(ctx context.Context) net.Network {
	if state := stateFromContext(ctx); state != nil {
		return state.allowedNetwork
	}
	return net.Network_Unknown
}

func ContextWithFullHandler(ctx context.Context, handler outbound.Handler) context.Context {
	return contextWithChange(ctx, func(s *state) { s.fullHandler = handler })
}

func FullHandlerFromContext(ctx context.Context) outbound.Handler {
	if state := stateFromContext(ctx); state != nil {
		return state.fullHandler
	}
	return nil
}

func ContextWithMitmAlpn11(ctx context.Context, alpn11 bool) context.Context {
	return contextWithChange(ctx, func(s *state) { s.mitmAlpn11 = alpn11 })
}

func MitmAlpn11FromContext(ctx context.Context) bool {
	if state := stateFromContext(ctx); state != nil {
		return state.mitmAlpn11
	}
	return false
}

func ContextWithMitmServerName(ctx context.Context, serverName string) context.Context {
	return contextWithChange(ctx, func(s *state) { s.mitmServerName = serverName })
}

func MitmServerNameFromContext(ctx context.Context) string {
	if state := stateFromContext(ctx); state != nil {
		return state.mitmServerName
	}
	return ""
}

func ContextWithStreamSettings(ctx context.Context, streamSettings any) context.Context {
	return contextWithChange(ctx, func(s *state) { s.streamSettings = streamSettings })
}

func StreamSettingsFromContext(ctx context.Context) any {
	if state := stateFromContext(ctx); state != nil {
		return state.streamSettings
	}
	return nil
}
