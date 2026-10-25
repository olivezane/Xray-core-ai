package stun

import (
	"encoding/binary"
	"errors"

	"github.com/xtls/xray-core/common"
)

type SniffHeader struct{}

func (h *SniffHeader) Protocol() string {
	return "stun"
}

func (h *SniffHeader) Domain() string {
	return ""
}

var errNotSTUN = errors.New("not stun header")

func SniffSTUN(b []byte) (*SniffHeader, error) {
	if len(b) < 20 {
		return nil, common.ErrNoClue
	}

	// The two leading bits of every STUN message are zero.
	// https://datatracker.ietf.org/doc/html/rfc5389#section-6
	if b[0]&0xc0 != 0 {
		return nil, errNotSTUN
	}

	if binary.BigEndian.Uint32(b[4:8]) != 0x2112a442 {
		return nil, errNotSTUN
	}

	// A body that is truncated or not a multiple of 4 bytes is not STUN.
	if bodyLen := int(binary.BigEndian.Uint16(b[2:4])); bodyLen%4 != 0 || len(b) < 20+bodyLen {
		return nil, errNotSTUN
	}

	return &SniffHeader{}, nil
}
