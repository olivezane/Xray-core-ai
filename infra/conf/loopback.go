package conf

import (
	"github.com/xtls/xray-core/common/errors"
	coreconfig "github.com/xtls/xray-core/core/config"
	"github.com/xtls/xray-core/proxy/loopback"
)

// LoopbackConfig is the settings of a loopback inbound.
type LoopbackConfig struct {
	coreconfig.Loopback
}

func (l LoopbackConfig) Build() (any, error) {
	c := &loopback.Config{InboundTag: l.InboundTag}
	if l.Sniffing != nil {
		sc, err := BuildSniffing(l.Sniffing)
		if err != nil {
			return nil, errors.New("failed to build sniffing config").Base(err)
		}
		c.Sniffing = sc
	}
	return c, nil
}
