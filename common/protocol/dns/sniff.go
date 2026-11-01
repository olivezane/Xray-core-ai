package dns

import (
	"encoding/binary"
	"errors"

	"github.com/xtls/xray-core/common"
	"golang.org/x/net/dns/dnsmessage"
)

type SniffHeader struct{}

func (h *SniffHeader) Protocol() string {
	return "dns"
}

func (h *SniffHeader) Domain() string {
	return ""
}

var errNotDNS = errors.New("not dns query")

// isQuery reports whether the 12-octet header h belongs to a standard query.
func isQuery(h []byte) bool {
	// RFC 1035 §4.1.1: QR is 0 in a query, OPCODE 0 is a standard query and Z must be zero.
	// Z is down to the 0x40 bit, AD and CD took the others (RFC 4035 §3).
	// RFC 9619 allows QDCOUNT 0 or 1; only the question sets DNS apart from other traffic, so require 1.
	return h[2]&0xf8 == 0 && h[3]&0x40 == 0 && binary.BigEndian.Uint16(h[4:6]) == 1
}

// SniffUDP matches a standard query sent as a bare DNS message, as over UDP (RFC 1035 §4.2.1).
// Bytes after the message are ignored, as the sniffing buffer may also hold the next datagram.
func SniffUDP(b []byte) (*SniffHeader, error) {
	var p dnsmessage.Parser
	if _, err := p.Start(b); err != nil || !isQuery(b) {
		return nil, errNotDNS
	}
	// The question needs a valid name (RFC 1035 §3.1, §4.1.4) and every record must fit in the message (§4.1.3).
	if _, err := p.Question(); err != nil {
		return nil, errNotDNS
	}
	if p.SkipAllQuestions() != nil || p.SkipAllAnswers() != nil || p.SkipAllAuthorities() != nil || p.SkipAllAdditionals() != nil {
		return nil, errNotDNS
	}
	return &SniffHeader{}, nil
}

// SniffTCP matches a standard query over TCP, where a two-octet length precedes each message (RFC 1035 §4.2.2, RFC 7766 §8).
func SniffTCP(b []byte) (*SniffHeader, error) {
	if len(b) < 2+12 {
		return nil, common.ErrNoClue
	}
	if !isQuery(b[2:]) {
		return nil, errNotDNS
	}
	end := 2 + int(binary.BigEndian.Uint16(b))
	if len(b) < end {
		// Not ErrProtoNeedMoreData: that drops the other pending sniffers, e.g. TLS waiting for the rest of a record.
		return nil, common.ErrNoClue
	}
	return SniffUDP(b[2:end])
}
