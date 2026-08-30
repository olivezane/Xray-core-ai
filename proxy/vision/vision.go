package vision

import (
	"bytes"
	"context"
	"crypto/rand"
	"math/big"
	"strconv"

	"github.com/xtls/xray-core/common/buf"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/session"
	"github.com/xtls/xray-core/common/signal"
	"github.com/xtls/xray-core/features/stats"
	"github.com/xtls/xray-core/transport/internet/rawconn"
)

type Direction int

const (
	DirectionUpstream Direction = iota
	DirectionDownstream
)

var (
	Tls13SupportedVersions  = []byte{0x00, 0x2b, 0x00, 0x02, 0x03, 0x04}
	TlsClientHandShakeStart = []byte{0x16, 0x03}
	TlsServerHandShakeStart = []byte{0x16, 0x03, 0x03}
	TlsApplicationDataStart = []byte{0x17, 0x03, 0x03}

	Tls13CipherSuiteDic = map[uint16]string{
		0x1301: "TLS_AES_128_GCM_SHA256",
		0x1302: "TLS_AES_256_GCM_SHA384",
		0x1303: "TLS_CHACHA20_POLY1305_SHA256",
		0x1304: "TLS_AES_128_CCM_SHA256",
		0x1305: "TLS_AES_128_CCM_8_SHA256",
	}

	TlsHandshakeTypeClientHello byte = 0x01
	TlsHandshakeTypeServerHello byte = 0x02

	CommandPaddingContinue byte = 0x00
	CommandPaddingEnd      byte = 0x01
	CommandPaddingDirect   byte = 0x02
)

type InboundState struct {
	WithinPaddingBuffers     bool
	UplinkReaderDirectCopy   bool
	RemainingCommand         int32
	RemainingContent         int32
	RemainingPadding         int32
	CurrentCommand           int
	IsPadding                bool
	DownlinkWriterDirectCopy bool
}

type OutboundState struct {
	WithinPaddingBuffers     bool
	DownlinkReaderDirectCopy bool
	RemainingCommand         int32
	RemainingContent         int32
	RemainingPadding         int32
	CurrentCommand           int
	IsPadding                bool
	UplinkWriterDirectCopy   bool
}

type TrafficState struct {
	UserUUID               []byte
	NumberOfPacketToFilter int
	EnableXtls             bool
	IsTLS12orAbove         bool
	IsTLS                  bool
	Cipher                 uint16
	RemainingServerHello   int32
	Inbound                InboundState
	Outbound               OutboundState
}

func NewTrafficState(userUUID []byte) *TrafficState {
	return &TrafficState{
		UserUUID:               userUUID,
		NumberOfPacketToFilter: 8,
		EnableXtls:             false,
		IsTLS12orAbove:         false,
		IsTLS:                  false,
		Cipher:                 0,
		RemainingServerHello:   -1,
		Inbound: InboundState{
			WithinPaddingBuffers: true,
			RemainingCommand:     -1,
			RemainingContent:     -1,
			RemainingPadding:     -1,
			CurrentCommand:       0,
			IsPadding:            true,
		},
		Outbound: OutboundState{
			WithinPaddingBuffers: true,
			RemainingCommand:     -1,
			RemainingContent:     -1,
			RemainingPadding:     -1,
			CurrentCommand:       0,
			IsPadding:            true,
		},
	}
}

func isUpstream(dir Direction) bool { return dir == DirectionUpstream }

type visionReader struct {
	buf.Reader
	trafficState      *TrafficState
	ctx               context.Context
	isUplink          bool
	conn              net.Conn
	input             *bytes.Reader
	rawInput          *bytes.Buffer
	ob                *session.Outbound
	directReadCounter stats.Counter
}

// WrapReader wraps r with the XTLS-Vision reader for the given direction.
// input/rawInput mirror the upstream contract: the underlying TLS/REALITY
// conn buffers decrypted/raw data in those structures; the reader replays
// them into the stream when switching to direct copy. Pass nil to keep the
// legacy fork behavior (no replay — used only when no buffered data exists).
func WrapReader(r buf.Reader, conn net.Conn, state *TrafficState, dir Direction, ctx context.Context, input *bytes.Reader, rawInput *bytes.Buffer) buf.Reader {
	if input == nil {
		input = &bytes.Reader{}
	}
	if rawInput == nil {
		rawInput = &bytes.Buffer{}
	}
	return &visionReader{
		Reader:       r,
		trafficState: state,
		ctx:          ctx,
		isUplink:     isUpstream(dir),
		conn:         conn,
		input:        input,
		rawInput:     rawInput,
		ob:           getLastOutbound(ctx),
	}
}

func (w *visionReader) ReadMultiBuffer() (buf.MultiBuffer, error) {
	buffer, err := w.Reader.ReadMultiBuffer()
	if buffer.IsEmpty() {
		return buffer, err
	}

	var WithinPaddingBuffers *bool
	var RemainingContent *int32
	var RemainingPadding *int32
	var CurrentCommand *int
	var switchToDirectCopy *bool
	if w.isUplink {
		WithinPaddingBuffers = &w.trafficState.Inbound.WithinPaddingBuffers
		RemainingContent = &w.trafficState.Inbound.RemainingContent
		RemainingPadding = &w.trafficState.Inbound.RemainingPadding
		CurrentCommand = &w.trafficState.Inbound.CurrentCommand
		switchToDirectCopy = &w.trafficState.Inbound.UplinkReaderDirectCopy
	} else {
		WithinPaddingBuffers = &w.trafficState.Outbound.WithinPaddingBuffers
		RemainingContent = &w.trafficState.Outbound.RemainingContent
		RemainingPadding = &w.trafficState.Outbound.RemainingPadding
		CurrentCommand = &w.trafficState.Outbound.CurrentCommand
		switchToDirectCopy = &w.trafficState.Outbound.DownlinkReaderDirectCopy
	}

	if *switchToDirectCopy {
		if w.directReadCounter != nil {
			w.directReadCounter.Add(int64(buffer.Len()))
		}
		return buffer, err
	}

	if *WithinPaddingBuffers || w.trafficState.NumberOfPacketToFilter > 0 {
		mb2 := make(buf.MultiBuffer, 0, len(buffer))
		for _, b := range buffer {
			newbuffer := xtlsUnpadding(b, w.trafficState, w.isUplink, w.ctx)
			if newbuffer.Len() > 0 {
				mb2 = append(mb2, newbuffer)
			}
		}
		buffer = mb2
		if *RemainingContent > 0 || *RemainingPadding > 0 || *CurrentCommand == 0 {
			*WithinPaddingBuffers = true
		} else if *CurrentCommand == 1 {
			*WithinPaddingBuffers = false
		} else if *CurrentCommand == 2 {
			*WithinPaddingBuffers = false
			*switchToDirectCopy = true
		} else {
			errors.LogDebug(w.ctx, "vision: unknown command ", *CurrentCommand, buffer.Len())
		}
	}
	if w.trafficState.NumberOfPacketToFilter > 0 {
		xtlsFilterTls(buffer, w.trafficState, w.ctx)
	}

	if *switchToDirectCopy {
		if inputBuffer, err := buf.ReadFrom(w.input); err == nil && !inputBuffer.IsEmpty() {
			buffer, _ = buf.MergeMulti(buffer, inputBuffer)
		}
		if rawInputBuffer, err := buf.ReadFrom(w.rawInput); err == nil && !rawInputBuffer.IsEmpty() {
			buffer, _ = buf.MergeMulti(buffer, rawInputBuffer)
		}
		*w.input = bytes.Reader{}
		w.input = nil
		*w.rawInput = bytes.Buffer{}
		w.rawInput = nil

		if inbound := session.InboundFromContext(w.ctx); inbound != nil && inbound.Conn != nil {
			if !w.isUplink && w.ob != nil && w.ob.CanSpliceCopy == 2 {
				w.ob.CanSpliceCopy = 1
			}
		}
		readerConn, readCounter, _ := rawconn.Unwrap(w.conn)
		w.directReadCounter = readCounter
		w.Reader = buf.NewReader(readerConn)
	}
	return buffer, err
}

type visionWriter struct {
	buf.Writer
	trafficState       *TrafficState
	ctx                context.Context
	isUplink           bool
	conn               net.Conn
	ob                 *session.Outbound
	writeOnceUserUUID  []byte
	directWriteCounter stats.Counter
	testseed           []uint32
}

func WrapWriter(w buf.Writer, conn net.Conn, state *TrafficState, dir Direction, ctx context.Context, testseed []uint32) buf.Writer {
	uuid := make([]byte, len(state.UserUUID))
	copy(uuid, state.UserUUID)
	if len(testseed) < 4 {
		testseed = []uint32{900, 500, 900, 256}
	}
	return &visionWriter{
		Writer:            w,
		trafficState:      state,
		ctx:               ctx,
		writeOnceUserUUID: uuid,
		isUplink:          isUpstream(dir),
		conn:              conn,
		ob:                getLastOutbound(ctx),
		testseed:          testseed,
	}
}

func (w *visionWriter) WriteMultiBuffer(mb buf.MultiBuffer) error {
	var IsPadding *bool
	var switchToDirectCopy *bool
	var spliceReadyInbound *session.Inbound
	if w.isUplink {
		IsPadding = &w.trafficState.Outbound.IsPadding
		switchToDirectCopy = &w.trafficState.Outbound.UplinkWriterDirectCopy
	} else {
		IsPadding = &w.trafficState.Inbound.IsPadding
		switchToDirectCopy = &w.trafficState.Inbound.DownlinkWriterDirectCopy
	}

	if *switchToDirectCopy {
		if inbound := session.InboundFromContext(w.ctx); inbound != nil {
			if !w.isUplink && inbound.CanSpliceCopy == 2 {
				spliceReadyInbound = inbound
			}
		}
		rawConn, _, writerCounter := rawconn.Unwrap(w.conn)
		w.Writer = buf.NewWriter(rawConn)
		w.directWriteCounter = writerCounter
		*switchToDirectCopy = false
	}
	if !mb.IsEmpty() && w.directWriteCounter != nil {
		w.directWriteCounter.Add(int64(mb.Len()))
	}

	if w.trafficState.NumberOfPacketToFilter > 0 {
		xtlsFilterTls(mb, w.trafficState, w.ctx)
	}

	if *IsPadding {
		if len(mb) == 1 && mb[0] == nil {
			mb[0] = xtlsPadding(nil, CommandPaddingContinue, &w.writeOnceUserUUID, true, w.ctx, w.testseed)
		} else {
			isComplete := isCompleteRecord(mb)
			mb = reshapeMultiBuffer(w.ctx, mb)
			longPadding := w.trafficState.IsTLS
			for i, b := range mb {
				if w.trafficState.IsTLS && b.Len() >= 6 && bytes.Equal(TlsApplicationDataStart, b.BytesTo(3)) && isComplete {
					if w.trafficState.EnableXtls {
						*switchToDirectCopy = true
					}
					var command byte = CommandPaddingContinue
					if i == len(mb)-1 {
						command = CommandPaddingEnd
						if w.trafficState.EnableXtls {
							command = CommandPaddingDirect
						}
					}
					mb[i] = xtlsPadding(b, command, &w.writeOnceUserUUID, true, w.ctx, w.testseed)
					*IsPadding = false
					longPadding = false
					continue
				} else if !w.trafficState.IsTLS12orAbove && w.trafficState.NumberOfPacketToFilter <= 1 {
					*IsPadding = false
					mb[i] = xtlsPadding(b, CommandPaddingEnd, &w.writeOnceUserUUID, longPadding, w.ctx, w.testseed)
					break
				}
				var command byte = CommandPaddingContinue
				if i == len(mb)-1 && !*IsPadding {
					command = CommandPaddingEnd
					if w.trafficState.EnableXtls {
						command = CommandPaddingDirect
					}
				}
				mb[i] = xtlsPadding(b, command, &w.writeOnceUserUUID, longPadding, w.ctx, w.testseed)
			}
		}
	}
	if err := w.Writer.WriteMultiBuffer(mb); err != nil {
		return err
	}
	if spliceReadyInbound != nil && spliceReadyInbound.CanSpliceCopy == 2 {
		spliceReadyInbound.CanSpliceCopy = 1
	}
	return nil
}

func RunReader(r buf.Reader, w buf.Writer, conn net.Conn, state *TrafficState, dir Direction, ctx context.Context, timer *signal.ActivityTimer) error {
	isUplink := isUpstream(dir)
	return runReader(r, w, conn, state, isUplink, ctx, timer)
}

func runReader(reader buf.Reader, writer buf.Writer, conn net.Conn, trafficState *TrafficState, isUplink bool, ctx context.Context, timer *signal.ActivityTimer) error {
	for {
		if isUplink && trafficState.Inbound.UplinkReaderDirectCopy || !isUplink && trafficState.Outbound.DownlinkReaderDirectCopy {
			var writerConn net.Conn
			var inTimer *signal.ActivityTimer
			if inbound := session.InboundFromContext(ctx); inbound != nil && inbound.Conn != nil {
				writerConn = inbound.Conn
				inTimer = inbound.Timer
			}
			return rawconn.CopyIfExist(ctx, conn, writerConn, writer, timer, inTimer)
		}
		buffer, err := reader.ReadMultiBuffer()
		if !buffer.IsEmpty() {
			timer.Update()
			if werr := writer.WriteMultiBuffer(buffer); werr != nil {
				return werr
			}
		}
		if err != nil {
			return err
		}
	}
}

func getLastOutbound(ctx context.Context) *session.Outbound {
	obs := session.OutboundsFromContext(ctx)
	if len(obs) == 0 {
		return nil
	}
	return obs[len(obs)-1]
}

func isCompleteRecord(buffer buf.MultiBuffer) bool {
	b := make([]byte, buffer.Len())
	if buffer.Copy(b) != int(buffer.Len()) {
		panic("impossible bytes allocation")
	}
	var headerLen int = 5
	var recordLen int

	totalLen := len(b)
	i := 0
	for i < totalLen {
		if headerLen > 0 {
			data := b[i]
			i++
			switch headerLen {
			case 5:
				if data != 0x17 {
					return false
				}
			case 4:
				if data != 0x03 {
					return false
				}
			case 3:
				if data != 0x03 {
					return false
				}
			case 2:
				recordLen = int(data) << 8
			case 1:
				recordLen = recordLen | int(data)
			}
			headerLen--
		} else if recordLen > 0 {
			remaining := totalLen - i
			if remaining < recordLen {
				return false
			} else {
				i += recordLen
				recordLen = 0
				headerLen = 5
			}
		} else {
			return false
		}
	}
	if headerLen == 5 && recordLen == 0 {
		return true
	}
	return false
}

func reshapeMultiBuffer(ctx context.Context, buffer buf.MultiBuffer) buf.MultiBuffer {
	needReshape := 0
	for _, b := range buffer {
		if b.Len() >= buf.Size-21 {
			needReshape += 1
		}
	}
	if needReshape == 0 {
		return buffer
	}
	mb2 := make(buf.MultiBuffer, 0, len(buffer)+needReshape)
	toPrint := ""
	for i, buffer1 := range buffer {
		if buffer1.Len() >= buf.Size-21 {
			index := int32(bytes.LastIndex(buffer1.Bytes(), TlsApplicationDataStart))
			if index < 21 || index > buf.Size-21 {
				index = buf.Size / 2
			}
			buffer2 := buf.New()
			buffer2.Write(buffer1.BytesFrom(index))
			buffer1.Resize(0, index)
			mb2 = append(mb2, buffer1, buffer2)
			toPrint += " " + strconv.Itoa(int(buffer1.Len())) + " " + strconv.Itoa(int(buffer2.Len()))
		} else {
			mb2 = append(mb2, buffer1)
			toPrint += " " + strconv.Itoa(int(buffer1.Len()))
		}
		buffer[i] = nil
	}
	buffer = buffer[:0]
	errors.LogDebug(ctx, "vision: reshape ", toPrint)
	return mb2
}

func xtlsPadding(b *buf.Buffer, command byte, userUUID *[]byte, longPadding bool, ctx context.Context, testseed []uint32) *buf.Buffer {
	var contentLen int32 = 0
	var paddingLen int32 = 0
	if b != nil {
		contentLen = b.Len()
	}
	if contentLen < int32(testseed[0]) && longPadding {
		l, err := rand.Int(rand.Reader, big.NewInt(int64(testseed[1])))
		if err != nil {
			errors.LogDebugInner(ctx, err, "failed to generate padding")
		}
		paddingLen = int32(l.Int64()) + int32(testseed[2]) - contentLen
	} else {
		l, err := rand.Int(rand.Reader, big.NewInt(int64(testseed[3])))
		if err != nil {
			errors.LogDebugInner(ctx, err, "failed to generate padding")
		}
		paddingLen = int32(l.Int64())
	}
	if paddingLen > buf.Size-21-contentLen {
		paddingLen = buf.Size - 21 - contentLen
	}
	newbuffer := buf.New()
	if userUUID != nil {
		newbuffer.Write(*userUUID)
		*userUUID = nil
	}
	newbuffer.Write([]byte{command, byte(contentLen >> 8), byte(contentLen), byte(paddingLen >> 8), byte(paddingLen)})
	if b != nil {
		newbuffer.Write(b.Bytes())
		b.Release()
		b = nil
	}
	newbuffer.Extend(paddingLen)
	errors.LogDebug(ctx, "vision: padding ", contentLen, " ", paddingLen, " ", command)
	return newbuffer
}

func xtlsUnpadding(b *buf.Buffer, s *TrafficState, isUplink bool, ctx context.Context) *buf.Buffer {
	var RemainingCommand *int32
	var RemainingContent *int32
	var RemainingPadding *int32
	var CurrentCommand *int
	if isUplink {
		RemainingCommand = &s.Inbound.RemainingCommand
		RemainingContent = &s.Inbound.RemainingContent
		RemainingPadding = &s.Inbound.RemainingPadding
		CurrentCommand = &s.Inbound.CurrentCommand
	} else {
		RemainingCommand = &s.Outbound.RemainingCommand
		RemainingContent = &s.Outbound.RemainingContent
		RemainingPadding = &s.Outbound.RemainingPadding
		CurrentCommand = &s.Outbound.CurrentCommand
	}
	if *RemainingCommand == -1 && *RemainingContent == -1 && *RemainingPadding == -1 {
		if b.Len() >= 21 && bytes.Equal(s.UserUUID, b.BytesTo(16)) {
			b.Advance(16)
			*RemainingCommand = 5
		} else {
			return b
		}
	}
	newbuffer := buf.New()
	for b.Len() > 0 {
		if *RemainingCommand > 0 {
			data, err := b.ReadByte()
			if err != nil {
				return newbuffer
			}
			switch *RemainingCommand {
			case 5:
				*CurrentCommand = int(data)
			case 4:
				*RemainingContent = int32(data) << 8
			case 3:
				*RemainingContent = *RemainingContent | int32(data)
			case 2:
				*RemainingPadding = int32(data) << 8
			case 1:
				*RemainingPadding = *RemainingPadding | int32(data)
				errors.LogDebug(ctx, "vision: unpadding new block, content ", *RemainingContent, " padding ", *RemainingPadding, " command ", *CurrentCommand)
			}
			*RemainingCommand--
		} else if *RemainingContent > 0 {
			length := *RemainingContent
			if b.Len() < length {
				length = b.Len()
			}
			data, err := b.ReadBytes(length)
			if err != nil {
				return newbuffer
			}
			newbuffer.Write(data)
			*RemainingContent -= length
		} else {
			length := *RemainingPadding
			if b.Len() < length {
				length = b.Len()
			}
			b.Advance(length)
			*RemainingPadding -= length
		}
		if *RemainingCommand <= 0 && *RemainingContent <= 0 && *RemainingPadding <= 0 {
			if *CurrentCommand == 0 {
				*RemainingCommand = 5
			} else {
				*RemainingCommand = -1
				*RemainingContent = -1
				*RemainingPadding = -1
				if b.Len() > 0 {
					newbuffer.Write(b.Bytes())
				}
				break
			}
		}
	}
	b.Release()
	b = nil
	return newbuffer
}

func xtlsFilterTls(buffer buf.MultiBuffer, trafficState *TrafficState, ctx context.Context) {
	for _, b := range buffer {
		if b == nil {
			continue
		}
		trafficState.NumberOfPacketToFilter--
		if b.Len() >= 6 {
			startsBytes := b.BytesTo(6)
			if bytes.Equal(TlsServerHandShakeStart, startsBytes[:3]) && startsBytes[5] == TlsHandshakeTypeServerHello {
				trafficState.RemainingServerHello = (int32(startsBytes[3])<<8 | int32(startsBytes[4])) + 5
				trafficState.IsTLS12orAbove = true
				trafficState.IsTLS = true
				if b.Len() >= 79 && trafficState.RemainingServerHello >= 79 {
					sessionIdLen := int32(b.Byte(43))
					cipherSuite := b.BytesRange(43+sessionIdLen+1, 43+sessionIdLen+3)
					trafficState.Cipher = uint16(cipherSuite[0])<<8 | uint16(cipherSuite[1])
				} else {
					errors.LogDebug(ctx, "vision: short server hello, tls 1.2 or older? ", b.Len(), " ", trafficState.RemainingServerHello)
				}
			} else if bytes.Equal(TlsClientHandShakeStart, startsBytes[:2]) && startsBytes[5] == TlsHandshakeTypeClientHello {
				trafficState.IsTLS = true
				errors.LogDebug(ctx, "vision: found tls client hello! ", buffer.Len())
			}
		}
		if trafficState.RemainingServerHello > 0 {
			end := trafficState.RemainingServerHello
			if end > b.Len() {
				end = b.Len()
			}
			trafficState.RemainingServerHello -= b.Len()
			if bytes.Contains(b.BytesTo(end), Tls13SupportedVersions) {
				v, ok := Tls13CipherSuiteDic[trafficState.Cipher]
				if !ok {
					v = "Old cipher: " + strconv.FormatUint(uint64(trafficState.Cipher), 16)
				} else if v != "TLS_AES_128_CCM_8_SHA256" {
					trafficState.EnableXtls = true
				}
				errors.LogDebug(ctx, "vision: found tls 1.3! ", b.Len(), " ", v)
				trafficState.NumberOfPacketToFilter = 0
				return
			} else if trafficState.RemainingServerHello <= 0 {
				errors.LogDebug(ctx, "vision: found tls 1.2! ", b.Len())
				trafficState.NumberOfPacketToFilter = 0
				return
			}
			errors.LogDebug(ctx, "vision: inconclusive server hello ", b.Len(), " ", trafficState.RemainingServerHello)
		}
		if trafficState.NumberOfPacketToFilter <= 0 {
			errors.LogDebug(ctx, "vision: stop filtering", buffer.Len())
		}
	}
}
