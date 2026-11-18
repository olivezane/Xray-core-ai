package conf_test

import (
	"encoding/json"
	"testing"

	"github.com/xtls/xray-core/app/commander"
	"github.com/xtls/xray-core/app/metrics"
	"github.com/xtls/xray-core/app/stats"
	statsservice "github.com/xtls/xray-core/app/stats/command"
	. "github.com/xtls/xray-core/infra/conf"
)

func TestAPIConfigBuildsStatsAndMetrics(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		services int
		metrics  bool
	}{
		{
			"api with nested metrics",
			`{"api":{"listen":"127.0.0.1:10085","metrics":{"listen":"127.0.0.1:8080"}}}`,
			1,
			true,
		},
		{
			"legacy blocks",
			`{"api":{"tag":"api","services":["HandlerService"]},"stats":{}}`,
			2,
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parserConfig := new(Config)
			if err := json.Unmarshal([]byte(tt.raw), parserConfig); err != nil {
				t.Fatal(err)
			}
			built, err := parserConfig.Build()
			if err != nil {
				t.Fatal(err)
			}

			var (
				commanderConf *commander.Config
				metricsConf   *metrics.Config
				hasStats      bool
			)
			for _, app := range built.App {
				instance, err := app.GetInstance()
				if err != nil {
					t.Fatal(err)
				}
				switch v := instance.(type) {
				case *commander.Config:
					commanderConf = v
				case *metrics.Config:
					metricsConf = v
				case *stats.Config:
					hasStats = true
				}
			}

			if !hasStats {
				t.Error("stats app not built")
			}
			if commanderConf == nil {
				t.Fatal("commander app not built")
			}
			if len(commanderConf.Service) != tt.services {
				t.Errorf("got %d services, want %d", len(commanderConf.Service), tt.services)
			}
			if last, err := commanderConf.Service[len(commanderConf.Service)-1].GetInstance(); err != nil {
				t.Fatal(err)
			} else if _, ok := last.(*statsservice.Config); !ok {
				t.Errorf("last service is %T, want stats service", last)
			}
			if tt.metrics && (metricsConf == nil || metricsConf.Listen != "127.0.0.1:8080") {
				t.Errorf("metrics app = %+v", metricsConf)
			}
			if !tt.metrics && metricsConf != nil {
				t.Errorf("unexpected metrics app %+v", metricsConf)
			}
		})
	}
}
