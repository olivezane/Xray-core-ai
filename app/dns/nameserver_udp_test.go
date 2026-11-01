package dns

import (
	"context"
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/buf"
	udp_proto "github.com/xtls/xray-core/common/protocol/udp"
	"github.com/xtls/xray-core/common/task"
	dns_feature "github.com/xtls/xray-core/features/dns"
	"golang.org/x/net/dns/dnsmessage"
)

func newTestUDPServer() *ClassicNameServer {
	s := &ClassicNameServer{
		requests:        make(map[uint16]*udpDnsRequest),
		cacheController: NewCacheController("test", false, false, 0),
	}
	s.requestsCleanup = &task.Periodic{
		Interval: time.Hour,
		Execute:  s.RequestsCleanup,
	}
	return s
}

func TestAddPendingRequestAvoidsCollision(t *testing.T) {
	s := newTestUDPServer()

	reqs, err := buildReqMsgs("example.com.", dns_feature.IPOption{IPv4Enable: true}, func() uint16 { return 42 }, nil)
	common.Must(err)
	s.addPendingRequest(&udpDnsRequest{dnsRequest: *reqs[0]})

	// A second query drawing the same ID must not overwrite the pending one:
	// the in-flight request would be lost and only time out.
	collided := *reqs[0]
	s.addPendingRequest(&udpDnsRequest{dnsRequest: collided})

	if len(s.requests) != 2 {
		t.Fatalf("expected 2 pending requests, got %d", len(s.requests))
	}
	if collided.msg.ID == 42 {
		t.Error("colliding request kept the busy ID")
	}
	if _, ok := s.requests[42]; !ok {
		t.Error("pending request with ID 42 was dropped")
	}
	if _, ok := s.requests[collided.msg.ID]; !ok {
		t.Error("regenerated request is not pending")
	}
}

func TestHandleResponseKeepsPendingOnMismatch(t *testing.T) {
	s := newTestUDPServer()
	ctx := context.Background()

	reqs, err := buildReqMsgs("example.com.", dns_feature.IPOption{IPv4Enable: true}, func() uint16 { return 42 }, nil)
	common.Must(err)
	s.addPendingRequest(&udpDnsRequest{dnsRequest: *reqs[0], ctx: ctx})

	answer := func(name string) *buf.Buffer {
		msg := new(dns.Msg)
		msg.SetQuestion(name, dns.TypeA)
		msg.Id = 42
		msg.Response = true
		msg.Answer = append(msg.Answer, common.Must2(dns.NewRR(name+" 60 IN A 192.0.2.53")))
		return buf.FromBytes(common.Must2(msg.Pack()))
	}

	// The ID matches but the question does not (RFC5452 §9.1): the pending
	// request must survive so the real answer still counts.
	s.HandleResponse(ctx, &udp_proto.Packet{Payload: answer("example.net.")})
	if _, ok := s.requests[42]; !ok {
		t.Fatal("mismatching response dropped the pending request")
	}

	s.HandleResponse(ctx, &udp_proto.Packet{Payload: answer("example.com.")})
	if _, ok := s.requests[42]; ok {
		t.Error("answered request is still pending")
	}
}

func TestHandleResponseFailsStillTruncated(t *testing.T) {
	s := newTestUDPServer()
	ctx := context.Background()

	reqs, err := buildReqMsgs("example.com.", dns_feature.IPOption{IPv4Enable: true}, func() uint16 { return 42 }, nil)
	common.Must(err)
	reqs[0].msg.Additionals = append(reqs[0].msg.Additionals, dnsmessage.Resource{}) // already retried with EDNS0
	errCh := make(chan error, 1)
	s.addPendingRequest(&udpDnsRequest{dnsRequest: *reqs[0], ctx: ctx, errCh: errCh})

	msg := new(dns.Msg)
	msg.SetQuestion("example.com.", dns.TypeA)
	msg.Id = 42
	msg.Response = true
	msg.Truncated = true
	s.HandleResponse(ctx, &udp_proto.Packet{Payload: buf.FromBytes(common.Must2(msg.Pack()))})

	// The waiter must fail at once instead of sitting out the 8s request timeout.
	select {
	case <-errCh:
	default:
		t.Fatal("still truncated response did not fail the query")
	}
	if _, ok := s.requests[42]; ok {
		t.Error("failed request is still pending")
	}
}
