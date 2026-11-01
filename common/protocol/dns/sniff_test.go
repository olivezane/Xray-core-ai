package dns

import (
	"encoding/binary"
	"testing"

	"github.com/xtls/xray-core/common"
	"golang.org/x/net/dns/dnsmessage"
)

func TestSniffDNS(t *testing.T) {
	var opt dnsmessage.ResourceHeader
	common.Must(opt.SetEDNS0(1232, dnsmessage.RCodeSuccess, false))
	msg := dnsmessage.Message{
		Header:      dnsmessage.Header{ID: 0x1234, RecursionDesired: true},
		Questions:   []dnsmessage.Question{{Name: dnsmessage.MustNewName("example.com."), Type: dnsmessage.TypeA, Class: dnsmessage.ClassINET}},
		Additionals: []dnsmessage.Resource{{Header: opt, Body: &dnsmessage.OPTResource{}}},
	}
	query, err := msg.Pack()
	common.Must(err)

	mutate := func(f func([]byte)) []byte {
		b := append([]byte(nil), query...)
		f(b)
		return b
	}
	frame := func(m []byte) []byte {
		return append(binary.BigEndian.AppendUint16(nil, uint16(len(m))), m...)
	}
	response := mutate(func(b []byte) { b[2] |= 0x80 })

	cases := []struct {
		name    string
		sniff   func([]byte) (*SniffHeader, error)
		payload []byte
		err     error
	}{
		{"udp query", SniffUDP, query, nil},
		{"udp query followed by another datagram", SniffUDP, append(append([]byte(nil), query...), query...), nil},
		{"udp response", SniffUDP, response, errNotDNS},
		{"udp notify", SniffUDP, mutate(func(b []byte) { b[2] |= 4 << 3 }), errNotDNS},
		{"udp z bit set", SniffUDP, mutate(func(b []byte) { b[3] |= 0x40 }), errNotDNS},
		{"udp ad and cd bits set", SniffUDP, mutate(func(b []byte) { b[3] |= 0x30 }), nil},
		{"udp no question", SniffUDP, mutate(func(b []byte) { b[5] = 0 }), errNotDNS},
		{"udp two questions", SniffUDP, mutate(func(b []byte) { b[5] = 2 }), errNotDNS},
		{"udp record past the end", SniffUDP, mutate(func(b []byte) { b[11] = 2 }), errNotDNS},
		{"udp shorter than the header", SniffUDP, query[:11], errNotDNS},
		{"tcp query", SniffTCP, frame(query), nil},
		{"tcp pipelined queries", SniffTCP, append(frame(query), frame(query)...), nil},
		{"tcp shorter than the header", SniffTCP, frame(query)[:13], common.ErrNoClue},
		{"tcp message not complete", SniffTCP, frame(query)[:20], common.ErrNoClue},
		{"tcp length shorter than the header", SniffTCP, append([]byte{0, 5}, query...), errNotDNS},
		{"tcp response", SniffTCP, frame(response), errNotDNS},
		{"tcp text protocol", SniffTCP, []byte("SSH-2.0-OpenSSH_9.6\r\n"), errNotDNS},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h, err := c.sniff(c.payload)
			if err != c.err {
				t.Fatalf("expected error %v, got %v", c.err, err)
			}
			if err == nil && h == nil {
				t.Fatal("expected a sniff header, got nil")
			}
		})
	}
}
