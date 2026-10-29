package dns

import (
	"context"
	gotls "crypto/tls"
	"net/url"

	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/net/cnc"
	"github.com/xtls/xray-core/features/routing"
	"github.com/xtls/xray-core/transport/internet"
	"github.com/xtls/xray-core/transport/internet/tls"
)

// TLSNameServer implemented DNS over TLS (RFC7858): DNS over TCP (RFC7766) carried
// in a TLS session on port 853 (RFC7858 §3.1). TLS is a byte stream, so the two-octet
// length field of RFC1035 §4.2.2 is still needed inside the session (RFC7858 §3.3).
//
// shortcut: no EDNS(0) block-length padding (RFC8467 §4.1) and one connection per
// query instead of pipelining (RFC7858 §3.4), upgrade if traffic analysis or
// handshake cost matters.
type TLSNameServer struct {
	*TCPNameServer
	tlsConfig *gotls.Config
}

// NewTLSNameServer creates DNS over TLS server object for remote resolving.
func NewTLSNameServer(
	url *url.URL,
	dispatcher routing.Dispatcher,
	disableCache bool, serveStale bool, serveExpiredTTL uint32,
	clientIP net.IP,
) (*TLSNameServer, error) {
	s, err := baseTLSNameServer(url, "TLS", disableCache, serveStale, serveExpiredTTL, clientIP)
	if err != nil {
		return nil, err
	}

	s.dial = func(ctx context.Context) (net.Conn, error) {
		link, err := dispatcher.Dispatch(toDnsContext(ctx, s.destination.String()), *s.destination)
		if err != nil {
			return nil, err
		}

		return s.handshake(ctx, cnc.NewConnection(
			cnc.ConnectionInputMulti(link.Writer),
			cnc.ConnectionOutputMulti(link.Reader),
		))
	}

	errors.LogInfo(context.Background(), "DNS: created TLS client initialized for ", url.String())
	return s, nil
}

// NewTLSLocalNameServer creates DNS over TLS client object for local resolving
func NewTLSLocalNameServer(url *url.URL, disableCache bool, serveStale bool, serveExpiredTTL uint32, clientIP net.IP) (*TLSNameServer, error) {
	s, err := baseTLSNameServer(url, "TLSL", disableCache, serveStale, serveExpiredTTL, clientIP)
	if err != nil {
		return nil, err
	}

	s.dial = func(ctx context.Context) (net.Conn, error) {
		conn, err := internet.DialSystem(ctx, *s.destination, nil)
		if err != nil {
			return nil, err
		}
		return s.handshake(ctx, conn)
	}

	errors.LogInfo(context.Background(), "DNS: created Local TLS client initialized for ", url.String())
	return s, nil
}

func baseTLSNameServer(url *url.URL, prefix string, disableCache bool, serveStale bool, serveExpiredTTL uint32, clientIP net.IP) (*TLSNameServer, error) {
	tcpServer, err := baseTCPNameServer(url, prefix, net.Port(853), disableCache, serveStale, serveExpiredTTL, clientIP)
	if err != nil {
		return nil, err
	}

	tlsConfig := tls.Config{}
	// RFC8310 §3.3: the SNI carries the name of the DoT server. IP literals are not
	// valid SNI (RFC6066 §3), crypto/tls drops them and verifies the IP SANs instead.
	tlsConfig.ServerName = url.Hostname()

	return &TLSNameServer{
		TCPNameServer: tcpServer,
		tlsConfig:     tlsConfig.GetTLSConfig(),
	}, nil
}

// handshake does the mandatory TLS handshake before any DNS message (RFC7858 §3.1).
// It returns a connection the TCP framing of the nameserver can write to.
func (s *TLSNameServer) handshake(ctx context.Context, conn net.Conn) (net.Conn, error) {
	tlsConn := tls.Client(conn, s.tlsConfig)
	if err := tlsConn.(tls.Interface).HandshakeContext(ctx); err != nil {
		conn.Close()
		return nil, err
	}
	return tlsConn, nil
}
