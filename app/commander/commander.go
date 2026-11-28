package commander

import (
	"context"
	stderrors "errors"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/xtls/xray-core/app/metrics"
	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/signal/done"
	core "github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/features/outbound"
	"github.com/xtls/xray-core/transport/internet"
	"google.golang.org/grpc"
)

// Commander is a Xray feature that provides gRPC methods to external clients.
type Commander struct {
	sync.Mutex
	server     *grpc.Server
	httpServer *http.Server
	services   []Service
	ohm        outbound.Manager
	ctx        context.Context
	tag        string
	listen     string
}

// NewCommander creates a new Commander based on the given config.
func NewCommander(ctx context.Context, config *Config) (*Commander, error) {
	c := &Commander{
		tag:    config.Tag,
		listen: config.Listen,
		ctx:    ctx,
	}

	common.Must(core.RequireFeatures(ctx, func(om outbound.Manager) {
		c.ohm = om
	}))

	for _, rawConfig := range config.Service {
		// A service is configured by a plain Go value now; one that is still a
		// message is read the way protobuf reads it.
		serviceConfig := common.UnwrapSettings(rawConfig)
		if serviceConfig == any(rawConfig) {
			message, err := rawConfig.GetInstance()
			if err != nil {
				return nil, err
			}
			serviceConfig = message
		}
		rawService, err := common.CreateObject(ctx, serviceConfig)
		if err != nil {
			return nil, err
		}
		service, ok := rawService.(Service)
		if !ok {
			return nil, errors.New("not a Service.")
		}
		c.services = append(c.services, service)
	}

	return c, nil
}

// Type implements common.HasType.
func (c *Commander) Type() interface{} {
	return (*Commander)(nil)
}

// Start implements common.Runnable.
func (c *Commander) Start() error {
	c.Lock()
	grpcServer := grpc.NewServer()
	for _, service := range c.services {
		service.Register(grpcServer)
	}
	c.server = grpcServer
	c.Unlock()

	debugHandler, err := metrics.NewDebugHandler(c.ctx)
	if err != nil {
		return err
	}

	// A single port serves both the gRPC API and /debug/vars + /debug/pprof.
	root := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ProtoMajor == 2 && strings.HasPrefix(r.Header.Get("Content-Type"), "application/grpc") {
			grpcServer.ServeHTTP(w, r)
			return
		}
		debugHandler.ServeHTTP(w, r)
	})
	httpServer := &http.Server{Handler: root, Protocols: new(http.Protocols)}
	httpServer.Protocols.SetHTTP1(true)
	httpServer.Protocols.SetUnencryptedHTTP2(true)

	c.Lock()
	c.httpServer = httpServer
	c.Unlock()

	listen := func(listener net.Listener) {
		if err := httpServer.Serve(listener); err != nil && !stderrors.Is(err, http.ErrServerClosed) && !stderrors.Is(err, net.ErrClosed) {
			errors.LogErrorInner(context.Background(), err, "failed to start api server")
		}
	}

	if len(c.listen) > 0 {
		var addr net.Addr

		if strings.HasPrefix(c.listen, "/") || strings.HasPrefix(c.listen, "@") {
			addr = &net.UnixAddr{Name: c.listen, Net: "unix"}
		} else {
			tcpAddr, err := net.ResolveTCPAddr("tcp", c.listen)
			if err != nil {
				errors.LogErrorInner(context.Background(), err, "API server failed to parse listen address ", c.listen)
				return err
			}
			addr = tcpAddr
		}
		l, err := internet.ListenSystem(context.Background(), addr, nil)
		if err != nil {
			errors.LogErrorInner(context.Background(), err, "API server failed to listen on ", c.listen)
			return err
		}
		errors.LogInfo(context.Background(), "API server listening on ", l.Addr())
		go listen(l)
		return nil
	}

	listener := &OutboundListener{
		buffer: make(chan net.Conn, 4),
		done:   done.New(),
	}

	go listen(listener)

	if err := c.ohm.RemoveHandler(context.Background(), c.tag); err != nil {
		errors.LogInfoInner(context.Background(), err, "failed to remove existing handler")
	}

	return c.ohm.AddHandler(context.Background(), &Outbound{
		tag:      c.tag,
		listener: listener,
	})
}

// Close implements common.Closable.
func (c *Commander) Close() error {
	c.Lock()
	defer c.Unlock()

	if c.httpServer != nil {
		if err := c.httpServer.Close(); err != nil && !stderrors.Is(err, http.ErrServerClosed) {
			errors.LogInfoInner(context.Background(), err, "failed to close api server")
		}
		c.httpServer = nil
	}

	if c.server != nil {
		c.server.Stop()
		c.server = nil
	}

	return nil
}

// ConfigObjects is what this package creates from a configuration, for
// main/distro/all to install: nothing registers itself here, so this is the whole
// surface this package adds to the configuration table.
var ConfigObjectsCommander = []common.ConfigObject{
	{
		Config: (*Config)(nil),
		New: func(ctx context.Context, cfg interface{}) (interface{}, error) {
		return NewCommander(ctx, cfg.(*Config))
	},
	},
}
