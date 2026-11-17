package utils

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

const testPaddedErr = `Post "https://doh/": http2: request header list larger than peer's advertised limit`

type rejectPaddedTransport struct {
	calls int
	body  string
}

func (t *rejectPaddedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.calls++
	if req.Header.Get("X-Padding") != "" {
		req.Body.Close()
		return nil, errors.New(testPaddedErr)
	}
	b, _ := io.ReadAll(req.Body)
	t.body = string(b)
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(b)), Header: http.Header{}}, nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestDoWithH2PaddingRetriesUnpadded(t *testing.T) {
	rt := &rejectPaddedTransport{}
	// Timeout makes http.Client wrap req.Body, exercising the replay path for real.
	client := &http.Client{Transport: rt, Timeout: 5 * time.Second}

	req, err := http.NewRequest("POST", "https://doh/dns-query", bytes.NewBufferString("dns-query-bytes"))
	if err != nil {
		t.Fatal(err)
	}
	SetH2Padding(req.Header)

	resp, err := DoWithH2Padding(client, req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if rt.calls != 2 {
		t.Errorf("want 2 attempts, got %d", rt.calls)
	}
	if rt.body != "dns-query-bytes" {
		t.Errorf("body not replayed: %q", rt.body)
	}
	if req.Header.Get("X-Padding") != "" {
		t.Error("X-Padding not dropped")
	}
}

func TestDoWithH2PaddingNoRetryOnOtherErrors(t *testing.T) {
	calls := 0
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return nil, errors.New("boom")
	})}
	req, _ := http.NewRequest("POST", "https://doh/dns-query", strings.NewReader("x"))
	SetH2Padding(req.Header)
	_, err := DoWithH2Padding(client, req)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("want boom, got %v", err)
	}
	if calls != 1 {
		t.Errorf("want 1 attempt, got %d", calls)
	}
}

func TestDoWithH2PaddingNoRetryWithoutGetBody(t *testing.T) {
	calls := 0
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return nil, errors.New(testPaddedErr)
	})}
	req, _ := http.NewRequest("POST", "https://doh/dns-query", io.NopCloser(strings.NewReader("x")))
	req.GetBody = nil // streamed body cannot be replayed
	SetH2Padding(req.Header)
	_, err := DoWithH2Padding(client, req)
	if err == nil || !strings.Contains(err.Error(), errHeaderListTooLarge) {
		t.Fatalf("want the header-list error, got %v", err)
	}
	if calls != 1 {
		t.Errorf("want 1 attempt, got %d", calls)
	}
}

func TestSetH2PaddingFitsOpenDNS(t *testing.T) {
	// 1086 bytes of browser headers measured for the "fetch" variant plus the
	// padding field itself must stay under doh.opendns.com's 2048-byte limit.
	for range 1000 {
		h := http.Header{}
		SetH2Padding(h)
		if size := len("X-Padding") + len(h.Get("X-Padding")) + 32; 1086+size > 2048 {
			t.Fatalf("padding header too large: %d bytes", size)
		}
	}
}
