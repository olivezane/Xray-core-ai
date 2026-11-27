package commander

import (
	"context"

	"github.com/xtls/xray-core/common"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// Service is a Commander service.
type Service interface {
	// Register registers the service itself to a gRPC server.
	Register(*grpc.Server)
}

type reflectionService struct{}

func (r reflectionService) Register(s *grpc.Server) {
	reflection.Register(s)
}

// ConfigObjects is what this package creates from a configuration, for
// main/distro/all to install: nothing registers itself here, so this is the whole
// surface this package adds to the configuration table.
var ConfigObjectsCommanderService = []common.ConfigObject{
	{
		Config: (*ReflectionConfig)(nil),
		New: func(ctx context.Context, cfg interface{}) (interface{}, error) {
		return reflectionService{}, nil
	},
	},
}
