package conf

import (
	"net/url"
	"strings"

	"github.com/robfig/cron/v3"
	"github.com/xtls/xray-core/app/geodata"
	"github.com/xtls/xray-core/common/errors"
	coreconfig "github.com/xtls/xray-core/core/config"
)

// GeodataAssetConfig is one asset of the geodata section. It carries the model
// type as an embedded field and adds the conversion to the message, so that the
// configuration model stays in core/config while the boundary stays here. The
// embedded fields keep their JSON names, so a configuration decodes into this
// type unchanged.
type GeodataAssetConfig struct {
	coreconfig.Asset
}

// Build converts this asset to its message.
func (c *GeodataAssetConfig) Build() (*geodata.Asset, error) {
	return buildGeodataAsset(&c.Asset)
}

func buildGeodataAsset(c *coreconfig.Asset) (*geodata.Asset, error) {
	if err := validateHTTPS(c.URL); err != nil {
		return nil, errors.New("invalid geodata asset url: ", c.URL).Base(err)
	}
	return &geodata.Asset{
		Url:  migrateGeodataURL(c.URL),
		File: c.File,
	}, nil
}

func migrateGeodataURL(assetURL string) string {
	if strings.HasSuffix(strings.ToLower(assetURL), ".dat") && (strings.HasPrefix(strings.ToLower(assetURL), "https://github.com/xtls/xray-rules-dat/") ||
		strings.HasPrefix(strings.ToLower(assetURL), "https://raw.githubusercontent.com/xtls/xray-rules-dat/")) {
		errors.PrintDeprecatedFeatureWarning(assetURL, assetURL+".gz")
		return assetURL + ".gz"
	}
	return assetURL
}

func validateHTTPS(s string) error {
	u, err := url.ParseRequestURI(s)
	if err != nil {
		return err
	}
	if u.Scheme != "https" || u.Host == "" {
		return errors.New("scheme must be https")
	}
	return nil
}

// GeodataConfig is the geodata section of the configuration.
type GeodataConfig struct {
	coreconfig.Geodata
}

// Build converts the geodata section to its message.
func (c *GeodataConfig) Build() (any, error) {
	config := &geodata.Config{}

	if c.Cron != nil {
		if _, err := cron.ParseStandard(*c.Cron); err != nil {
			return nil, errors.New("invalid geodata cron").Base(err)
		}
		config.Cron = *c.Cron
	}

	config.Outbound = c.Outbound

	assets := make([]*geodata.Asset, 0, len(c.Assets))
	for _, asset := range c.Assets {
		built, err := buildGeodataAsset(asset)
		if err != nil {
			return nil, err
		}
		assets = append(assets, built)
	}
	config.Assets = assets

	return config, nil
}
