package conf

import (
	"strings"

	"google.golang.org/protobuf/proto"

	"github.com/xtls/xray-core/app/observatory/burst"
	coreconfig "github.com/xtls/xray-core/core/config"
	"github.com/xtls/xray-core/app/router"
)

const (
	strategyRandom     string = "random"
	strategyLeastPing  string = "leastping"
	strategyRoundRobin string = "roundrobin"
	strategyLeastLoad  string = "leastload"
)

var strategyConfigLoader = NewJSONConfigLoader(ConfigCreatorCache{
	strategyRandom:     func() interface{} { return new(strategyEmptyConfig) },
	strategyLeastPing:  func() interface{} { return new(strategyEmptyConfig) },
	strategyRoundRobin: func() interface{} { return new(strategyEmptyConfig) },
	strategyLeastLoad:  func() interface{} { return new(strategyLeastLoadConfig) },
}, "type", "settings")

type strategyEmptyConfig struct{}

func (v *strategyEmptyConfig) Build() (proto.Message, error) {
	return nil, nil
}

// strategyLeastLoadConfig is the least load strategy of the configuration. It
// carries the model type and adds the conversion, so the model stays in
// core/config while the boundary stays here.
type strategyLeastLoadConfig struct {
	coreconfig.LeastLoadStrategy
}

// HealthCheckSettings is the health check section of the configuration model.
type HealthCheckSettings = coreconfig.HealthCheck

// BuildHealthCheck converts the health check section to its message.
func BuildHealthCheck(h *coreconfig.HealthCheck) (proto.Message, error) {
	var httpMethod string
	if h.HttpMethod == "" {
		httpMethod = "HEAD"
	} else {
		httpMethod = strings.TrimSpace(h.HttpMethod)
	}
	return &burst.HealthPingConfig{
		Destination:   h.Destination,
		Connectivity:  h.Connectivity,
		Interval:      int64(h.Interval),
		Timeout:       int64(h.Timeout),
		SamplingCount: int32(h.SamplingCount),
		HttpMethod:    httpMethod,
	}, nil
}

// Build implements Buildable.
func (v *strategyLeastLoadConfig) Build() (proto.Message, error) {
	config := &router.StrategyLeastLoadConfig{}
	config.Costs = v.Costs
	config.Tolerance = float32(v.Tolerance)
	if config.Tolerance < 0 {
		config.Tolerance = 0
	}
	if config.Tolerance > 1 {
		config.Tolerance = 1
	}
	config.Expected = v.Expected
	if config.Expected < 0 {
		config.Expected = 0
	}
	config.MaxRTT = int64(v.MaxRTT)
	if config.MaxRTT < 0 {
		config.MaxRTT = 0
	}
	config.Baselines = make([]int64, 0)
	for _, b := range v.Baselines {
		if b <= 0 {
			continue
		}
		config.Baselines = append(config.Baselines, int64(b))
	}
	return config, nil
}
