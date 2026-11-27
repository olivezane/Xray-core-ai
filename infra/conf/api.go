package conf

import (
	"strings"

	"github.com/xtls/xray-core/app/commander"
	loggerservice "github.com/xtls/xray-core/app/log/command"
	observatoryservice "github.com/xtls/xray-core/app/observatory/command"
	handlerservice "github.com/xtls/xray-core/app/proxyman/command"
	routerservice "github.com/xtls/xray-core/app/router/command"
	statsservice "github.com/xtls/xray-core/app/stats/command"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/serial"
	coreconfig "github.com/xtls/xray-core/core/config"
)

// APIConfig is the single management block. Its one port serves both the gRPC
// control service and the HTTP /debug/vars + /debug/pprof endpoints.
// Statistics collection is implied, so the stats app never needs a block of its own.
// APIConfig is the management block of the configuration model.
type APIConfig = coreconfig.API

// BuildAPI converts the management block to the message the instance consumes:
// the service names of the user become the messages of the services here.
func BuildAPI(c *coreconfig.API) (*commander.Config, error) {
	if c.Tag == "" && c.Listen == "" {
		return nil, errors.New("API must have a tag or listen address.")
	}

	services := make([]*serial.TypedMessage, 0, 16)
	hasStatsService := false
	for _, s := range c.Services {
		switch strings.ToLower(s) {
		case "reflectionservice":
			services = append(services, serial.ToTypedMessage(&commander.ReflectionConfig{}))
		case "handlerservice":
			services = append(services, serial.ToTypedMessage(&handlerservice.Config{}))
		case "loggerservice":
			services = append(services, serial.ToTypedMessage(&loggerservice.Config{}))
		case "statsservice":
			hasStatsService = true
			services = append(services, serial.ToTypedMessage(&statsservice.Config{}))
		case "observatoryservice":
			services = append(services, serial.ToTypedMessage(&observatoryservice.Config{}))
		case "routingservice":
			services = append(services, serial.ToTypedMessage(&routerservice.Config{}))
		}
	}

	// stats is always collected for the API, so expose it without extra configuration.
	if !hasStatsService {
		services = append(services, serial.ToTypedMessage(&statsservice.Config{}))
	}

	return &commander.Config{
		Tag:     c.Tag,
		Listen:  c.Listen,
		Service: services,
	}, nil
}
