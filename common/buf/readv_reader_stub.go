//go:build openbsd

package buf

import (
	"io"
	"syscall"

	"github.com/xtls/xray-core/features/stats"
)

func useReadV() bool {
	return false
}

// ReloadEnvSettings is the OpenBSD version of the readv setting reload. readv is
// not available here, so there is nothing to read from the environment; the
// function exists so that the place that composes a build can register it on
// every platform.
func ReloadEnvSettings() error {
	return nil
}

func NewReadVReader(reader io.Reader, rawConn syscall.RawConn, counter stats.Counter) Reader {
	panic("not implemented")
}
