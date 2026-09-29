package router

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"maps"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/utils"
	"github.com/xtls/xray-core/features/routing"
	routing_session "github.com/xtls/xray-core/features/routing/session"
)

type event struct {
	Email          *string `json:"email"`
	Level          *uint32 `json:"level"`
	Protocol       *string `json:"protocol"`
	Network        *string `json:"network"`
	Source         *string `json:"source"`
	Destination    *string `json:"destination"`
	OriginalTarget *string `json:"originalTarget"`
	RouteTarget    *string `json:"routeTarget"`
	InboundTag     *string `json:"inboundTag"`
	InboundName    *string `json:"inboundName"`
	InboundLocal   *string `json:"inboundLocal"`
	OutboundTag    *string `json:"outboundTag"`
	Timestamp      int64   `json:"ts"`
}

type WebhookNotifier struct {
	url           string
	headers       map[string]string
	deduplication uint32
	client        *http.Client
	seen          sync.Map
	lastSweep     atomic.Int64
	done          chan struct{}
	wg            sync.WaitGroup
	closeOnce     sync.Once
}

func NewWebhookNotifier(cfg *WebhookConfig) (*WebhookNotifier, error) {
	if cfg == nil || cfg.Url == "" {
		return nil, nil
	}

	httpURL, socketPath := utils.SplitHTTPUnixURL(cfg.Url)
	h := &WebhookNotifier{
		url:           httpURL,
		deduplication: cfg.Deduplication,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		done: make(chan struct{}),
	}

	if socketPath != "" {
		dialAddr := utils.ResolveSocketPath(socketPath)
		h.client.Transport = &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", dialAddr)
			},
		}
	}

	if len(cfg.Headers) > 0 {
		h.headers = make(map[string]string, len(cfg.Headers))
		maps.Copy(h.headers, cfg.Headers)
	}

	return h, nil
}

func (h *WebhookNotifier) Fire(ctx routing.Context, outboundTag string) {
	ev := buildEvent(ctx, outboundTag)

	email := ""
	if ev.Email != nil {
		email = *ev.Email
	}
	if h.isDuplicate(email) {
		return
	}

	h.wg.Add(1)
	select {
	case <-h.done:
		h.wg.Done()
		return
	default:
	}
	go func() {
		defer h.wg.Done()
		h.post(ev)
	}()
}

func buildEvent(ctx routing.Context, outboundTag string) *event {
	ev := &event{
		Timestamp:   time.Now().Unix(),
		OutboundTag: new(outboundTag),
		InboundTag:  new(ctx.GetInboundTag()),
		Protocol:    new(ctx.GetProtocol()),
		Network:     new(ctx.GetNetwork().SystemString()),
	}

	if user := ctx.GetUser(); user != "" {
		ev.Email = new(user)
	}

	if srcIPs := ctx.GetSourceIPs(); len(srcIPs) > 0 {
		srcPort := ctx.GetSourcePort()
		ev.Source = new(net.JoinHostPort(srcIPs[0].String(), srcPort.String()))
	}

	targetPort := ctx.GetTargetPort()
	if domain := ctx.GetTargetDomain(); domain != "" {
		ev.Destination = new(net.JoinHostPort(domain, targetPort.String()))
	} else if targetIPs := ctx.GetTargetIPs(); len(targetIPs) > 0 {
		ev.Destination = new(net.JoinHostPort(targetIPs[0].String(), targetPort.String()))
	}

	if localIPs := ctx.GetLocalIPs(); len(localIPs) > 0 {
		localPort := ctx.GetLocalPort()
		ev.InboundLocal = new(net.JoinHostPort(localIPs[0].String(), localPort.String()))
	}

	if sctx, ok := ctx.(*routing_session.Context); ok {
		enrichFromSession(ev, sctx)
	}

	return ev
}

func enrichFromSession(ev *event, sctx *routing_session.Context) {
	if sctx.Inbound != nil {
		ev.InboundName = new(sctx.Inbound.Name)
		if sctx.Inbound.User != nil {
			ev.Level = new(sctx.Inbound.User.Level)
		}
	}
	if sctx.Outbound != nil {
		if sctx.Outbound.OriginalTarget.Address != nil {
			ev.OriginalTarget = new(sctx.Outbound.OriginalTarget.String())
		}
		if sctx.Outbound.RouteTarget.Address != nil {
			ev.RouteTarget = new(sctx.Outbound.RouteTarget.String())
		}
	}
}

func (h *WebhookNotifier) post(ev *event) {
	body, err := json.Marshal(ev)
	if err != nil {
		errors.LogWarning(context.Background(), "webhook: marshal failed: ", err)
		return
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, h.url, bytes.NewReader(body))
	if err != nil {
		errors.LogWarning(context.Background(), "webhook: request build failed: ", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	for k, v := range h.headers {
		req.Header.Set(k, v)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		errors.LogInfo(context.Background(), "webhook: POST failed: ", err)
		return
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()
	if resp.StatusCode >= 400 {
		errors.LogWarning(context.Background(), "webhook: POST returned status ", resp.StatusCode)
	}
}

func (h *WebhookNotifier) isDuplicate(email string) bool {
	if h.deduplication == 0 || email == "" {
		return false
	}
	ttl := time.Duration(h.deduplication) * time.Second
	now := time.Now()
	h.maybeSweep(now, ttl)
	if v, loaded := h.seen.LoadOrStore(email, now); loaded {
		//nolint:forcetypeassert // the webhook cache only ever stores time.Time
		if now.Sub(v.(time.Time)) < ttl {
			return true
		}
		h.seen.Store(email, now)
	}
	return false
}

func (h *WebhookNotifier) maybeSweep(now time.Time, ttl time.Duration) {
	last := h.lastSweep.Load()
	if now.UnixNano()-last < int64(ttl) {
		return
	}
	if !h.lastSweep.CompareAndSwap(last, now.UnixNano()) {
		return // another goroutine did the sweep
	}
	h.seen.Range(func(key, value any) bool {
		//nolint:forcetypeassert // the webhook cache only ever stores time.Time
		if now.Sub(value.(time.Time)) >= ttl {
			h.seen.Delete(key)
		}
		return true
	})
}

// Only need to call if the Notifier is really used, otherwise GC can clean it
func (h *WebhookNotifier) Close() error {
	h.closeOnce.Do(func() {
		close(h.done)
	})
	h.wg.Wait()
	h.client.CloseIdleConnections()
	return nil
}
