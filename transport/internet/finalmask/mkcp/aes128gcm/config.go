package aes128gcm

import (
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/transport/internet/finalmask"
)
// Config is the settings of the aes128gcm mkcp mask.
type Config struct {
	Password string `json:"password,omitempty"`
}
// SettingsName is what a configuration written for the API calls these settings.
func (*Config) SettingsName() string {
	return "xray.transport.internet.finalmask.mkcp.aes128gcm.Config"
}

func (c *Config) HeaderConn() {}

func (c *Config) WrapPacketConnClient(conn net.PacketConn, dest *net.Destination, dialer *finalmask.Dialer) (net.PacketConn, error) {
	return NewConnClient(c, conn)
}

func (c *Config) WrapPacketConnServer(conn net.PacketConn, addr net.Addr, lc *finalmask.ListenConfig) (net.PacketConn, error) {
	return NewConnServer(c, conn)
}
