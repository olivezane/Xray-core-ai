package proxyprotocol

import (
	"github.com/pires/go-proxyproto"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/transport/internet/finalmask"
)
// Config is the settings of the PROXY protocol mask.
type Config struct {
	Version uint32 `json:"version,omitempty"`
}
// SettingsName is what a configuration written for the API calls these settings.
func (*Config) SettingsName() string {
	return "xray.transport.internet.finalmask.proxyprotocol.Config"
}

func (c *Config) WrapConnClient(conn net.Conn, dest *net.Destination, dialer *finalmask.Dialer) (net.Conn, error) {
	version := c.Version
	if version == 0 {
		version = 2
	}
	if version != 1 && version != 2 {
		return nil, errors.New("PROXY protocol version must be 1 or 2")
	}
	source := conn.LocalAddr()
	if dialer != nil && dialer.Source != nil {
		source = dialer.Source
	}
	header := proxyproto.HeaderProxyFromAddrs(byte(version), source, conn.RemoteAddr())
	if _, err := header.WriteTo(conn); err != nil {
		return nil, errors.New("failed to write PROXY protocol header").Base(err)
	}
	return conn, nil
}

func (c *Config) WrapConnServer(conn net.Conn) (net.Conn, error) {
	return proxyproto.NewConn(conn, proxyproto.WithPolicy(proxyproto.REQUIRE)), nil
}
