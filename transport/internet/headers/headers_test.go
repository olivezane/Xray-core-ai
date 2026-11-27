package headers

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/xtls/xray-core/transport/internet"
	"github.com/xtls/xray-core/transport/internet/headers/http"
	"github.com/xtls/xray-core/transport/internet/headers/noop"
)

// TestAllIsInstalled guards that the list above is what internet looks up: a
// header that is listed but not installed, or the other way round, is a header
// nobody can use.
func TestAllIsInstalled(t *testing.T) {
	for _, header := range All {
		switch header.Config.(type) {
		case *noop.Config:
			if _, err := internet.CreatePacketHeader(header.Config); err != nil {
				t.Errorf("noop packet header is listed but cannot be created: %v", err)
			}
		case *noop.ConnectionConfig, *http.Config:
			if _, err := internet.CreateConnectionAuthenticator(header.Config); err != nil {
				t.Errorf("connection header for %T is listed but cannot be created: %v", header.Config, err)
			}
		default:
			t.Errorf("header for %T is listed but this test does not know how to create it", header.Config)
		}
	}
}

// TestAllIsWired checks every line of the list: a configuration type and a
// constructor, each configuration type listed once.
func TestAllIsWired(t *testing.T) {
	seen := make(map[reflect.Type]bool, len(All))
	for _, header := range All {
		if header.Config == nil {
			t.Error("a listed header has no configuration type")
			continue
		}
		if header.New == nil {
			t.Errorf("header for %T has no constructor", header.Config)
		}
		configType := reflect.TypeOf(header.Config)
		if seen[configType] {
			t.Errorf("header for %v is listed twice", configType)
		}
		seen[configType] = true
	}
}

// TestUnknownHeaderIsReported checks what an unlisted configuration tells the
// user: the type, and where the headers of this build are listed.
func TestUnknownHeaderIsReported(t *testing.T) {
	_, err := internet.CreateConnectionAuthenticator(&struct{ Unlisted bool }{})
	if err == nil {
		t.Fatal("creating a header from an unlisted configuration succeeded")
	}
	if !strings.Contains(err.Error(), "transport/internet/headers") || !strings.Contains(err.Error(), "struct") {
		t.Errorf("error does not name the configuration type and where headers are listed: %v", err)
	}
}

// TestBrokenListIsReported checks that a mistake in the list is reported where it
// is written instead of turning into "no header" at runtime.
func TestBrokenListIsReported(t *testing.T) {
	constructor := func(context.Context, interface{}) (interface{}, error) { return nil, nil }
	for _, test := range []struct {
		name   string
		list   []internet.Header
		report string
	}{
		{name: "no config", list: []internet.Header{{New: constructor}}, report: "without a configuration type"},
		{name: "no constructor", list: []internet.Header{{Config: (*noop.Config)(nil)}}, report: "has no constructor"},
		{name: "twice", list: []internet.Header{
			{Config: (*noop.Config)(nil), New: constructor},
			{Config: (*noop.Config)(nil), New: constructor},
		}, report: "listed twice"},
	} {
		err := internet.SetHeaders(test.list)
		if err == nil {
			t.Fatalf("%s: broken header list was accepted", test.name)
		}
		if !strings.Contains(err.Error(), test.report) {
			t.Errorf("%s: error %q does not say %q", test.name, err, test.report)
		}
	}
	// The rejected lists must not have replaced what was installed.
	if _, err := internet.CreatePacketHeader((*noop.Config)(nil)); err != nil {
		t.Errorf("a rejected header list was installed anyway: %v", err)
	}
}
