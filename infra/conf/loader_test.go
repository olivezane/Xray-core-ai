package conf_test

import (
	"strings"
	"testing"

	. "github.com/xtls/xray-core/infra/conf"
)

func TestJSONConfigLoader_RejectsUnknownSettings(t *testing.T) {
	cache := NewConfigRegistry()
	if err := cache.Register("freedom", func() interface{} {
		return new(FreedomConfig)
	}); err != nil {
		t.Fatal(err)
	}
	loader := NewJSONConfigLoader(cache, "protocol", "settings")

	raw := []byte(`{"protocol": "freedom", "settings": {"unknwn": "value"}}`)
	_, _, err := loader.Load(raw)
	if err == nil {
		t.Fatal("expected error for unknown field in settings, got nil")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected error about unknown field, got: %v", err)
	}
}

func TestJSONConfigLoader_AcceptsValidSettings(t *testing.T) {
	cache := NewConfigRegistry()
	if err := cache.Register("freedom", func() interface{} {
		return new(FreedomConfig)
	}); err != nil {
		t.Fatal(err)
	}
	loader := NewJSONConfigLoader(cache, "protocol", "settings")

	raw := []byte(`{"protocol": "freedom", "settings": {"domainStrategy": "AsIs"}}`)
	_, _, err := loader.Load(raw)
	if err != nil {
		t.Fatalf("expected no error for valid settings, got: %v", err)
	}
}

func TestConfigRegistry_FailureSurfacesWithKey(t *testing.T) {
	registry := NewConfigRegistry()
	if err := registry.Register("freedom", func() interface{} { return new(FreedomConfig) }); err != nil {
		t.Fatal(err)
	}
	// Duplicate registration fails with the offending key.
	err := registry.Register("freedom", func() interface{} { return new(FreedomConfig) })
	if err == nil || !strings.Contains(err.Error(), "freedom") {
		t.Fatalf("expected duplicate-registration error naming the key, got: %v", err)
	}
	// A non-lowercase key can never be matched by the lowercasing loader;
	// reject it at registration time, naming the key.
	err = registry.Register("Freedom", func() interface{} { return new(FreedomConfig) })
	if err == nil || !strings.Contains(err.Error(), "Freedom") {
		t.Fatalf("expected non-lowercase registration error naming the key, got: %v", err)
	}
	// MustRegister panics (init-time), carrying the offending key.
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected MustRegister to panic on duplicate")
		} else if !strings.Contains(r.(error).Error(), "freedom") {
			t.Fatalf("expected panic naming the key, got: %v", r)
		}
	}()
	registry.MustRegister("freedom", func() interface{} { return new(FreedomConfig) })
}

func TestJSONConfigLoader_RejectsUnknownInPlainSettings(t *testing.T) {
	cache := NewConfigRegistry()
	if err := cache.Register("freedom", func() interface{} {
		return new(FreedomConfig)
	}); err != nil {
		t.Fatal(err)
	}
	loader := NewJSONConfigLoader(cache, "protocol", "")

	raw := []byte(`{"protocol": "freedom", "unknownKey": "nope"}`)
	_, _, err := loader.Load(raw)
	if err == nil {
		t.Fatal("expected error for unknown field, got nil")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected error about unknown field, got: %v", err)
	}
}
