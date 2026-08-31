package router_test

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	. "github.com/xtls/xray-core/app/router"
)

func TestWebhookNotifierFireAndClose(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
	}))
	defer srv.Close()

	h, err := NewWebhookNotifier(&WebhookConfig{Url: srv.URL})
	if err != nil || h == nil {
		t.Fatalf("NewWebhookNotifier = %v, %v", h, err)
	}

	// Fire before Close: event must be delivered before Close returns.
	h.Fire(withBackground(), "outbound1")
	h.Fire(withBackground(), "outbound2")
	if err := h.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("requests = %d, want 2", got)
	}

	// Fire after Close: must not panic, must not deliver.
	h.Fire(withBackground(), "outbound3")
	time.Sleep(50 * time.Millisecond)
	if got := requests.Load(); got != 2 {
		t.Fatalf("requests after Close = %d, want 2", got)
	}
}
