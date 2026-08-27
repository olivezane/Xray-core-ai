package conf

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"

	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/platform"
)

type ConfigCreator func() interface{}

type JSONConfigLoader struct {
	cache     *ConfigRegistry
	idKey     string
	configKey string
}

func NewJSONConfigLoader(cache *ConfigRegistry, idKey string, configKey string) *JSONConfigLoader {
	return &JSONConfigLoader{
		idKey:     idKey,
		configKey: configKey,
		cache:     cache,
	}
}

// Register adds a creator; see ConfigRegistry.Register.
func (v *JSONConfigLoader) Register(id string, creator ConfigCreator) error {
	return v.cache.Register(id, creator)
}

// MustRegister panics on registration failure; see ConfigRegistry.MustRegister.
func (v *JSONConfigLoader) MustRegister(id string, creator ConfigCreator) {
	v.cache.MustRegister(id, creator)
}

func rejectUnknownFields() bool {
	return platform.NewEnvFlag(platform.UseStrictJSON).GetValue(func() string { return "" }) != "false"
}

// DecodeJSON decodes JSON from reader into config, rejecting unknown fields
// unless platform.UseStrictJSON is set to "false". Sole owner of the
// strict-mode semantics; shared by JSONConfigLoader and serial's decoders.
func DecodeJSON(reader io.Reader, config interface{}) error {
	dec := json.NewDecoder(reader)
	if rejectUnknownFields() {
		dec.DisallowUnknownFields()
	}
	return dec.Decode(config)
}

func (v *JSONConfigLoader) LoadWithID(raw []byte, id string) (interface{}, error) {
	id = strings.ToLower(id)
	config, err := v.cache.Create(id)
	if err != nil {
		return nil, err
	}
	if err := DecodeJSON(bytes.NewReader(raw), config); err != nil {
		return nil, err
	}
	return config, nil
}

func (v *JSONConfigLoader) Load(raw []byte) (interface{}, string, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, "", err
	}
	rawID, found := obj[v.idKey]
	if !found {
		return nil, "", errors.New(v.idKey, " not found in JSON context").AtError()
	}
	var id string
	if err := json.Unmarshal(rawID, &id); err != nil {
		return nil, "", err
	}
	var rawConfig json.RawMessage
	if len(v.configKey) > 0 {
		configValue, found := obj[v.configKey]
		if found {
			rawConfig = configValue
		} else {
			rawConfig = json.RawMessage([]byte("{}"))
		}
	} else {
		// When there's no configKey, the idKey (e.g. "type") is already consumed
		// as routing metadata. Strip it so LoadWithID doesn't reject it as unknown.
		delete(obj, v.idKey)
		cleaned, err := json.Marshal(obj)
		if err != nil {
			return nil, "", err
		}
		rawConfig = cleaned
	}
	config, err := v.LoadWithID([]byte(rawConfig), id)
	if err != nil {
		return nil, id, err
	}
	return config, id, nil
}
