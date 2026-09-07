package hysteria

import (
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

// TestMasqHandlerUnixProxy: masquerade proxy 走 unix socket 上游时必须用
// 配置里的真实 socket 路径拨号。修复前代码先把 MasqUrl 替换成
// localhost URL 才取 path,导致 dial 路径恒为空,所有请求必然 502。
func TestMasqHandlerUnixProxy(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "masq.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	upstream := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("masq-ok"))
	})}
	defer upstream.Close()
	go upstream.Serve(ln) //nolint:errcheck

	cfg := &Config{MasqType: "proxy", MasqUrl: "unix://" + sock}
	h, err := buildMasqHandler(cfg)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://example.com/anything", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	if body := rr.Body.String(); body != "masq-ok" {
		t.Fatalf("body = %q, want masq-ok", body)
	}
}
