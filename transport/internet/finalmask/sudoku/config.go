package sudoku

import (
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/transport/internet/finalmask"
)
// Config is the settings of the sudoku mask.
type Config struct {
	Password     string   `json:"password,omitempty"`
	Ascii        string   `json:"ascii,omitempty"`
	CustomTable  string   `json:"custom_table,omitempty"`
	PaddingMin   uint32   `json:"padding_min,omitempty"`
	PaddingMax   uint32   `json:"padding_max,omitempty"`
	CustomTables []string `json:"custom_tables,omitempty"`
}
// SettingsName is what a configuration written for the API calls these settings.
func (*Config) SettingsName() string {
	return "xray.transport.internet.finalmask.sudoku.Config"
}

// Sudoku in finalmask mode is a pure appearance transform with no standalone handshake.
// TCP always keeps classic sudoku on uplink and uses packed downlink optimization on server writes.
func (c *Config) WrapConnClient(conn net.Conn, dest *net.Destination, dialer *finalmask.Dialer) (net.Conn, error) {
	return newPackedDirectionalConn(conn, c, true)
}

func (c *Config) WrapConnServer(conn net.Conn) (net.Conn, error) {
	return newPackedDirectionalConn(conn, c, false)
}

func newPackedDirectionalConn(raw net.Conn, config *Config, readPacked bool) (net.Conn, error) {
	pureReader, pureWriter, err := newPureReaderWriter(raw, config)
	if err != nil {
		return nil, err
	}
	packedReader, packedWriter, err := newPackedReaderWriter(raw, config)
	if err != nil {
		return nil, err
	}

	reader, writer := pureReader, pureWriter
	if readPacked {
		reader = packedReader
	} else {
		writer = packedWriter
	}

	return newWrappedConn(raw, reader, writer), nil
}

func (c *Config) WrapPacketConnClient(conn net.PacketConn, dest *net.Destination, dialer *finalmask.Dialer) (net.PacketConn, error) {
	return NewUDPConn(conn, c)
}

func (c *Config) WrapPacketConnServer(conn net.PacketConn, addr net.Addr, lc *finalmask.ListenConfig) (net.PacketConn, error) {
	return NewUDPConn(conn, c)
}
