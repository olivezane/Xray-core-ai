package proxy_test

import (
	"testing"

	"github.com/xtls/xray-core/proxy"
)

// Upstream API surface must keep compiling: TrafficState with exported
// Inbound/Outbound state structs, plus the TLS constants used by XTLS.
func TestUpstreamCompatSurface(t *testing.T) {
	s := proxy.NewTrafficState([]byte("1234567890123456"))
	if len(s.UserUUID) != 16 {
		t.Fatalf("expected 16-byte uuid, got %d", len(s.UserUUID))
	}
	s.Inbound.WithinPaddingBuffers = false
	s.Outbound.DownlinkReaderDirectCopy = true
	s.Inbound.CurrentCommand = 2
	s.Outbound.UplinkWriterDirectCopy = true

	if proxy.Tls13SupportedVersions[0] != 0x00 {
		t.Fatal("Tls13SupportedVersions mismatch")
	}
	if proxy.TlsClientHandShakeStart[0] != 0x16 {
		t.Fatal("TlsClientHandShakeStart mismatch")
	}
	if proxy.TlsServerHandShakeStart[0] != 0x16 {
		t.Fatal("TlsServerHandShakeStart mismatch")
	}
	if proxy.TlsApplicationDataStart[0] != 0x17 {
		t.Fatal("TlsApplicationDataStart mismatch")
	}
	if _, ok := proxy.Tls13CipherSuiteDic[0x1301]; !ok {
		t.Fatal("Tls13CipherSuiteDic missing 0x1301")
	}
	if proxy.TlsHandshakeTypeClientHello != 0x01 || proxy.TlsHandshakeTypeServerHello != 0x02 {
		t.Fatal("handshake type constants mismatch")
	}
	if proxy.CommandPaddingContinue != 0x00 || proxy.CommandPaddingEnd != 0x01 || proxy.CommandPaddingDirect != 0x02 {
		t.Fatal("command padding constants mismatch")
	}
}
