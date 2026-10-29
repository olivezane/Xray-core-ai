package http

import (
	"bytes"
	"context"
	"errors"
	"strings"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/session"
)

type version byte

const (
	HTTP1 version = iota
	HTTP2
)

type SniffHeader struct {
	version version
	host    string
}

func (h *SniffHeader) Protocol() string {
	switch h.version {
	case HTTP1:
		return "http1"
	case HTTP2:
		return "http2"
	default:
		return "unknown"
	}
}

func (h *SniffHeader) Domain() string {
	return h.host
}

var (
	methods = [...]string{"get", "post", "head", "put", "delete", "options", "connect", "patch", "trace"}

	errNotHTTPMethod = errors.New("not an HTTP method")
)

func beginWithHTTPMethod(b []byte) error {
	for _, m := range &methods {
		if len(b) < len(m) {
			// Only ask for more data when the buffer is still a prefix of this method.
			if strings.EqualFold(string(b), m[:len(b)]) {
				return common.ErrNoClue
			}
			continue
		}

		if strings.EqualFold(string(b[:len(m)]), m) {
			return nil
		}
	}

	return errNotHTTPMethod
}

func SniffHTTP(b []byte, c context.Context) (*SniffHeader, error) {
	content := session.ContentFromContext(c)
	ShouldSniffAttr := true
	// If content.Attributes have information, that means it comes from HTTP inbound PlainHTTP mode.
	// It will set attributes, so skip it.
	if content == nil || len(content.Attributes) != 0 {
		ShouldSniffAttr = false
	}
	if err := beginWithHTTPMethod(b); err != nil {
		return nil, err
	}

	sh := &SniffHeader{
		version: HTTP1,
	}

	headers := bytes.Split(b, []byte{'\n'})
	for i := 1; i < len(headers); i++ {
		// A CRLF-terminated empty line ends the header section, RFC 9112 Section 2.2.
		header := bytes.TrimSuffix(headers[i], []byte{'\r'})
		if len(header) == 0 {
			break
		}
		parts := bytes.SplitN(header, []byte{':'}, 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(string(parts[0]))
		value := string(bytes.TrimSpace(parts[1]))
		if ShouldSniffAttr {
			content.SetAttribute(key, value) // Put header in attribute
		}
		if key == "host" {
			rawHost := strings.ToLower(value)
			dest, err := ParseHost(rawHost, net.Port(80))
			if err != nil {
				return nil, err
			}
			sh.host = dest.Address.String()
		}
	}
	// Parse request line
	// Request line is like this
	// "GET /homo/114514 HTTP/1.1"
	if len(headers) > 0 {
		RequestLineParts := bytes.Split(bytes.TrimSuffix(headers[0], []byte{'\r'}), []byte{' '})
		if len(RequestLineParts) == 3 {
			if ShouldSniffAttr {
				content.SetAttribute(":method", string(RequestLineParts[0]))
				content.SetAttribute(":path", string(RequestLineParts[1]))
			}
			// RFC 9112 §3.2: the authority of an absolute-form target replaces the
			// Host field (§3.2.2), CONNECT carries it in the target itself (§3.2.3).
			if host, ok := targetAuthority(RequestLineParts[0], RequestLineParts[1]); ok {
				sh.host = host
			}
		}
	}

	if len(sh.host) > 0 {
		return sh, nil
	}

	return nil, common.ErrNoClue
}

// targetAuthority returns the host of an absolute-form request-target
// (RFC 9112 §3.2.2) or of the authority-form target of a CONNECT request
// (§3.2.3). The Host field is only a fallback for the other forms, so ok is
// false when the target carries no authority at all.
func targetAuthority(method, target []byte) (string, bool) {
	authority := []byte(nil)
	if bytes.EqualFold(method, []byte("connect")) {
		authority = target
	} else if scheme, after, found := bytes.Cut(target, []byte("://")); found && isScheme(scheme) {
		authority = after
		if i := bytes.IndexAny(authority, "/#?"); i >= 0 {
			authority = authority[:i]
		}
	}
	if len(authority) == 0 {
		return "", false
	}
	dest, err := ParseHost(strings.ToLower(string(authority)), net.Port(80))
	if err != nil || dest.Address == nil || len(dest.Address.String()) == 0 {
		return "", false
	}
	return dest.Address.String(), true
}

// isScheme reports whether s is a URI scheme (RFC 3986 §3.1). Without this check
// an origin-form target like "/r?u=http://other.com/" would pass for absolute-form
// and let a path segment take over the destination.
func isScheme(s []byte) bool {
	for i, c := range s {
		switch {
		case c|0x20 >= 'a' && c|0x20 <= 'z':
		case i > 0 && (c >= '0' && c <= '9' || c == '+' || c == '-' || c == '.'):
		default:
			return false
		}
	}
	return len(s) > 0
}
