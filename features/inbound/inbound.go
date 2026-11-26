package inbound

import (
	"context"
	"time"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/serial"
	"github.com/xtls/xray-core/features"
)

// Handler is the interface for handlers that process inbound connections.
//
// xray:api:stable
type Handler interface {
	common.Runnable
	// The tag of this handler.
	Tag() string
	// Returns the active receiver settings.
	ReceiverSettings() *serial.TypedMessage
	// Returns the active proxy settings.
	ProxySettings() *serial.TypedMessage
}

// Manager is a feature that manages InboundHandlers.
//
// xray:api:stable
type Manager interface {
	features.Feature
	// GetHandler returns an InboundHandler for the given tag.
	GetHandler(ctx context.Context, tag string) (Handler, error)
	// AddHandler adds the given handler into this Manager.
	AddHandler(ctx context.Context, handler Handler) error

	// RemoveHandler removes a handler from Manager.
	RemoveHandler(ctx context.Context, tag string) error

	// ListHandlers returns a list of inbound.Handler.
	ListHandlers(ctx context.Context) []Handler
}

// ManagerType returns the type of Manager interface. Can be used for implementing common.HasType.
//
// xray:api:stable
func ManagerType() interface{} {
	return (*Manager)(nil)
}

// Session is a snapshot of one connection an inbound handler is serving.
//
// xray:api:beta
type Session struct {
	// InboundTag is the tag of the inbound handling the connection.
	InboundTag string
	// Network is the network the connection runs on.
	Network net.Network
	// Source is the peer address of the connection.
	Source net.Destination
	// Local is the address the connection was accepted on.
	Local net.Destination
	// Email is the user authenticated for the connection. Empty while the
	// protocol has not authenticated it yet, or when it allows anonymous traffic.
	Email string
	// Start is when the connection was accepted.
	Start time.Time
}

// SessionTracker is implemented by inbound handlers and by the inbound Manager.
// It exposes the connections currently being served, so that they can be
// inspected and closed as a whole, per inbound, or per user.
//
// xray:api:beta
type SessionTracker interface {
	// Sessions returns a snapshot of the connections being served.
	Sessions(ctx context.Context) []Session
	// CloseSessions closes the connections matching match and reports how many
	// it closed. A nil match closes every connection.
	CloseSessions(ctx context.Context, match func(Session) bool) int
}
