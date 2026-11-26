package multiplex_test

import (
	"context"
	"testing"
	"time"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/buf"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/session"
	"github.com/xtls/xray-core/proxy/vless/multiplex"
	"github.com/xtls/xray-core/transport"
	"github.com/xtls/xray-core/transport/pipe"
)

// TestServerWorkerParsesReverseFrames locks the frame layout to the context a
// carrier is created with.
//
// A reverse mux new-session frame carries the source and local endpoints after
// the target, and the reader only picks them up when
// session.IsReverseMuxFromContext is set on the context its read loop runs with.
// The reverse portal and the bridge both rely on that, so a carrier that starts
// its read loop with a fresh context silently loses the endpoints.
func TestServerWorkerParsesReverseFrames(t *testing.T) {
	source := net.TCPDestination(net.LocalHostIP, 1234)
	local := net.TCPDestination(net.LocalHostIP, 443)

	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	ctx = session.ContextWithIsReverseMux(ctx, true)
	ctx = session.ContextWithInbound(ctx, &session.Inbound{Tag: "reverse"})

	reader, writer := pipe.New(pipe.WithoutSizeLimit())
	link := &transport.Link{Reader: reader, Writer: buf.Discard}
	dispatched := make(chan *session.Inbound, 1)
	worker := multiplex.NewServerWorker(ctx, func(ctx context.Context, dest net.Destination, link *transport.Link) error {
		dispatched <- session.InboundFromContext(ctx)
		return nil
	}, link)
	defer common.CloseIfExists(worker)

	frame := buf.New()
	common.Must((multiplex.FrameMetadata{
		SessionStatus: multiplex.SessionStatusNew,
		SessionID:     1,
		Target:        net.TCPDestination(net.LocalHostIP, 8443),
		Inbound:       &session.Inbound{Source: source, Local: local},
	}).WriteTo(frame))
	common.Must(writer.WriteMultiBuffer(buf.MultiBuffer{frame}))

	select {
	case inbound := <-dispatched:
		if inbound == nil {
			t.Fatal("dispatched context has no inbound metadata")
		}
		if inbound.Source != source || inbound.Local != local {
			t.Fatalf("reverse frame endpoints were lost: source %v, local %v", inbound.Source, inbound.Local)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server worker did not dispatch the reverse frame")
	}
}
