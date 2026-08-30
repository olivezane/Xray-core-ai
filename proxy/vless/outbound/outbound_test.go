package outbound

import (
	"net"
	"testing"

	"github.com/xtls/xray-core/proxy/vless/encryption"
)

type unsupportedConn struct {
	net.Conn
}

// extractConnBuffers must fail closed for connection types that cannot carry
// XTLS-Vision buffers, so the vision path never runs with zeroed pointers.
func TestCheckConnType(t *testing.T) {
	_, _, err := extractConnBuffers(&unsupportedConn{}, &unsupportedConn{})
	if err == nil {
		t.Error("expected error for unsupported connection type, got nil")
	}
}

func TestCheckConnTypeNil(t *testing.T) {
	_, _, err := extractConnBuffers(nil, nil)
	if err == nil {
		t.Error("expected error for nil connection, got nil")
	}
}

func TestCheckConnTypeCommonConn(t *testing.T) {
	inner := &unsupportedConn{}
	cc := &encryption.CommonConn{Conn: inner}
	input, rawInput, err := extractConnBuffers(cc, inner)
	if err != nil {
		t.Fatalf("expected CommonConn extraction to succeed, got: %v", err)
	}
	if input == nil || rawInput == nil {
		t.Fatal("expected non-nil input/rawInput buffers from CommonConn")
	}
}
