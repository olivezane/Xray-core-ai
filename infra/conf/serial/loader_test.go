package serial_test

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/xtls/xray-core/app/log"
	"github.com/xtls/xray-core/common/cmdarg"
	clog "github.com/xtls/xray-core/common/log"
	"github.com/xtls/xray-core/infra/conf/serial"
	"github.com/xtls/xray-core/main/confloader"
)

// Default (env unset) must match upstream: unknown fields are allowed.
func TestDecodeJSONConfig_AllowsUnknownFieldsByDefault(t *testing.T) {
	unknownField := `{
		"outbound": [{
			"protocol": "freedom"
		}]
	}`
	reader := bytes.NewReader([]byte(unknownField))
	_, err := serial.DecodeJSONConfig(reader)
	if err != nil {
		t.Fatalf("expected unknown fields to be allowed by default (upstream behavior), got: %v", err)
	}
}

// Strict mode (xray.json.strict=true) is the fork's explicit opt-in: it
// rejects unknown fields in addition to skipping comment-stripping.
func TestDecodeJSONConfig_RejectsUnknownFieldsWhenStrict(t *testing.T) {
	t.Setenv("XRAY_JSON_STRICT", "true")
	unknownField := `{
		"outbound": [{
			"protocol": "freedom"
		}]
	}`
	reader := bytes.NewReader([]byte(unknownField))
	_, err := serial.DecodeJSONConfig(reader)
	if err == nil {
		t.Fatal("expected error for unknown field 'outbound' in strict mode, got nil")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected error about unknown field, got: %v", err)
	}
}

func TestDecodeJSONConfig_AllowsUnknownWhenPermissive(t *testing.T) {
	t.Setenv("XRAY_JSON_STRICT", "false")
	unknownField := `{
		"outbound": [{
			"protocol": "freedom"
		}]
	}`
	reader := bytes.NewReader([]byte(unknownField))
	_, err := serial.DecodeJSONConfig(reader)
	if err != nil {
		t.Fatalf("expected no error when XRAY_JSON_STRICT=false, got: %v", err)
	}
}

func TestDecodeJSONConfig_AcceptsValidConfig(t *testing.T) {
	valid := `{
		"log": {
			"loglevel": "info"
		},
		"inbounds": [{
			"port": 1080,
			"listen": "127.0.0.1",
			"protocol": "socks",
			"settings": {
				"auth": "noauth",
				"udp": true
			}
		}]
	}`
	reader := bytes.NewReader([]byte(valid))
	_, err := serial.DecodeJSONConfig(reader)
	if err != nil {
		t.Fatalf("expected no error for valid config, got: %v", err)
	}
}

func TestDecodeTOMLConfig_AllowsUnknownFieldsByDefault(t *testing.T) {
	unknownField := `log_level = "info"
outbound = []`
	reader := bytes.NewReader([]byte(unknownField))
	_, err := serial.DecodeTOMLConfig(reader)
	if err != nil {
		t.Fatalf("expected unknown fields to be allowed by default in TOML, got: %v", err)
	}
}

func TestDecodeTOMLConfig_RejectsUnknownFieldsWhenStrict(t *testing.T) {
	t.Setenv("XRAY_JSON_STRICT", "true")
	unknownField := `log_level = "info"
outbound = []`
	reader := bytes.NewReader([]byte(unknownField))
	_, err := serial.DecodeTOMLConfig(reader)
	if err == nil {
		t.Fatal("expected error for unknown field 'outbound' in strict TOML, got nil")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected error about unknown field, got: %v", err)
	}
}

func TestDecodeYAMLConfig_AllowsUnknownFieldsByDefault(t *testing.T) {
	unknownField := `log:
  loglevel: info
outbound: []`
	reader := bytes.NewReader([]byte(unknownField))
	_, err := serial.DecodeYAMLConfig(reader)
	if err != nil {
		t.Fatalf("expected unknown fields to be allowed by default in YAML, got: %v", err)
	}
}

func TestDecodeYAMLConfig_RejectsUnknownFieldsWhenStrict(t *testing.T) {
	t.Setenv("XRAY_JSON_STRICT", "true")
	unknownField := `log:
  loglevel: info
outbound: []`
	reader := bytes.NewReader([]byte(unknownField))
	_, err := serial.DecodeYAMLConfig(reader)
	if err == nil {
		t.Fatal("expected error for unknown field 'outbound' in strict YAML, got nil")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected error about unknown field, got: %v", err)
	}
}

func TestLoaderError(t *testing.T) {
	testCases := []struct {
		Input  string
		Output string
	}{
		{
			Input: `{
				"log": {
					// abcd
					0,
					"loglevel": "info"
				}
		}`,
			Output: "line 4 char 6",
		},
		{
			Input: `{
				"log": {
					// abcd
					"loglevel": "info",
				}
		}`,
			Output: "line 5 char 5",
		},
		{
			Input: `{
				"inbounds": [{
					"protocol": "test",
					"port": 1
				}]
		}`,
			Output: "parse json config",
		},
		{
			Input: `{
				"inbounds": [{
					"port": 1,
					"listen": 0,
					"protocol": "test"
				}]
		}`,
			Output: "line 1 char 1",
		},
	}
	for _, testCase := range testCases {
		reader := bytes.NewReader([]byte(testCase.Input))
		_, err := serial.LoadJSONConfig(reader)
		errString := err.Error()
		if !strings.Contains(errString, testCase.Output) {
			t.Error("unexpected output from json: ", testCase.Input, ". expected ", testCase.Output, ", but actually ", errString)
		}
	}
}

func TestLoadMultiFileConfig(t *testing.T) {
	files := map[string]string{
		"a.json": `{"log":{"loglevel":"warning"},"inbounds":[{"protocol":"dokodemo-door","port":12345,"settings":{"address":"1.2.3.4"}}]}`,
		"b.json": `{"log":{"loglevel":"debug"}}`,
	}
	confloader.EffectiveConfigFileLoader = func(file string) (io.Reader, error) {
		if c, ok := files[file]; ok {
			return strings.NewReader(c), nil
		}
		return nil, io.EOF
	}
	t.Cleanup(func() { confloader.EffectiveConfigFileLoader = nil })

	cfg, err := serial.LoadMultiFileConfig(cmdarg.Arg{"a.json", "b.json"}, serial.DecodeJSONConfig)
	if err != nil {
		t.Fatal(err)
	}
	var sawLogApp, sawProxymanApp bool
	for _, app := range cfg.App {
		switch m := app.Type; {
		case strings.Contains(m, "xray.app.log.Config"):
			msg, err := app.GetInstance()
			if err != nil {
				t.Fatal(err)
			}
			logCfg, ok := msg.(*log.Config)
			if !ok {
				t.Fatalf("unexpected app type %T", msg)
			}
			sawLogApp = true
			if logCfg.ErrorLogLevel != clog.Severity_Debug {
				t.Fatalf("later config files must override overlapping settings, got loglevel %v", logCfg.ErrorLogLevel)
			}
		case strings.Contains(m, "xray.app.proxyman"):
			sawProxymanApp = true
		}
	}
	if !sawLogApp {
		t.Fatal("log app missing from built config")
	}
	if !sawProxymanApp {
		t.Fatal("settings present only in earlier files must be kept: inbounds from a.json lost")
	}
}
