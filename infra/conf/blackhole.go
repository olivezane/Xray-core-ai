package conf

import (
	"encoding/json"

	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/serial"
	"github.com/xtls/xray-core/proxy/blackhole"
	"google.golang.org/protobuf/proto"
)

type NoneResponse struct{}

func (*NoneResponse) Build() (proto.Message, error) {
	return new(blackhole.NoneResponse), nil
}

type HTTPResponse struct{}

func (*HTTPResponse) Build() (proto.Message, error) {
	return new(blackhole.HTTPResponse), nil
}

type BlackholeConfig struct {
	Response json.RawMessage `json:"response"`
}

func (v *BlackholeConfig) Build() (proto.Message, error) {
	config := new(blackhole.Config)
	if v.Response != nil {
		response, _, err := configLoader.Load(v.Response)
		if err != nil {
			return nil, errors.New("Config: Failed to parse Blackhole response config.").Base(err)
		}
		responseSettings, err := response.(Buildable).Build()
		if err != nil {
			return nil, err
		}
		config.Response = serial.ToTypedMessage(responseSettings)
	}

	return config, nil
}

var configLoader = NewJSONConfigLoader(NewConfigRegistry(), "type", "")

func init() {
	configLoader.MustRegister("none", func() interface{} { return new(NoneResponse) })
	configLoader.MustRegister("http", func() interface{} { return new(HTTPResponse) })
	outboundConfigLoader.MustRegister("block", func() interface{} { return new(BlackholeConfig) })
	outboundConfigLoader.MustRegister("blackhole", func() interface{} { return new(BlackholeConfig) })
}
