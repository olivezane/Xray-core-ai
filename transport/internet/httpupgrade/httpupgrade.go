package httpupgrade

import (
	"net/http"

	"golang.org/x/net/http/httpguts"
)

const protocolName = "httpupgrade"

// headerHasToken reports whether the named header field carries token. Both
// Connection and Upgrade are comma-separated lists (RFC 9110 §5.6.1), and an
// upgrade only asks for the token to be present (RFC 6455 §4.2.1).
func headerHasToken(header http.Header, name, token string) bool {
	return httpguts.HeaderValuesContainsToken(header.Values(name), token)
}
