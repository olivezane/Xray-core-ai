package dns_test

import (
	"context"
	"errors"
	"testing"

	. "github.com/xtls/xray-core/app/dns"
	"github.com/xtls/xray-core/app/dns/fakedns"
	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/features/dns"
)

func TestFakeDNSServerWithoutIPv6Pool(t *testing.T) {
	engine, err := fakedns.NewFakeDNSHolderConfigOnly(&fakedns.FakeDnsPool{IpPool: "198.18.0.0/24", LruSize: 16})
	common.Must(err)
	common.Must(engine.Start())
	server := NewFakeDNSServer(engine)

	ips, _, err := server.QueryIP(context.Background(), "example.com", dns.IPOption{IPv4Enable: true})
	common.Must(err)
	if len(ips) != 1 || ips[0].To4() == nil {
		t.Fatalf("want one IPv4 answer, got %v", ips)
	}

	// The pool has no IPv6 addresses: answer NODATA instead of falling back,
	// which would leak a real AAAA answer and bypass fake IP routing.
	_, _, err = server.QueryIP(context.Background(), "example.com", dns.IPOption{IPv6Enable: true})
	if !errors.Is(err, dns.ErrNoRecords) {
		t.Fatalf("want ErrNoRecords, got %v", err)
	}
}
