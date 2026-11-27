// Package hub is the accept loop of a stream inbound: it owns the listener of a
// transport, the context that stops it, and the goroutine that hands every
// accepted connection to the handler.
//
// It lives in transport rather than next to the inbounds because that is what it
// is: transport machinery. An inbound only says where to listen, with what stream
// settings, and what to do with a connection.
package hub

import (
	"context"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/serial"
	"github.com/xtls/xray-core/transport/internet"
	"github.com/xtls/xray-core/transport/internet/stat"
)

// Handler is what an inbound does with a connection the hub accepted.
type Handler func(ctx context.Context, conn stat.Connection)

// Options are the transport-level extras of a listener.
type Options struct {
	// PrepareStreamContext derives the context the listener and its connections
	// start from, e.g. to install a transport's per-listener state. It may be nil.
	PrepareStreamContext func(ctx context.Context) context.Context
}

// Listener is a started accept loop.
type Listener struct {
	hub    internet.Listener
	cancel context.CancelFunc
	port   net.Port
}

// ListenTCP starts accepting TCP connections on address:port and hands each one
// to handler in its own goroutine. The context of the listener is the parent of
// every connection context, so closing the listener cancels what it accepted.
func ListenTCP(ctx context.Context, address net.Address, port net.Port, settings *internet.MemoryStreamConfig, handler Handler, options Options) (*Listener, error) {
	return listen(ctx, address, port, settings, handler, options, false)
}

// ListenUnix starts accepting Unix domain socket connections on address.
func ListenUnix(ctx context.Context, address net.Address, settings *internet.MemoryStreamConfig, handler Handler, options Options) (*Listener, error) {
	return listen(ctx, address, 0, settings, handler, options, true)
}

func listen(ctx context.Context, address net.Address, port net.Port, settings *internet.MemoryStreamConfig, handler Handler, options Options, unix bool) (*Listener, error) {
	ctx, cancel := context.WithCancel(ctx)
	if options.PrepareStreamContext != nil {
		ctx = options.PrepareStreamContext(ctx)
	}

	accept := func(connCtx context.Context, conn stat.Connection) {
		go handler(connCtx, conn)
	}

	var (
		listener internet.Listener
		err      error
	)
	if unix {
		listener, err = internet.ListenUnix(ctx, address, settings, accept)
		if err != nil {
			cancel()
			return nil, errors.New("failed to listen Unix Domain Socket on ", address).Base(err)
		}
	} else {
		listener, err = internet.ListenTCP(ctx, address, port, settings, accept)
		if err != nil {
			cancel()
			return nil, errors.New("failed to listen TCP on ", port).Base(err)
		}
	}

	return &Listener{hub: listener, cancel: cancel, port: port}, nil
}

// Close stops the accept loop and every connection it started.
func (l *Listener) Close() error {
	if l == nil {
		return nil
	}
	l.cancel()
	return common.Close(l.hub)
}

// Port is the port the listener accepts on.
func (l *Listener) Port() net.Port {
	if l == nil {
		return 0
	}
	return l.port
}

// Addr is the address the listener accepts on.
func (l *Listener) Addr() net.Addr {
	if l == nil || l.hub == nil {
		return nil
	}
	return l.hub.Addr()
}

// CloseAll closes everything given and reports the first failure, for the close
// paths that have to close a listener and an inbound together. Values that are
// not closable are skipped, as common.Close does.
func CloseAll(closers ...interface{}) error {
	var errs []interface{}
	for _, closer := range closers {
		if closer == nil {
			continue
		}
		if err := common.Close(closer); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.New("failed to close all resources").Base(errors.New(serial.Concat(errs...)))
	}
	return nil
}
