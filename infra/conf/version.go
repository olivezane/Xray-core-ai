package conf

import (
	"strconv"

	"github.com/xtls/xray-core/app/version"
	"github.com/xtls/xray-core/core"
	coreconfig "github.com/xtls/xray-core/core/config"
)

// VersionConfig is the "version" section of the configuration model.
type VersionConfig = coreconfig.Version

// BuildVersion converts the version section to the message the instance consumes.
func BuildVersion(c *coreconfig.Version) (*version.Config, error) {
	coreVersion := strconv.Itoa(int(core.Version_x)) + "." + strconv.Itoa(int(core.Version_y)) + "." + strconv.Itoa(int(core.Version_z))

	return &version.Config{
		CoreVersion: coreVersion,
		MinVersion:  c.MinVersion,
		MaxVersion:  c.MaxVersion,
	}, nil
}
