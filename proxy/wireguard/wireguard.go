package wireguard

import (
	"context"

	"github.com/xtls/xray-core/common"
)

func init() {
	common.Must(common.RegisterConfig((*DeviceConfig)(nil), func(ctx context.Context, config *DeviceConfig) (any, error) {
		deviceConfig := config
		if deviceConfig.IsClient {
			return NewClient(ctx, deviceConfig)
		} else {
			return NewServer(ctx, deviceConfig)
		}
	}))
}
