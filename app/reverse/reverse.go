package reverse

import (
	"github.com/xtls/xray-core/common/net"
)

const (
	internalDomain = "reverse"
)

func isDomain(dest net.Destination, domain string) bool {
	return dest.Address.Family().IsDomain() && dest.Address.Domain() == domain
}

func isInternalDomain(dest net.Destination) bool {
	return isDomain(dest, internalDomain)
}
