package common_test

import (
	"context"
	"testing"

	. "github.com/xtls/xray-core/common"
)

type TConfig struct {
	value int
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
