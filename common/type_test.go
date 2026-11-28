package common_test

import (
	"context"
	"testing"

	. "github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/serial"
)

type TConfig struct {
	value int
}

// NamedConfig is a configuration that is a plain Go value, so the API carries it
// as JSON under the name it says for itself.
type NamedConfig struct {
	Value int `json:"value"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*NamedConfig) SettingsName() string {
	return "test.NamedConfig"
}

type YConfig struct {
	value string
}

func TestObjectCreation(t *testing.T) {
	f := func(ctx context.Context, t interface{}) (interface{}, error) {
		return func() int {
			return t.(*TConfig).value
		}, nil
	}

	err := SetConfigObjects([]ConfigObject{
		{Config: (*TConfig)(nil), New: f},
	})
	Must(err)

	g, err := CreateObject(context.Background(), &TConfig{value: 2})
	Must(err)
	if v := g.(func() int)(); v != 2 {
		t.Error("expect return value 2, but got ", v)
	}

	_, err = CreateObject(context.Background(), &YConfig{value: "T"})
	if err == nil {
		t.Error("expect non-nil error, but got nil")
	}
}

// TestBrokenConfigObjects checks that a mistake in the table is reported where it
// is written instead of turning into "not a configuration of this build" later.
func TestBrokenConfigObjects(t *testing.T) {
	creator := func(context.Context, interface{}) (interface{}, error) { return nil, nil }
	for _, test := range []struct {
		name   string
		list   []ConfigObject
		report string
	}{
		{name: "no config", list: []ConfigObject{{New: creator}}, report: "without a configuration type"},
		{name: "no creator", list: []ConfigObject{{Config: (*TConfig)(nil)}}, report: "has no creator"},
		{name: "twice", list: []ConfigObject{
			{Config: (*TConfig)(nil), New: creator},
			{Config: (*TConfig)(nil), New: creator},
		}, report: "listed twice"},
	} {
		err := SetConfigObjects(test.list)
		if err == nil {
			t.Fatalf("%s: broken table was accepted", test.name)
		}
		if !contains(err.Error(), test.report) {
			t.Errorf("%s: error %q does not say %q", test.name, err, test.report)
		}
	}
}

// TestSettingsOfPlainGoConfig checks that a configuration that is not a protobuf
// message still crosses the API: it says the name the API calls it, is carried as
// JSON, and arrives at the creator as the value that was wrapped.
func TestSettingsOfPlainGoConfig(t *testing.T) {
	var created interface{}
	err := SetConfigObjects([]ConfigObject{{
		Config: (*NamedConfig)(nil),
		New: func(ctx context.Context, config interface{}) (interface{}, error) {
			created = config
			return config, nil
		},
	}})
	Must(err)

	settings := serial.ToTypedMessage(&NamedConfig{Value: 7})
	if settings == nil {
		t.Fatal("a configuration that says its name was not carried")
	}
	if settings.Type != "test.NamedConfig" {
		t.Error("settings are named ", settings.Type)
	}
	if string(settings.Value) != "{\"value\":7}" {
		t.Error("settings are carried as ", string(settings.Value))
	}

	object, err := CreateObject(context.Background(), settings)
	Must(err)
	if created != object {
		t.Error("the creator did not receive the settings")
	}
	if v := object.(*NamedConfig).Value; v != 7 {
		t.Error("expect value 7, but got ", v)
	}
}

// TestSettingsWithoutName checks that a configuration which does not say the name
// the API calls it cannot cross the API: there is no other place a name could come
// from.
func TestSettingsWithoutName(t *testing.T) {
	Must(SetConfigObjects([]ConfigObject{{
		Config: (*TConfig)(nil),
		New:    func(context.Context, interface{}) (interface{}, error) { return nil, nil },
	}}))

	if settings := serial.ToTypedMessage(&TConfig{value: 1}); settings != nil {
		t.Error("a configuration without a name was carried as ", settings.Type)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
