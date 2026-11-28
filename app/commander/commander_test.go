package commander_test

import (
	"context"
	"io"
	stdnet "net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/xtls/xray-core/app/commander"
	"github.com/xtls/xray-core/app/dispatcher"
	"github.com/xtls/xray-core/app/proxyman"
	_ "github.com/xtls/xray-core/app/proxyman/inbound"
	_ "github.com/xtls/xray-core/app/proxyman/outbound"
	logcommand "github.com/xtls/xray-core/app/log/command"
	appstats "github.com/xtls/xray-core/app/stats"
	statsservice "github.com/xtls/xray-core/app/stats/command"
	"github.com/xtls/xray-core/common/serial"
	"github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/testing/configs"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestCommanderServesGRPCAndMetricsOnOnePort(t *testing.T) {
	listen := pickListenAddress(t)

	server, err := core.New(&core.Config{
		App: []any{
			serial.ToTypedMessage(&dispatcher.Config{}),
			serial.ToTypedMessage(&proxyman.InboundConfig{}),
			serial.ToTypedMessage(&proxyman.OutboundConfig{}),
			serial.ToTypedMessage(&appstats.Config{}),
			serial.ToTypedMessage(&commander.Config{
				Listen:  listen,
				Service: []*serial.TypedMessage{serial.ToTypedMessage(&statsservice.Config{})},
			}),
		},
	})
	if err != nil {
		t.Fatalf("failed to create instance: %v", err)
	}
	if err := server.Start(); err != nil {
		_ = server.Close()
		t.Fatalf("failed to start instance: %v", err)
	}
	t.Cleanup(func() {
		_ = server.Close()
	})

	conn, err := grpc.NewClient(listen, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to dial api server: %v", err)
	}
	t.Cleanup(func() {
		_ = conn.Close()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := statsservice.NewStatsServiceClient(conn).QueryStats(ctx, &statsservice.QueryStatsRequest{}); err != nil {
		t.Fatalf("gRPC call on the api port failed: %v", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+listen+"/debug/vars", nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("HTTP request on the api port failed: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %d", response.StatusCode)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "\"stats\"") {
		t.Fatalf("metrics body missing stats: %s", body)
	}
}

// TestCommanderAcceptsAServiceThatIsAPlainGoConfig checks that a service whose
// settings are a plain Go value is created. A service that is no longer a protobuf
// message crosses the configuration the same way, and used to be refused here.
func TestCommanderAcceptsAServiceThatIsAPlainGoConfig(t *testing.T) {
	server, err := core.New(&core.Config{
		App: []any{
			serial.ToTypedMessage(&dispatcher.Config{}),
			serial.ToTypedMessage(&proxyman.InboundConfig{}),
			serial.ToTypedMessage(&proxyman.OutboundConfig{}),
			serial.ToTypedMessage(&commander.Config{
				Service: []*serial.TypedMessage{serial.ToTypedMessage(&logcommand.Config{})},
			}),
		},
	})
	if err != nil {
		t.Fatalf("a service that is a plain Go configuration was refused: %v", err)
	}
	t.Cleanup(func() {
		_ = server.Close()
	})
}

func pickListenAddress(t *testing.T) string {
	t.Helper()

	listener, err := stdnet.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to pick listen address: %v", err)
	}
	defer listener.Close()
	return listener.Addr().String()
}

func TestMain(m *testing.M) {
	configs.MustInstall(commander.ConfigObjectsCommander, commander.ConfigObjectsCommanderService, statsservice.ConfigObjects, logcommand.ConfigObjects)
	os.Exit(m.Run())
}