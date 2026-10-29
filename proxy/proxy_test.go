package proxy

import (
	"bytes"
	"context"
	"testing"

	"github.com/xtls/xray-core/common/buf"
)

func tlsRecord(n int) []byte {
	b := make([]byte, 5+n)
	b[0], b[1], b[2] = 0x17, 0x03, 0x03
	b[3], b[4] = byte(n>>8), byte(n)
	for i := 5; i < len(b); i++ {
		b[i] = 0xaa
	}
	return b
}

// splitBytes turns raw bytes into a MultiBuffer cut at the given offsets.
func splitBytes(data []byte, cuts ...int) buf.MultiBuffer {
	mb := make(buf.MultiBuffer, 0, len(cuts)+1)
	last := 0
	for _, c := range append(cuts, len(data)) {
		mb = append(mb, buf.FromBytes(data[last:c]))
		last = c
	}
	return mb
}

func TestIsCompleteRecord(t *testing.T) {
	two := append(tlsRecord(10), tlsRecord(7)...)
	cases := []struct {
		name string
		mb   buf.MultiBuffer
		want bool
	}{
		{"empty", nil, true},
		{"one record", splitBytes(tlsRecord(10)), true},
		{"two records", splitBytes(two), true},
		{"header split", splitBytes(two, 2, 5), true},
		{"body split", splitBytes(two, 8, 20), true},
		{"truncated body", splitBytes(two[:len(two)-1]), false},
		{"header only", splitBytes([]byte{0x17, 0x03}), false},
		{"bad version", splitBytes([]byte{0x16, 0x03, 0x03, 0x00, 0x01, 0xaa}), false},
	}
	for _, c := range cases {
		if got := IsCompleteRecord(c.mb); got != c.want {
			t.Errorf("%s: expected %v, got %v", c.name, c.want, got)
		}
		buf.ReleaseMulti(c.mb)
	}
}

func TestLastTLSRecordStart(t *testing.T) {
	cases := [][]byte{
		nil,
		bytes.Repeat([]byte{0x00}, 100),
		{0x17, 0x03, 0x03, 0x00, 0x00},
		{0x17, 0x03, 0x00, 0x17, 0x03, 0x03, 0x00, 0x00},
		{0x17, 0x03, 0x03, 0x17},
		append(bytes.Repeat([]byte{0x17}, 50), 0x03, 0x03),
	}
	for i, c := range cases {
		got, want := lastTLSRecordStart(c), bytes.LastIndex(c, TlsApplicationDataStart)
		if got != want {
			t.Errorf("case %d: expected %d, got %d", i, want, got)
		}
	}
}

func TestReshapeMultiBuffer(t *testing.T) {
	payload := make([]byte, buf.Size)
	for i := range payload {
		payload[i] = 0x11
	}
	const index = buf.Size - 100
	copy(payload[index:], TlsApplicationDataStart)

	b := buf.New()
	b.Write(payload)
	mb := ReshapeMultiBuffer(context.Background(), buf.MultiBuffer{b})

	if len(mb) != 2 {
		t.Fatalf("expected 2 buffers, got %d", len(mb))
	}
	if mb[0].Len() != index {
		t.Errorf("expected first part of %d bytes, got %d", index, mb[0].Len())
	}
	if !bytes.Equal(append(mb[0].Bytes(), mb[1].Bytes()...), payload) {
		t.Error("reshape corrupted the payload")
	}
	buf.ReleaseMulti(mb)
}
