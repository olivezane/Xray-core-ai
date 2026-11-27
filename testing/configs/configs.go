// Package configs wires the configuration table for tests that build xray
// instances in process, the way the binary wires it through main/distro/all.
//
// Nothing registers itself any more, so a test that creates handlers or
// instances says which configurations it uses. Tests pass their own package's
// entries to Install, which keeps the wiring visible in the test instead of
// implicit in an import.
package configs

import (
	"reflect"

	"github.com/xtls/xray-core/app/dispatcher"
	"github.com/xtls/xray-core/app/policy"
	"github.com/xtls/xray-core/app/proxyman/inbound"
	"github.com/xtls/xray-core/app/proxyman/outbound"
	"github.com/xtls/xray-core/app/stats"
	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/errors"
)

// Defaults are the configurations every instance needs: the apps core.New adds
// when a configuration does not name them.
func Defaults() []common.ConfigObject {
	var objects []common.ConfigObject
	for _, list := range [][]common.ConfigObject{
		dispatcher.ConfigObjects,
		policy.ConfigObjects,
		stats.ConfigObjects,
		inbound.ConfigObjects,
		outbound.ConfigObjects,
	} {
		objects = append(objects, list...)
	}
	return objects
}

// Install replaces the configuration table with Defaults plus extra. A
// configuration listed twice keeps the first entry, so a test can pass a package
// that Defaults already covers without having to know that it does.
func Install(extra ...[]common.ConfigObject) error {
	objects := Defaults()
	seen := make(map[reflect.Type]bool, len(objects))
	for _, object := range objects {
		seen[reflect.TypeOf(object.Config)] = true
	}
	for _, list := range extra {
		for _, object := range list {
			if seen[reflect.TypeOf(object.Config)] {
				continue
			}
			seen[reflect.TypeOf(object.Config)] = true
			objects = append(objects, object)
		}
	}
	return common.SetConfigObjects(objects)
}

// MustInstall is Install for a TestMain, which has no *testing.T.
func MustInstall(extra ...[]common.ConfigObject) {
	if err := Install(extra...); err != nil {
		panic(errors.New("failed to install the configuration table").Base(err))
	}
}
