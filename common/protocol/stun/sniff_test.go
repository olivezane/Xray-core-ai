package stun

import (
	"encoding/hex"
	"testing"

	"github.com/xtls/xray-core/common"
)

func TestSniffSTUN(t *testing.T) {
	bindingRequest, err := hex.DecodeString("000100002112a44224b1a025d0c180c484341306")
	if err != nil {
		t.Fatal(err)
	}

	wrongMagic := append([]byte(nil), bindingRequest...)
	wrongMagic[4] = 0xde

	notFirstBits := append([]byte(nil), bindingRequest...)
	notFirstBits[0] = 0x80

	messageLength := append([]byte(nil), bindingRequest...)
	messageLength[2] = 0x00
	messageLength[3] = 0x08

	cases := []struct {
		name    string
		payload []byte
		err     error
	}{
		{"binding request", bindingRequest, nil},
		{"shorter than the header", bindingRequest[:19], common.ErrNoClue},
		{"wrong magic cookie", wrongMagic, errNotSTUN},
		{"first two bits not zero", notFirstBits, errNotSTUN},
		{"message length longer than the packet", messageLength, errNotSTUN},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h, err := SniffSTUN(c.payload)
			if err != c.err {
				t.Fatalf("expected error %v, got %v", c.err, err)
			}
			if err == nil && h == nil {
				t.Fatal("expected a sniff header, got nil")
			}
		})
	}
}
