package header

import (
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/transport/internet/finalmask"
)
// Config is the settings of the header mkcp mask.
type Config struct {
	ID     int32  `json:"ID,omitempty"`
	Domain string `json:"domain,omitempty"`
}
// SettingsName is what a configuration written for the API calls these settings.
func (*Config) SettingsName() string {
	return "xray.transport.internet.finalmask.mkcp.header.Config"
}

func (c *Config) HeaderConn() {}

func (c *Config) WrapPacketConnClient(conn net.PacketConn, dest *net.Destination, dialer *finalmask.Dialer) (net.PacketConn, error) {
	return NewConnClient(c, conn)
}

func (c *Config) WrapPacketConnServer(conn net.PacketConn, addr net.Addr, lc *finalmask.ListenConfig) (net.PacketConn, error) {
	return NewConnServer(c, conn)
}
