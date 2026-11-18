package reverse

import (
	"context"
	"time"

	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/mux"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/session"
	"github.com/xtls/xray-core/common/signal"
	"github.com/xtls/xray-core/features/routing"
	"github.com/xtls/xray-core/transport"
	"github.com/xtls/xray-core/transport/pipe"
	"google.golang.org/protobuf/proto"
)

type BridgeWorker struct {
	Tag        string
	Worker     *mux.ServerWorker
	Dispatcher routing.Dispatcher
	State      Control_State
	Timer      *signal.ActivityTimer
}

func NewBridgeWorker(domain string, tag string, d routing.Dispatcher) (*BridgeWorker, error) {
	ctx := context.Background()
	ctx = session.ContextWithInbound(ctx, &session.Inbound{
		Tag: tag,
	})
	link, err := d.Dispatch(ctx, net.Destination{
		Network: net.Network_TCP,
		Address: net.DomainAddress(domain),
		Port:    0,
	})
	if err != nil {
		return nil, err
	}

	w := &BridgeWorker{
		Dispatcher: d,
		Tag:        tag,
	}

	worker, err := mux.NewServerWorker(context.Background(), w, link)
	if err != nil {
		return nil, err
	}
	w.Worker = worker

	terminate := func() {
		worker.Close()
	}
	w.Timer = signal.CancelAfterInactivity(ctx, terminate, 60*time.Second)
	return w, nil
}

func (w *BridgeWorker) Type() interface{} {
	return routing.DispatcherType()
}

func (w *BridgeWorker) Start() error {
	return nil
}

func (w *BridgeWorker) Close() error {
	return nil
}

func (w *BridgeWorker) IsActive() bool {
	return w.State == Control_ACTIVE && !w.Worker.Closed()
}

func (w *BridgeWorker) Closed() bool {
	return w.Worker.Closed()
}

func (w *BridgeWorker) Connections() uint32 {
	return w.Worker.ActiveConnections()
}

func (w *BridgeWorker) handleInternalConn(link *transport.Link) {
	reader := link.Reader
	for {
		mb, err := reader.ReadMultiBuffer()
		if err != nil {
			if w.Timer != nil {
				if w.Closed() {
					w.Timer.SetTimeout(0)
				} else {
					w.Timer.SetTimeout(24 * time.Hour)
				}
			}
			return
		}
		if w.Timer != nil {
			w.Timer.Update()
		}
		for _, b := range mb {
			var ctl Control
			if err := proto.Unmarshal(b.Bytes(), &ctl); err != nil {
				errors.LogInfoInner(context.Background(), err, "failed to parse proto message")
				if w.Timer != nil {
					w.Timer.SetTimeout(0)
				}
				return
			}
			if ctl.State != w.State {
				w.State = ctl.State
			}
		}
	}
}

func (w *BridgeWorker) Dispatch(ctx context.Context, dest net.Destination) (*transport.Link, error) {
	if !isInternalDomain(dest) {
		if session.InboundFromContext(ctx) == nil {
			ctx = session.ContextWithInbound(ctx, &session.Inbound{
				Tag: w.Tag,
			})
		}
		return w.Dispatcher.Dispatch(ctx, dest)
	}

	opt := []pipe.Option{pipe.WithSizeLimit(16 * 1024)}
	uplinkReader, uplinkWriter := pipe.New(opt...)
	downlinkReader, downlinkWriter := pipe.New(opt...)

	go w.handleInternalConn(&transport.Link{
		Reader: downlinkReader,
		Writer: uplinkWriter,
	})

	return &transport.Link{
		Reader: uplinkReader,
		Writer: downlinkWriter,
	}, nil
}

func (w *BridgeWorker) DispatchLink(ctx context.Context, dest net.Destination, link *transport.Link) error {
	if !isInternalDomain(dest) {
		if session.InboundFromContext(ctx) == nil {
			ctx = session.ContextWithInbound(ctx, &session.Inbound{
				Tag: w.Tag,
			})
		}
		return w.Dispatcher.DispatchLink(ctx, dest, link)
	}
	w.handleInternalConn(link)

	return nil
}
