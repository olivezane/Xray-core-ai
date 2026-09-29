package serial_test

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	. "github.com/xtls/xray-core/common/serial"
)

func TestToString(t *testing.T) {
	s := "a"
	data := []struct {
		Value  any
		String string
	}{
		{Value: s, String: s},
		{Value: &s, String: s},
		{Value: errors.New("t"), String: "t"},
		{Value: []byte{'b', 'c'}, String: "[98 99]"},
	}

	for _, c := range data {
		if r := cmp.Diff(ToString(c.Value), c.String); r != "" {
			t.Error(r)
		}
	}
}

func TestConcat(t *testing.T) {
	testCases := []struct {
		Input  []any
		Output string
	}{
		{
			Input: []any{
				"a", "b",
			},
			Output: "ab",
		},
	}

	for _, testCase := range testCases {
		actual := Concat(testCase.Input...)
		if actual != testCase.Output {
			t.Error("Unexpected output: ", actual, " but want: ", testCase.Output)
		}
	}
}

func BenchmarkConcat(b *testing.B) {
	input := []any{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"}

	b.ReportAllocs()
	for b.Loop() {
		_ = Concat(input...)
	}
}
