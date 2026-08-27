package xmc

import (
	"fmt"
	"net"
)

// XMC connections handshake (Minecraft login) lazily before proxy data flows,
// under finalmask.HandshakeDeadlines arbitration.

func (c *Config) TCP() {
}

func (c *Config) WrapConnClient(conn net.Conn) (net.Conn, error) {
	profiles, err := profilesFromConfig(c.Profiles)
	if err != nil {
		return nil, fmt.Errorf("minecraft finalmask: %w", err)
	}
	cc, err := newClientConn(conn, profiles, c.Password, c.RsaPublicKey, c.Hostname)
	if err != nil {
		return nil, fmt.Errorf("minecraft finalmask: %w", err)
	}

	return cc, nil
}

func (c *Config) WrapConnServer(conn net.Conn) (net.Conn, error) {
	profiles, err := profilesFromConfig(c.Profiles)
	if err != nil {
		return nil, fmt.Errorf("minecraft finalmask: %w", err)
	}
	cc, err := wrapConnServer(conn, profiles, c.Password, c.RsaPrivateKey, c.RsaPublicKey)
	if err != nil {
		return nil, fmt.Errorf("minecraft finalmask: %w", err)
	}

	return cc, nil
}
