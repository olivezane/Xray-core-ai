package conf_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/xtls/xray-core/common"
	. "github.com/xtls/xray-core/infra/conf"
	"google.golang.org/protobuf/proto"
)

func loadJSON(creator func() Buildable) func(string) (any, error) {
	return func(s string) (any, error) {
		instance := creator()
		if err := json.Unmarshal([]byte(s), instance); err != nil {
			return nil, err
		}
		return instance.Build()
	}
}

type TestCase struct {
	Input  string
	Parser func(string) (any, error)
	Output any
}

// equalConfig compares what a parser produced with what a case expects. A message
// is compared as a message; a configuration, which is no longer one, by the JSON
// of its sections, because its settings are pointers and values that only mean
// something once written out.
func equalConfig(actual, expected any) bool {
	actualMessage, actualIsMessage := actual.(proto.Message)
	expectedMessage, expectedIsMessage := expected.(proto.Message)
	if actualIsMessage && expectedIsMessage {
		return proto.Equal(actualMessage, expectedMessage)
	}
	return configJSON(actual) == configJSON(expected)
}

func configJSON(value any) string {
	written, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprint(value)
	}
	return string(written)
}

func runMultiTestCase(t *testing.T, testCases []TestCase) {
	for _, testCase := range testCases {
		actual, err := testCase.Parser(testCase.Input)
		common.Must(err)
		if !equalConfig(actual, testCase.Output) {
			t.Fatalf("Failed in test case:\n%s\nActual:\n%s\nExpected:\n%s", testCase.Input, configJSON(actual), configJSON(testCase.Output))
		}
	}
}
