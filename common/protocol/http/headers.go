package http

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
)

// ApplyTrustedXForwardedFor returns remoteAddr overridden by X-Forwarded-For only when a configured trusted header is present.
func ApplyTrustedXForwardedFor(header http.Header, trusted []string, remoteAddr net.Addr) net.Addr {
	value := header.Get("X-Forwarded-For")
	if value == "" {
		return remoteAddr
	}
	for _, t := range trusted {
		if len(header.Values(t)) > 0 {
			if idx := strings.IndexByte(value, ','); idx >= 0 {
				value = value[:idx]
			}
			if addr := net.ParseAddress(value); addr.Family().IsIP() {
				return &net.TCPAddr{
					IP:   addr.IP(),
					Port: 0,
				}
			}
			return remoteAddr
		}
	}
	if len(trusted) == 0 {
		errors.LogWarning(context.Background(), `received "X-Forwarded-For" from `, remoteAddr, ` but "sockopt.trustedXForwardedFor" is not configured; ignoring it and using the real remote address`)
	} else {
		errors.LogError(context.Background(), `ignored potentially forged "X-Forwarded-For" from `, remoteAddr, `: `, value)
	}
	return remoteAddr
}

// RemoveHopByHopHeaders removes hop by hop headers in http header list.
func RemoveHopByHopHeaders(header http.Header) {
	// Strip the hop-by-hop fields listed by RFC 9110 §7.6.1 (RFC 2616 §13.5.1 is
	// obsolete and called "Trailer" by its old name "Trailers"), plus the
	// non-standard Proxy-Connection.
	// https://www.mnot.net/blog/2011/07/11/what_proxies_must_do

	header.Del("Proxy-Connection")
	header.Del("Proxy-Authenticate")
	header.Del("Proxy-Authorization")
	header.Del("TE")
	header.Del("Trailer")
	header.Del("Transfer-Encoding")
	header.Del("Upgrade")
	header.Del("Keep-Alive")

	connections := header.Get("Connection")
	header.Del("Connection")
	if connections == "" {
		return
	}
	for _, h := range strings.Split(connections, ",") {
		header.Del(strings.TrimSpace(h))
	}
}

// ParseHost splits host and port from a raw string. Default port is used when raw string doesn't contain port.
func ParseHost(rawHost string, defaultPort net.Port) (net.Destination, error) {
	port := defaultPort
	host, rawPort, err := net.SplitHostPort(rawHost)
	if err != nil {
		if addrError, ok := err.(*net.AddrError); ok && strings.Contains(addrError.Err, "missing port") {
			host = rawHost
		} else {
			return net.Destination{}, err
		}
	} else if len(rawPort) > 0 {
		intPort, err := strconv.Atoi(rawPort)
		if err != nil {
			return net.Destination{}, err
		}
		port = net.Port(intPort)
	}

	return net.TCPDestination(net.ParseAddress(host), port), nil
}
