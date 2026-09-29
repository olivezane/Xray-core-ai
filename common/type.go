package common

import (
	"context"
	"reflect"

	"github.com/xtls/xray-core/common/errors"
)

// ConfigCreator is a function to create an object by a config.
type ConfigCreator func(ctx context.Context, config any) (any, error)

var typeCreatorRegistry = make(map[reflect.Type]ConfigCreator)

// RegisterConfig registers a global config creator. The config can be nil but
// must have a type. The creator receives the config already cast to *T, so no
// call site has to assert the type itself.
func RegisterConfig[T any](config *T, configCreator func(ctx context.Context, config *T) (any, error)) error {
	configType := reflect.TypeOf(config)
	if _, found := typeCreatorRegistry[configType]; found {
		return errors.New(configType.Name() + " is already registered")
	}
	// The registry is keyed by reflect.TypeOf(config), which is *T, and
	// CreateObject looks the creator up with the same reflect.TypeOf call, so
	// the assertion below is guaranteed to hold.
	typeCreatorRegistry[configType] = func(ctx context.Context, config any) (any, error) {
		return configCreator(ctx, config.(*T)) //nolint:forcetypeassert // see RegisterConfig
	}
	return nil
}

// CreateObject creates an object by its config. The config type must be registered through RegisterConfig().
func CreateObject(ctx context.Context, config any) (any, error) {
	configType := reflect.TypeOf(config)
	creator, found := typeCreatorRegistry[configType]
	if !found {
		return nil, errors.New(configType.String() + " is not registered")
	}
	return creator(ctx, config)
}
