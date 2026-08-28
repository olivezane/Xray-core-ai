package json

import (
	"io"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/cmdarg"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/infra/conf/serial"
)

func init() {
	common.Must(core.RegisterConfigLoader(&core.ConfigFormat{
		Name:      "JSON",
		Extension: []string{"json"},
		Loader: func(input interface{}) (*core.Config, error) {
			switch v := input.(type) {
			case cmdarg.Arg:
				return serial.LoadMultiFileConfig(v, serial.DecodeJSONConfig)
			case io.Reader:
				if serial.UseStrictJSON {
					cfg, err := serial.DecodeJSONConfigStrict(v)
					if err != nil {
						return nil, err
					}
					return cfg.Build()
				}
				return serial.LoadJSONConfig(v)
			default:
				return nil, errors.New("unknown type")
			}
		},
	}))
}
