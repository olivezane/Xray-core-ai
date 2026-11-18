package conf_test

import (
	"encoding/json"
	"testing"

	"github.com/xtls/xray-core/app/commander"
	"github.com/xtls/xray-core/app/metrics"
	"github.com/xtls/xray-core/app/observatory"
	"github.com/xtls/xray-core/app/observatory/burst"
	"github.com/xtls/xray-core/app/stats"
	statsservice "github.com/xtls/xray-core/app/stats/command"
	. "github.com/xtls/xray-core/infra/conf"
)

func TestAPIConfigBuildsStatsAndCommander(t *testing.T) {
	tests := []struct {
		name        string
		raw         string
		api         bool
		services    int
		observatory bool
		burst       bool
	}{
		{
			"api block",
			`{"api":{"listen":"127.0.0.1:10085"}}`,
			true,
			1,
			false,
			false,
		},
		{
			"api block with explicit services",
			`{"api":{"tag":"api","services":["HandlerService"]}}`,
			true,
			2,
			false,
			false,
		},
		{
			"api block with observatory",
			`{"api":{"listen":"127.0.0.1:10085","observatory":{"subjectSelector":["direct"],"probeURL":"https://www.google.com/generate_204","probeInterval":"10s"}}}`,
			true,
			1,
			true,
			false,
		},
		{
			"api block with burst observatory",
			`{"api":{"listen":"127.0.0.1:10085","burstObservatory":{"subjectSelector":["direct"],"pingConfig":{"destination":"https://www.google.com/generate_204","interval":"1m","sampling":3,"timeout":"5s"}}}}`,
			true,
			1,
			false,
			true,
		},
		{
			"removed legacy blocks",
			`{"metrics":{"listen":"127.0.0.1:8080"},"stats":{},"observatory":{"subjectSelector":["direct"]}}`,
			false,
			0,
			false,
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
				commanderConf   *commander.Config
				metricsConf     *metrics.Config
				observatoryConf *observatory.Config
				burstConf       *burst.Config
				hasStats        bool
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
				case *observatory.Config:
					observatoryConf = v
				case *burst.Config:
					burstConf = v
				}
			}

			if hasStats != tt.api {
				t.Errorf("stats app built = %v, want %v", hasStats, tt.api)
			}
			if got := commanderConf != nil; got != tt.api {
				t.Fatalf("commander app built = %v, want %v", got, tt.api)
			}
			if metricsConf != nil {
				t.Error("metrics app must not be built")
			}
			if got := observatoryConf != nil; got != tt.observatory {
				t.Errorf("observatory app built = %v, want %v", got, tt.observatory)
			}
			if got := burstConf != nil; got != tt.burst {
				t.Errorf("burst observatory app built = %v, want %v", got, tt.burst)
			}
			if !tt.api {
				return
			}
			if len(commanderConf.Service) != tt.services {
				t.Errorf("got %d services, want %d", len(commanderConf.Service), tt.services)
			}
			last, err := commanderConf.Service[len(commanderConf.Service)-1].GetInstance()
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := last.(*statsservice.Config); !ok {
				t.Errorf("last service is %T, want stats service", last)
			}
		})
	}
}
