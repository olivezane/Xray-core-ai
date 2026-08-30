// Compatibility surface: upstream Xray-core exported these XTLS-Vision state
// types and TLS constants from package proxy. The fork moved the
// implementation to proxy/vision; keep the upstream symbols here so code
// importing xray-core as a library keeps compiling.
package proxy

import "github.com/xtls/xray-core/proxy/vision"

type (
	TrafficState  = vision.TrafficState
	InboundState  = vision.InboundState
	OutboundState = vision.OutboundState
)

var (
	Tls13SupportedVersions  = vision.Tls13SupportedVersions
	TlsClientHandShakeStart = vision.TlsClientHandShakeStart
	TlsServerHandShakeStart = vision.TlsServerHandShakeStart
	TlsApplicationDataStart = vision.TlsApplicationDataStart
	Tls13CipherSuiteDic     = vision.Tls13CipherSuiteDic
)

const (
	TlsHandshakeTypeClientHello byte = 0x01
	TlsHandshakeTypeServerHello byte = 0x02

	CommandPaddingContinue byte = 0x00
	CommandPaddingEnd      byte = 0x01
	CommandPaddingDirect   byte = 0x02
)

func NewTrafficState(userUUID []byte) *TrafficState {
	return vision.NewTrafficState(userUUID)
}
