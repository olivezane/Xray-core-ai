package tun

import (
	"net"

	"github.com/xtls/xray-core/common/buf"
	"github.com/xtls/xray-core/features/stats"
	"github.com/xtls/xray-core/transport/internet/stat"
)

// lazyBufferReader is the receive side of the stack buffer APIs that hand the
// destination buffer out only once payload is available.
type lazyBufferReader interface {
	// ReadWithBuffer reads the next payload into the slice getBuffer returns.
	ReadWithBuffer(getBuffer func(sizeHint int) []byte) (int, error)
}

// newUplinkReader reads the uplink of one connection. A stack connection that
// hands its buffer out lazily keeps the buffer pool untouched while the
// connection sits idle, and counts its bytes here instead of in the connection
// wrapper its Read method never reaches.
func newUplinkReader(conn net.Conn, counter stats.Counter) buf.Reader {
	lazy, ok := stat.TryUnwrapStatsConn(conn).(lazyBufferReader)
	if !ok {
		return buf.NewReader(conn)
	}
	return &lazyReader{readWithBuffer: lazy.ReadWithBuffer, counter: counter}
}

// lazyReader reads one pooled buffer per payload, taking it from the pool when
// the stack offers the payload instead of before blocking on it. The size hint
// of the stack is ignored: the buffer is the same pooled one the eager reader
// took before blocking, so payloads are still read in chunks of that size.
type lazyReader struct {
	readWithBuffer func(getBuffer func(sizeHint int) []byte) (int, error)
	counter        stats.Counter
}

// ReadMultiBuffer implements buf.Reader.
func (r *lazyReader) ReadMultiBuffer() (buf.MultiBuffer, error) {
	var buffer *buf.Buffer
	n, err := r.readWithBuffer(func(int) []byte {
		buffer = buf.New()
		return buffer.Extend(buf.Size)
	})
	if buffer == nil {
		// the payload never arrived, so the pool was never touched
		return nil, err
	}
	if r.counter != nil {
		r.counter.Add(int64(n))
	}
	if n == 0 {
		// the stack reports an empty buffer as a short buffer without
		// consuming payload, and the caller must still release it
		buffer.Release()
		return nil, err
	}
	buffer.Resize(0, int32(n))
	return buf.MultiBuffer{buffer}, err
}
