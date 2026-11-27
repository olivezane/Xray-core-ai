package confloader

import (
	"context"
	"io"
	"os"

	"github.com/xtls/xray-core/common/errors"
)

type (
	configFileLoader func(string) (io.Reader, error)
)

var EffectiveConfigFileLoader configFileLoader

// SetConfigFileLoader installs the loader for config files given by path or URL.
// The loader lives in main/confloader/external, which this package cannot import,
// so the place that composes a build installs it instead of an init() function
// here and there.
func SetConfigFileLoader(loader func(string) (io.Reader, error)) {
	EffectiveConfigFileLoader = loader
}

// LoadConfig reads from a path/url/stdin
// actual work is in external module
func LoadConfig(file string) (io.Reader, error) {
	if EffectiveConfigFileLoader == nil {
		errors.LogInfo(context.Background(), "external config module not loaded, reading from stdin")
		return os.Stdin, nil
	}
	return EffectiveConfigFileLoader(file)
}
