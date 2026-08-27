package serial

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/ghodss/yaml"
	"github.com/pelletier/go-toml"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/infra/conf"
	json_reader "github.com/xtls/xray-core/infra/conf/json"
)

type offset struct {
	line int
	char int
}

func findOffset(b []byte, o int) *offset {
	if o >= len(b) || o < 0 {
		return nil
	}

	line := 1
	char := 0
	for i, x := range b {
		if i == o {
			break
		}
		if x == '\n' {
			line++
			char = 0
		} else {
			char++
		}
	}

	return &offset{line: line, char: char}
}

// decodeJSONConfig decodes into *conf.Config via conf.DecodeJSON, which owns
// the strict-mode (UseStrictJSON) semantics shared by all decoders.
func decodeJSONConfig(reader io.Reader, permissive bool) (*conf.Config, error) {
	jsonConfig := &conf.Config{}

	var r io.Reader = reader
	var jsonContent *bytes.Buffer
	if permissive {
		// Accepts JSON with Java/Python-style comments via json_reader.Reader.
		// Used for local files and stdin where the config is human-edited.
		jsonContent = bytes.NewBuffer(make([]byte, 0, 10240))
		r = io.TeeReader(&json_reader.Reader{Reader: reader}, jsonContent)
	}

	if err := conf.DecodeJSON(r, jsonConfig); err != nil {
		if permissive {
			var pos *offset
			cause := errors.Cause(err)
			switch tErr := cause.(type) {
			case *json.SyntaxError:
				pos = findOffset(jsonContent.Bytes(), int(tErr.Offset))
			case *json.UnmarshalTypeError:
				pos = findOffset(jsonContent.Bytes(), int(tErr.Offset))
			}
			if pos != nil {
				return nil, errors.New("failed to read config file at line ", pos.line, " char ", pos.char).Base(err)
			}
			return nil, errors.New("failed to read config file").Base(err)
		}
		return nil, errors.New("failed to parse remote JSON config").Base(err)
	}

	return jsonConfig, nil
}

// DecodeJSONConfig reads from reader and decode the config into *conf.Config
// syntax error could be detected.
func DecodeJSONConfig(reader io.Reader) (*conf.Config, error) {
	return decodeJSONConfig(reader, true)
}

// DecodeJSONConfigStrict reads standard RFC 8259 JSON without comment-stripping.
// Used for remote sources (http/https/http+unix) where the payload is produced by
// automated systems and cannot contain JSON5/JSONC extensions. Avoids the
// byte-by-byte comment stripper and TeeReader, which are significant overhead on
// large configs.
func DecodeJSONConfigStrict(reader io.Reader) (*conf.Config, error) {
	return decodeJSONConfig(reader, false)
}

func LoadJSONConfig(reader io.Reader) (*core.Config, error) {
	jsonConfig, err := DecodeJSONConfig(reader)
	if err != nil {
		return nil, err
	}

	pbConfig, err := jsonConfig.Build()
	if err != nil {
		return nil, errors.New("failed to parse json config").Base(err)
	}

	return pbConfig, nil
}

// DecodeTOMLConfig reads from reader and decode the config into *conf.Config
// using github.com/pelletier/go-toml and map to convert toml to json.
func DecodeTOMLConfig(reader io.Reader) (*conf.Config, error) {
	tomlFile, err := io.ReadAll(reader)
	if err != nil {
		return nil, errors.New("failed to read config file").Base(err)
	}

	configMap := make(map[string]interface{})
	if err := toml.Unmarshal(tomlFile, &configMap); err != nil {
		return nil, errors.New("failed to convert toml to map").Base(err)
	}

	jsonFile, err := json.Marshal(&configMap)
	if err != nil {
		return nil, errors.New("failed to convert map to json").Base(err)
	}

	return DecodeJSONConfig(bytes.NewReader(jsonFile))
}

func LoadTOMLConfig(reader io.Reader) (*core.Config, error) {
	tomlConfig, err := DecodeTOMLConfig(reader)
	if err != nil {
		return nil, err
	}

	pbConfig, err := tomlConfig.Build()
	if err != nil {
		return nil, errors.New("failed to parse toml config").Base(err)
	}

	return pbConfig, nil
}

// DecodeYAMLConfig reads from reader and decode the config into *conf.Config
// using github.com/ghodss/yaml to convert yaml to json.
func DecodeYAMLConfig(reader io.Reader) (*conf.Config, error) {
	yamlFile, err := io.ReadAll(reader)
	if err != nil {
		return nil, errors.New("failed to read config file").Base(err)
	}

	jsonFile, err := yaml.YAMLToJSON(yamlFile)
	if err != nil {
		return nil, errors.New("failed to convert yaml to json").Base(err)
	}

	return DecodeJSONConfig(bytes.NewReader(jsonFile))
}

func LoadYAMLConfig(reader io.Reader) (*core.Config, error) {
	yamlConfig, err := DecodeYAMLConfig(reader)
	if err != nil {
		return nil, err
	}

	pbConfig, err := yamlConfig.Build()
	if err != nil {
		return nil, errors.New("failed to parse yaml config").Base(err)
	}

	return pbConfig, nil
}
