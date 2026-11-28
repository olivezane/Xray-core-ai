package core

import (
	"io"
	"slices"
	"strings"

	"github.com/xtls/xray-core/common/cmdarg"
	"github.com/xtls/xray-core/common/errors"
)

// ConfigFormat is a configurable format of Xray config file.
type ConfigFormat struct {
	Name      string
	Extension []string
	Loader    ConfigLoader
}

type ConfigSource struct {
	Name   string
	Format string
}

// ConfigLoader is a utility to load Xray config from external source.
type ConfigLoader func(input interface{}) (*Config, error)

// ConfigBuilder is a builder to build core.Config from filenames and formats
type ConfigBuilder func(files []*ConfigSource) (*Config, error)

// ConfigsMerger merges multiple json configs into a single one
type ConfigsMerger func(files []*ConfigSource) (string, error)

var (
	configLoaderByName    = make(map[string]*ConfigFormat)
	configLoaderByExt     = make(map[string]*ConfigFormat)
	ConfigBuilderForFiles ConfigBuilder
	ConfigMergedFormFiles ConfigsMerger
)

// DefaultConfigFormats returns the configuration formats of core itself. Core
// carries none: a configuration is JSON, TOML or YAML, and the formats are added
// by the packages that read them. Protobuf is not a configuration format: it is
// the shape a value takes when it crosses the API.
func DefaultConfigFormats() []*ConfigFormat {
	return nil
}

// SetConfigFormats installs the configuration formats of a build, one line per
// format at the place that composes the build. A format is not registered from an
// init() function, so reading the place that calls this tells you which config
// files a build can read.
func SetConfigFormats(formats []*ConfigFormat) error {
	byName := make(map[string]*ConfigFormat, len(formats))
	byExt := make(map[string]*ConfigFormat, len(formats))
	for _, format := range formats {
		if format == nil || format.Name == "" || format.Loader == nil {
			return errors.New("config format without a name or a loader")
		}
		name := strings.ToLower(format.Name)
		if _, found := byName[name]; found {
			return errors.New(format.Name, " is listed twice")
		}
		byName[name] = format

		for _, ext := range format.Extension {
			lext := strings.ToLower(ext)
			if f, found := byExt[lext]; found {
				return errors.New(ext, " is claimed by both ", format.Name, " and ", f.Name)
			}
			byExt[lext] = format
		}
	}
	configLoaderByName = byName
	configLoaderByExt = byExt
	return nil
}

func GetMergedConfig(args cmdarg.Arg) (string, error) {
	var files []*ConfigSource
	supported := []string{"json", "yaml", "toml"}
	for _, file := range args {
		format := "json"
		if file != "stdin:" {
			format = GetFormat(file)
		}

		if slices.Contains(supported, format) {
			files = append(files, &ConfigSource{
				Name:   file,
				Format: format,
			})
		}
	}
	return ConfigMergedFormFiles(files)
}

func GetFormatByExtension(ext string) string {
	switch strings.ToLower(ext) {
	case "pb", "protobuf":
		return "protobuf"
	case "yaml", "yml":
		return "yaml"
	case "toml":
		return "toml"
	case "json", "jsonc":
		return "json"
	default:
		return ""
	}
}

func getExtension(filename string) string {
	idx := strings.LastIndexByte(filename, '.')
	if idx == -1 {
		return ""
	}
	return filename[idx+1:]
}

func GetFormat(filename string) string {
	return GetFormatByExtension(getExtension(filename))
}

func LoadConfig(formatName string, input interface{}) (*Config, error) {
	switch v := input.(type) {
	case cmdarg.Arg:
		files := make([]*ConfigSource, len(v))
		hasProtobuf := false
		for i, file := range v {
			var f string

			if formatName == "auto" {
				if file != "stdin:" {
					f = GetFormat(file)
				} else {
					f = "json"
				}
			} else {
				f = formatName
			}

			if f == "" {
				return nil, errors.New("Failed to get format of ", file)
			}

			if f == "protobuf" {
				hasProtobuf = true
			}
			files[i] = &ConfigSource{
				Name:   file,
				Format: f,
			}
		}

		// Protobuf is not a configuration format: it is the shape a value takes
		// when it crosses the API, and nothing reads a configuration written that
		// way.
		if hasProtobuf {
			return nil, errors.New("protobuf is not a configuration format; write JSON, TOML or YAML")
		}

		// to avoid import cycle
		return ConfigBuilderForFiles(files)
	case io.Reader:
		if f, found := configLoaderByName[formatName]; found {
			return f.Loader(v)
		} else {
			return nil, errors.New("Unable to load config in", formatName)
		}
	}

	return nil, errors.New("Unable to load config")
}


