//go:build !linux && !windows && !android && !darwin && !freebsd

package tun

import (
	"net"

	"github.com/xtls/xray-core/common/errors"
)

// NewTun builds new tun interface handler
func NewTun(options *Config) (Tun, error) {
	return nil, errors.New("Tun is not supported on your platform")
}

func setinterface(string, string, uintptr, *net.Interface) error {
	return errors.New("Tun is not supported on your platform")
}
