package conf_test

import (
	"strings"
	"testing"

	. "github.com/xtls/xray-core/infra/conf"
)

// SOCKS servers 条目缺 address 时必须报错:Build 目前对 nil Address
// 静默放行(nil-safe Build 返回 nil),错误直到运行时 dial 才暴露。
func TestSocksServerMissingAddressRejected(t *testing.T) {
	v := &SocksClientConfig{
		Servers: []*SocksRemoteConfig{{Port: 1080}},
	}
	_, err := v.Build()
	if err == nil {
		t.Fatal("expected error for missing address, got nil")
	}
	if !strings.Contains(err.Error(), "address") {
		t.Fatalf("error should mention address, got: %v", err)
	}
}

// HTTP servers 条目缺 address 时同样必须报错。
func TestHTTPServerMissingAddressRejected(t *testing.T) {
	v := &HTTPClientConfig{
		Servers: []*HTTPRemoteConfig{{Port: 8080}},
	}
	_, err := v.Build()
	if err == nil {
		t.Fatal("expected error for missing address, got nil")
	}
	if !strings.Contains(err.Error(), "address") {
		t.Fatalf("error should mention address, got: %v", err)
	}
}

// wireguard 密钥错误必须保留可诊断的错误链:xtls errors.New 不支持
// fmt 动词,不能把 %w 当字面量拼进错误文本。
func TestWireGuardInvalidKeyErrorHasNoLiteralPercentW(t *testing.T) {
	v := &WireGuardConfig{SecretKey: ".%%%"}
	_, err := v.Build()
	if err == nil {
		t.Fatal("expected error for invalid secret key, got nil")
	}
	if strings.Contains(err.Error(), "%w") {
		t.Fatalf("error leaks literal %%w, got: %v", err)
	}
}

// proxyProtocol 只接受 0/1/2:非法值必须报错而不是静默退化为未启用。
func TestFreedomProxyProtocolOutOfRangeRejected(t *testing.T) {
	v := &FreedomConfig{ProxyProtocol: 3}
	if _, err := v.Build(); err == nil {
		t.Fatal("expected error for proxyProtocol 3, got nil")
	}
}
