package xudp

import (
	"testing"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/buf"
	"github.com/xtls/xray-core/common/net"
)

func TestXudpReadWrite(t *testing.T) {
	addr, _ := net.ParseDestination("tcp:127.0.0.1:1345")
	mb := make(buf.MultiBuffer, 0, 16)
	m := buf.MultiBufferContainer{
		MultiBuffer: mb,
	}
	var arr [8]byte
	writer := NewPacketWriter(&m, addr, arr)

	source := make(buf.MultiBuffer, 0, 16)
	b := buf.New()
	b.WriteByte('a')
	b.UDP = &addr
	source = append(source, b)
	writer.WriteMultiBuffer(source)

	reader := NewPacketReader(&m)
	dest, err := reader.ReadMultiBuffer()
	common.Must(err)
	if dest[0].Byte(0) != 'a' {
		t.Error("failed to parse xudp buffer")
	}
	if dest[0].UDP.Port != 1345 {
		t.Error("failed to parse xudp buffer")
	}
}

func TestXudpWriteMultiPacketCoalesce(t *testing.T) {
	addr, _ := net.ParseDestination("tcp:127.0.0.1:1345")
	var m buf.MultiBufferContainer
	var arr [8]byte
	writer := NewPacketWriter(&m, addr, arr)

	payloads := []string{"first", "second", "third"}
	source := make(buf.MultiBuffer, 0, len(payloads))
	for _, p := range payloads {
		b := buf.New()
		b.Write([]byte(p))
		b.UDP = &addr
		source = append(source, b)
	}
	common.Must(writer.WriteMultiBuffer(source))

	// all frames of one batch must land in a single buffer: one TLS record
	if len(m.MultiBuffer) != 1 {
		t.Fatalf("expected 1 buffer, got %d", len(m.MultiBuffer))
	}

	reader := NewPacketReader(&m)
	for _, p := range payloads {
		mb, err := reader.ReadMultiBuffer()
		common.Must(err)
		if got := string(mb[0].Bytes()); got != p {
			t.Errorf("expected %q, got %q", p, got)
		}
		if mb[0].UDP == nil || mb[0].UDP.Port != 1345 {
			t.Error("failed to parse xudp buffer")
		}
		buf.ReleaseMulti(mb)
	}
}

func TestXudpWriteMultiPacketStraddle(t *testing.T) {
	addr, _ := net.ParseDestination("tcp:127.0.0.1:1345")
	var m buf.MultiBufferContainer
	var arr [8]byte
	writer := NewPacketWriter(&m, addr, arr)

	const size = buf.Size - 666
	source := make(buf.MultiBuffer, 0, 2)
	for i := 0; i < 2; i++ {
		b := buf.New()
		b.Write(make([]byte, size))
		b.UDP = &addr
		source = append(source, b)
	}
	common.Must(writer.WriteMultiBuffer(source))

	if len(m.MultiBuffer) != 2 {
		t.Fatalf("expected 2 buffers, got %d", len(m.MultiBuffer))
	}
	reader := NewPacketReader(&m)
	for i := 0; i < 2; i++ {
		mb, err := reader.ReadMultiBuffer()
		common.Must(err)
		if mb[0].Len() != size {
			t.Errorf("expected %d bytes, got %d", size, mb[0].Len())
		}
		buf.ReleaseMulti(mb)
	}
}
