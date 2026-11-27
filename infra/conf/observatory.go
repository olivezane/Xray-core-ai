package conf

import (
	"google.golang.org/protobuf/proto"

	"github.com/xtls/xray-core/app/observatory"
	"github.com/xtls/xray-core/app/observatory/burst"
	"github.com/xtls/xray-core/common/errors"
	coreconfig "github.com/xtls/xray-core/core/config"
)

// ObservatoryConfig is the observatory section of the configuration model.
type ObservatoryConfig = coreconfig.Observatory

// BuildObservatory converts the observatory section to its message.
func BuildObservatory(o *coreconfig.Observatory) (proto.Message, error) {
	return &observatory.Config{SubjectSelector: o.SubjectSelector, ProbeUrl: o.ProbeURL, ProbeInterval: int64(o.ProbeInterval), EnableConcurrency: o.EnableConcurrency}, nil
}

// BurstObservatoryConfig is the burst observatory section of the model.
type BurstObservatoryConfig = coreconfig.BurstObservatory

// BuildBurstObservatory converts the burst observatory section to its message.
func BuildBurstObservatory(b *coreconfig.BurstObservatory) (proto.Message, error) {
	if b.HealthCheck == nil {
		return nil, errors.New("BurstObservatory requires a valid pingConfig")
	}
	if result, err := BuildHealthCheck(b.HealthCheck); err == nil {
		return &burst.Config{SubjectSelector: b.SubjectSelector, PingConfig: result.(*burst.HealthPingConfig)}, nil
	} else {
		return nil, err
	}
}
