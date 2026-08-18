package rawconn

import (
	"context"
	"io"
	"runtime"
	"time"

	"github.com/pires/go-proxyproto"
	"github.com/xtls/xray-core/app/dispatcher"
	"github.com/xtls/xray-core/common/buf"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/session"
	"github.com/xtls/xray-core/common/signal"
	"github.com/xtls/xray-core/features/stats"
	"github.com/xtls/xray-core/proxy/vless/encryption"
	"github.com/xtls/xray-core/transport/internet"
	"github.com/xtls/xray-core/transport/internet/finalmask"
	"github.com/xtls/xray-core/transport/internet/stat"
)

// peelSpliceTransparent peels exactly the layers that are transparent to a
// splice copy: stat counters (counted, then discarded) and the finalmask
// TcpMask when it allows splicing. It deliberately stops at TLS/Reality/
// CommonConn-style wrappers — their presence means "do not raw-handle me"
// (see the MITM guard in proxy/freedom). Shared by IsRAW and Unwrap.
func peelSpliceTransparent(conn net.Conn, readCounter, writerCounter stats.Counter) (net.Conn, stats.Counter, stats.Counter) {
	for conn != nil {
		// Extract stat counters (peel this layer, continue unwrapping).
		if sc, ok := conn.(*stat.CounterConnection); ok {
			if readCounter == nil {
				readCounter = sc.ReadCounter
			}
			if writerCounter == nil {
				writerCounter = sc.WriteCounter
			}
			conn = sc.Unwrap()
			continue
		}
		// finalmask TcpMaskConn — respects Splice() flag.
		if unwrapped := finalmask.UnwrapTcpMask(conn); unwrapped != conn {
			conn = unwrapped
			continue
		}
		break
	}
	return conn, readCounter, writerCounter
}

func IsRAW(conn stat.Connection) bool {
	if conn == nil {
		return false
	}
	iConn, _, _ := peelSpliceTransparent(conn, nil, nil)
	_, ok1 := iConn.(*proxyproto.Conn)
	_, ok2 := iConn.(*net.TCPConn)
	_, ok3 := iConn.(*internet.UnixConnWrapper)
	return ok1 || ok2 || ok3
}

func Unwrap(conn net.Conn) (net.Conn, stats.Counter, stats.Counter) {
	var readCounter, writerCounter stats.Counter
	for conn != nil {
		// Transparent layers first (counters + TcpMask), then deepen one
		// opaque layer at a time. Re-peeling each round keeps the original
		// priority (counters > Unwrapper > TcpMask > CommonConn > proxyproto)
		// intact for any wrapper nesting.
		conn, readCounter, writerCounter = peelSpliceTransparent(conn, readCounter, writerCounter)
		if conn == nil {
			return nil, readCounter, writerCounter
		}
		if u, ok := conn.(stat.Unwrapper); ok {
			conn = u.Unwrap()
			continue
		}
		// Special cases for external/internal types we can't modify.
		if cc, ok := conn.(*encryption.CommonConn); ok {
			conn = cc.Conn
			continue
		}
		if pc, ok := conn.(*proxyproto.Conn); ok {
			conn = pc.Raw()
			continue
		}
		break
	}
	return conn, readCounter, writerCounter
}

func CopyIfExist(ctx context.Context, readerConn net.Conn, writerConn net.Conn, writer buf.Writer, timer, inTimer *signal.ActivityTimer) error {
	readerConn, readCounter, _ := Unwrap(readerConn)
	writerConn, _, writeCounter := Unwrap(writerConn)
	reader := buf.NewReader(readerConn)
	if runtime.GOOS != "linux" && runtime.GOOS != "android" {
		return readV(ctx, reader, writer, timer, readCounter)
	}
	tc, ok := writerConn.(*net.TCPConn)
	if !ok || readerConn == nil || writerConn == nil {
		return readV(ctx, reader, writer, timer, readCounter)
	}
	inbound := session.InboundFromContext(ctx)
	if inbound == nil || inbound.CanSpliceCopy == 3 {
		return readV(ctx, reader, writer, timer, readCounter)
	}
	outbounds := session.OutboundsFromContext(ctx)
	if len(outbounds) == 0 {
		return readV(ctx, reader, writer, timer, readCounter)
	}
	for _, ob := range outbounds {
		if ob.CanSpliceCopy == 3 {
			return readV(ctx, reader, writer, timer, readCounter)
		}
	}

	for {
		inbound := session.InboundFromContext(ctx)
		outbounds := session.OutboundsFromContext(ctx)
		splice := inbound.CanSpliceCopy == 1
		for _, ob := range outbounds {
			if ob.CanSpliceCopy != 1 {
				splice = false
			}
		}
		if splice {
			errors.LogDebug(ctx, "rawconn: splice")
			statWriter, _ := writer.(*dispatcher.SizeStatWriter)
			timer.SetTimeout(24 * time.Hour)
			if inTimer != nil {
				inTimer.SetTimeout(24 * time.Hour)
			}
			w, err := tc.ReadFrom(readerConn)
			if readCounter != nil {
				readCounter.Add(w)
			}
			if writeCounter != nil {
				writeCounter.Add(w)
			}
			if statWriter != nil {
				statWriter.Counter.Add(w)
			}
			if err != nil && errors.Cause(err) != io.EOF {
				return err
			}
			return nil
		}
		buffer, err := reader.ReadMultiBuffer()
		if !buffer.IsEmpty() {
			if readCounter != nil {
				readCounter.Add(int64(buffer.Len()))
			}
			timer.Update()
			if werr := writer.WriteMultiBuffer(buffer); werr != nil {
				return werr
			}
		}
		if err != nil {
			if errors.Cause(err) == io.EOF {
				return nil
			}
			return err
		}
	}
}

func readV(ctx context.Context, reader buf.Reader, writer buf.Writer, timer signal.ActivityUpdater, readCounter stats.Counter) error {
	errors.LogDebug(ctx, "rawconn: copy (maybe) readv")
	if err := buf.Copy(reader, writer, buf.UpdateActivity(timer), buf.AddToStatCounter(readCounter)); err != nil {
		return errors.New("failed to process response").Base(err)
	}
	return nil
}
