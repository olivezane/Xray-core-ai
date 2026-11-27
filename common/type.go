package common

import (
	"context"
	"reflect"

	"github.com/xtls/xray-core/common/errors"
)

// ConfigCreator is a function to create an object by a config.
type ConfigCreator func(ctx context.Context, config interface{}) (interface{}, error)

// ConfigObject is one line of the table that says which configuration creates
// which object: the configuration it is created from and the creator itself.
//
// Objects do not register themselves from init() functions any more. Every
// feature of a build adds its line to main/distro/all, and that list is what
// SetConfigObjects installs, so reading it tells you what a configuration can
// create and who creates it.
type ConfigObject struct {
	// Config is the configuration the object is created from, e.g.
	// (*Config)(nil).
	Config interface{}
	// New creates the object from a configuration of that type.
	New ConfigCreator
}

var configObjects = make(map[reflect.Type]ConfigCreator)

// SetConfigObjects installs list as the complete table of this build. A
// configuration that is not listed cannot create anything, and a broken table is
// reported where it is written instead of when the object is first needed.
func SetConfigObjects(list []ConfigObject) error {
	installed := make(map[reflect.Type]ConfigCreator, len(list))
	for _, object := range list {
		if object.Config == nil {
			return errors.New("config object without a configuration type")
		}
		if object.New == nil {
			return errors.New("config object for ", reflect.TypeOf(object.Config), " has no creator")
		}
		configType := reflect.TypeOf(object.Config)
		if _, found := installed[configType]; found {
			return errors.New(configType.String(), " is listed twice")
		}
		installed[configType] = object.New
	}
	configObjects = installed
	return nil
}

// CreateObject creates an object by its config. The config type must be listed
// in the table installed by SetConfigObjects.
func CreateObject(ctx context.Context, config interface{}) (interface{}, error) {
	configType := reflect.TypeOf(config)
	creator, found := configObjects[configType]
	if !found {
		return nil, errors.New(configType.String(), " is not a configuration of this build; configurations are listed in main/distro/all")
	}
	return creator(ctx, config)
}
