package common

import (
	"context"
	"encoding/json"
	"reflect"

	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/serial"
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
	// Also lists types this package carries that a configuration does not create,
	// e.g. the account of a proxy: they cross the API by name like a setting, so
	// the name has to be read here even though nothing is built from them.
	Also []interface{}
}

var (
	configObjects = make(map[reflect.Type]ConfigCreator)
	// configTypes is the one way a setting says what it is: the name it gives
	// itself, to the type it belongs to.
	configTypes = make(map[string]reflect.Type)
)

// SetConfigObjects installs list as the complete table of this build. A
// configuration that is not listed cannot create anything, and a broken table is
// reported where it is written instead of when the object is first needed.
func SetConfigObjects(list []ConfigObject) error {
	installed := make(map[reflect.Type]ConfigCreator, len(list))
	installedTypes := make(map[string]reflect.Type, len(list))
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
		// A settings type that is a plain Go value says its own name, so the name
		// lives with the type instead of being repeated here.
		if named, ok := object.Config.(serial.SettingsNamer); ok {
			name := named.SettingsName()
			if name == "" {
				return errors.New(configType.String(), " has no name")
			}
			if _, found := installedTypes[name]; found {
				return errors.New(name, " is listed twice")
			}
			installedTypes[name] = configType
		}
		installed[configType] = object.New
		for _, extra := range object.Also {
			if extra == nil {
				return errors.New(configType.String(), " lists a nil settings type")
			}
			extraType := reflect.TypeOf(extra)
			extraName := ""
			if named, ok := extra.(serial.SettingsNamer); ok {
				extraName = named.SettingsName()
			}
			if extraName == "" {
				return errors.New(extraType.String(), " has no name")
			}
			if _, found := installedTypes[extraName]; found {
				return errors.New(extraName, " is listed twice")
			}
			installedTypes[extraName] = extraType
		}
	}
	configObjects = installed
	configTypes = installedTypes
	return nil
}

// SettingsType names a settings type that is not created from the configuration
// table: a stream transport and a mask create their settings themselves, but the
// settings still cross the API by name, so the name has to be read somewhere.
type SettingsType struct {
	Config interface{}
}

// SetSettingsTypes adds settings types to the names the build can read. The
// configuration table is installed first, and this adds to it.
func SetSettingsTypes(list []SettingsType) error {
	installedTypes := make(map[string]reflect.Type, len(configTypes)+len(list))
	for name, configType := range configTypes {
		installedTypes[name] = configType
	}
	for _, settings := range list {
		if settings.Config == nil {
			return errors.New("settings type without a configuration type")
		}
		configType := reflect.TypeOf(settings.Config)
		named, ok := settings.Config.(serial.SettingsNamer)
		if !ok {
			return errors.New(configType.String(), " has no name")
		}
		name := named.SettingsName()
		if name == "" {
			return errors.New(configType.String(), " has no name")
		}
		if _, found := installedTypes[name]; found {
			return errors.New(name, " is listed twice")
		}
		installedTypes[name] = configType
	}
	configTypes = installedTypes
	return nil
}

// UnwrapSettings returns the value of a setting. A setting may still arrive as a
// typed message, from a configuration written for the API, and is unwrapped here
// so the rest of the code sees the value.
//
// A setting that is a protobuf message is carried by the name protobuf gives it.
// One that is a plain Go value is carried as JSON under the name the table gives
// it, so a configuration can stop being a protobuf message without changing what
// the API carries.
func UnwrapSettings(settings interface{}) interface{} {
	typed, ok := settings.(*serial.TypedMessage)
	if !ok {
		return settings
	}
	if configType, found := configTypes[typed.Type]; found {
		if value, ok := UnwrapSettingsOf(typed, configType); ok {
			return value
		}
	}
	if instance, err := typed.GetInstance(); err == nil {
		return instance
	}
	return settings
}

// UnwrapSettingsOf reads a setting as the given type: a protobuf message by the
// name protobuf gives it, a plain Go value as JSON. It is what a package uses when
// it keeps its own settings types, e.g. the stream transports.
func UnwrapSettingsOf(typed *serial.TypedMessage, configType reflect.Type) (interface{}, bool) {
	if typed == nil || configType == nil {
		return nil, false
	}
	if instance, err := typed.GetInstance(); err == nil {
		return instance, true
	}
	value := reflect.New(configType.Elem())
	// A setting written without a value is the zero value of its type.
	if len(typed.Value) > 0 {
		if err := json.Unmarshal(typed.Value, value.Interface()); err != nil {
			return nil, false
		}
	}
	return value.Interface(), true
}

// CreateObject creates an object by its config. The config type must be listed
// in the table installed by SetConfigObjects.
func CreateObject(ctx context.Context, config interface{}) (interface{}, error) {
	config = UnwrapSettings(config)
	configType := reflect.TypeOf(config)
	creator, found := configObjects[configType]
	if !found {
		return nil, errors.New(configType.String(), " is not a configuration of this build; configurations are listed in main/distro/all")
	}
	return creator(ctx, config)
}
