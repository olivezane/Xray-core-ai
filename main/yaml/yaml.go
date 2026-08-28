package yaml

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
		Name:      "YAML",
		Extension: []string{"yaml", "yml"},
		Loader: func(input interface{}) (*core.Config, error) {
			switch v := input.(type) {
			case cmdarg.Arg:
				return serial.LoadMultiFileConfig(v, serial.DecodeYAMLConfig)
			case io.Reader:
				return serial.LoadYAMLConfig(v)
			default:
				return nil, errors.New("unknown type")
			}
		},
	}))
}
