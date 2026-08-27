package dispatcher

import (
	"context"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/protocol/bittorrent"
	"github.com/xtls/xray-core/common/protocol/http"
	"github.com/xtls/xray-core/common/protocol/quic"
	"github.com/xtls/xray-core/common/protocol/tls"
	"github.com/xtls/xray-core/features/dns"
)

// SnifferResult is the result of a protocol sniffer. Besides the sniffed
// protocol and domain, it carries three capabilities used by the dispatch
// path:
//   - ProtocolForDomainResult returns the protocol of the domain part, which
//     differs from Protocol() only for composite results.
//   - IsProtoSubsetOf reports whether the sniffed protocol is a subset of the
//     given protocol (e.g. "fakedns+others" of "http1").
//   - IsFakeDNS reports whether the domain comes from the fake DNS engine.
type SnifferResult interface {
	Protocol() string
	Domain() string
	ProtocolForDomainResult() string
	IsProtoSubsetOf(protocolName string) bool
	IsFakeDNS() bool
}

type protocolSniffer func(context.Context, []byte) (SnifferResult, error)

type protocolSnifferWithMetadata struct {
	protocolSniffer protocolSniffer
	// A Metadata sniffer will be invoked on connection establishment only, with nil body,
	// for both TCP and UDP connections
	// It will not be shown as a traffic type for routing unless there is no other successful sniffing.
	metadataSniffer bool
	network         net.Network
}

type Sniffer struct {
	sniffer []protocolSnifferWithMetadata
}

func NewSniffer(ctx context.Context) *Sniffer {
	ret := &Sniffer{
		sniffer: []protocolSnifferWithMetadata{
			{func(c context.Context, b []byte) (SnifferResult, error) { return http.SniffHTTP(b, c) }, false, net.Network_TCP},
			{func(c context.Context, b []byte) (SnifferResult, error) { return tls.SniffTLS(b) }, false, net.Network_TCP},
			{func(c context.Context, b []byte) (SnifferResult, error) { return bittorrent.SniffBittorrent(b) }, false, net.Network_TCP},
			{func(c context.Context, b []byte) (SnifferResult, error) { return quic.SniffQUIC(b) }, false, net.Network_UDP},
			{func(c context.Context, b []byte) (SnifferResult, error) { return bittorrent.SniffUTP(b) }, false, net.Network_UDP},
		},
	}
	if sniffer, err := newFakeDNSSniffer(ctx); err == nil {
		others := ret.sniffer
		ret.sniffer = append(ret.sniffer, sniffer)
		fakeDNSThenOthers, err := newFakeDNSThenOthers(ctx, sniffer, others)
		if err == nil {
			ret.sniffer = append([]protocolSnifferWithMetadata{fakeDNSThenOthers}, ret.sniffer...)
		}
	}
	return ret
}

var errUnknownContent = errors.New("unknown content")

func (s *Sniffer) Sniff(c context.Context, payload []byte, network net.Network) (SnifferResult, error) {
	var pendingSniffer []protocolSnifferWithMetadata
	for _, si := range s.sniffer {
		protocolSniffer := si.protocolSniffer
		if si.metadataSniffer || si.network != network {
			continue
		}
		result, err := protocolSniffer(c, payload)
		if err == common.ErrNoClue {
			pendingSniffer = append(pendingSniffer, si)
			continue
		} else if err == protocol.ErrProtoNeedMoreData { // Sniffer protocol matched, but need more data to complete sniffing
			s.sniffer = []protocolSnifferWithMetadata{si}
			return nil, err
		}

		if err == nil && result != nil {
			return result, nil
		}
	}

	if len(pendingSniffer) > 0 {
		s.sniffer = pendingSniffer
		return nil, common.ErrNoClue
	}

	return nil, errUnknownContent
}

func (s *Sniffer) SniffMetadata(c context.Context) (SnifferResult, error) {
	var pendingSniffer []protocolSnifferWithMetadata
	for _, si := range s.sniffer {
		s := si.protocolSniffer
		if !si.metadataSniffer {
			pendingSniffer = append(pendingSniffer, si)
			continue
		}
		result, err := s(c, nil)
		if err == common.ErrNoClue {
			pendingSniffer = append(pendingSniffer, si)
			continue
		}

		if err == nil && result != nil {
			return result, nil
		}
	}

	if len(pendingSniffer) > 0 {
		s.sniffer = pendingSniffer
		return nil, common.ErrNoClue
	}

	return nil, errUnknownContent
}

func CompositeResult(domainResult SnifferResult, protocolResult SnifferResult) SnifferResult {
	return &compositeResult{domainResult: domainResult, protocolResult: protocolResult}
}

type compositeResult struct {
	domainResult   SnifferResult
	protocolResult SnifferResult
}

func (c compositeResult) Protocol() string {
	return c.protocolResult.Protocol()
}

func (c compositeResult) Domain() string {
	return c.domainResult.Domain()
}

func (c compositeResult) ProtocolForDomainResult() string {
	return c.domainResult.Protocol()
}

// IsProtoSubsetOf reports false: composite results have never supported
// subset matching, and the subset-capable "fakedns+others" result is never
// wrapped into a composite. Keep routing identical to the pre-interface
// behavior, where the subset check was skipped for composites.
func (c compositeResult) IsProtoSubsetOf(protocolName string) bool {
	return false
}

func (c compositeResult) IsFakeDNS() bool {
	return c.domainResult.IsFakeDNS()
}

// isIPInFakeDNSPool reports whether addr belongs to the fake DNS IP pool.
// The second return value is false when the engine lacks the Rev0 capability.
func isIPInFakeDNSPool(fakeDNSEngine dns.FakeDNSEngine, addr net.Address) (inPool, ok bool) {
	fkr0, ok := fakeDNSEngine.(dns.FakeDNSEngineRev0)
	if !ok {
		return false, false
	}
	return fkr0.IsIPInIPPool(addr), true
}
