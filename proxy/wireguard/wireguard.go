package wireguard

import (
	"context"

	"github.com/xtls/xray-core/common"
)

// ConfigObjects is what this package creates from a configuration, for
// main/distro/all to install: nothing registers itself here, so this is the whole
// surface this package adds to the configuration table.
var ConfigObjects = []common.ConfigObject{
	{
		Config: (*DeviceConfig)(nil),
		New: func(ctx context.Context, config interface{}) (interface{}, error) {
		deviceConfig := config.(*DeviceConfig)
		if deviceConfig.IsClient {
			return NewClient(ctx, deviceConfig)
		} else {
			return NewServer(ctx, deviceConfig)
		}
	},
	},
}
