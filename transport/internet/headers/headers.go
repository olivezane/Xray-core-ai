// Package headers is the one place where Xray wires its stream headers.
//
// Each line names the configuration a header is created from and the function
// that creates it, so this file is the whole picture: a header does not register
// itself from an init() function, and nothing else calls into a header package
// to make it usable.
package headers

import (
	"context"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/transport/internet"
	"github.com/xtls/xray-core/transport/internet/headers/http"
	"github.com/xtls/xray-core/transport/internet/headers/noop"
)

// All is every stream header of this build, one line each.
var All = []internet.Header{
	{Config: (*http.Config)(nil), New: func(ctx context.Context, config interface{}) (interface{}, error) {
		return http.NewAuthenticator(ctx, config.(*http.Config))
	}},
	{Config: (*noop.Config)(nil), New: noop.NewNoOpHeader},
	{Config: (*noop.ConnectionConfig)(nil), New: noop.NewNoOpConnectionHeader},
}

func init() {
	common.Must(internet.SetHeaders(All))
	// A header's settings cross the API by name like a transport's, so the names
	// of this build are installed beside the headers.
	settings := make([]interface{}, 0, len(All))
	for _, header := range All {
		settings = append(settings, header.Config)
	}
	common.Must(internet.SetStreamSettings(settings))
}
