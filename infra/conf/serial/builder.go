package serial

import (
	"context"
	"io"

	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/platform"
	creflect "github.com/xtls/xray-core/common/reflect"
	"github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/infra/conf"
	"github.com/xtls/xray-core/main/confloader"
)

// UseStrictJSON, when true, makes JSON config decoders skip the custom
// comment-stripping reader and parse input as strict RFC 8259 JSON.
//
// Enabled by setting the env variable xray.json.strict=true (or its normalized
// form XRAY_JSON_STRICT=true). Default false preserves backward-compatible
// behavior for human-edited configs that may contain comments or other
// JSON5/JSONC syntax.
var UseStrictJSON = platform.NewEnvFlag(platform.UseStrictJSON).GetValue(func() string { return "" }) == "true"

func MergeConfigFromFiles(files []*core.ConfigSource) (string, error) {
	c, err := mergeConfigs(files)
	if err != nil {
		return "", err
	}

	if j, ok := creflect.MarshalToJson(c, true); ok {
		return j, nil
	}
	return "", errors.New("marshal to json failed.")
}

func mergeConfigs(files []*core.ConfigSource) (*conf.Config, error) {
	cf := &conf.Config{}
	for i, file := range files {
		errors.LogInfo(context.Background(), "Reading config: ", file)
		r, err := confloader.LoadConfig(file.Name)
		if err != nil {
			return nil, errors.New("failed to read config: ", file).Base(err)
		}
		decoder, found := ReaderDecoderFor(file.Format)
	if !found {
		return nil, errors.New("no decoder for format ", file.Format)
	}
		if file.Format == "json" && UseStrictJSON {
			decoder = DecodeJSONConfigStrict
		}
		c, err := decoder(r)
		if err != nil {
			return nil, errors.New("failed to decode config: ", file).Base(err)
		}
		if i == 0 {
			*cf = *c
			continue
		}
		cf.Override(c, file.Name)
	}
	return cf, nil
}

func BuildConfig(files []*core.ConfigSource) (*core.Config, error) {
	config, err := mergeConfigs(files)
	if err != nil {
		return nil, err
	}
	return config.Build()
}

type readerDecoder func(io.Reader) (*conf.Config, error)

// readerDecoders is the whole list of formats this package can read, one line
// each. It is a var rather than a map filled by an init() function, so reading it
// tells you which files a build reads.
var readerDecoders = map[string]readerDecoder{
	"json": DecodeJSONConfig,
	"yaml": DecodeYAMLConfig,
	"toml": DecodeTOMLConfig,
}

// ReaderDecoderFor returns the decoder of a config format.
func ReaderDecoderFor(format string) (readerDecoder, bool) {
	decoder, found := readerDecoders[format]
	return decoder, found
}

// Install hands the config builder to core, which cannot import this package
// because this package imports core. The place that composes a build calls it.
func Install() {
	core.ConfigBuilderForFiles = BuildConfig
	core.ConfigMergedFormFiles = MergeConfigFromFiles
}
