package utils

import (
	"math/rand/v2"
	"net/http"
	"strings"
)

var (
	// 8 ÷ (397/62)
	h2packCorrectionFactor = 1.2493702770780857
	base62TotalCharsNum    = 62
	base62Chars            = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
)

// errHeaderListTooLarge is the text of httpcommon.ErrRequestHeaderListSize
// (golang.org/x/net/internal/httpcommon/request.go), which x/net/http2 does not export.
const errHeaderListTooLarge = "request header list larger than peer's advertised limit"

// h2PadMaxLen bounds the padding so the whole header list stays under the 2048-byte
// SETTINGS_MAX_HEADER_LIST_SIZE some servers advertise (e.g. doh.opendns.com);
// the browser-like headers alone already take ~1086 bytes.
const h2PadMaxLen = 600

// H2Base62Pad generates a base62 padding string for HTTP/2 header
// The total len will be slightly longer than the input to match the length after h2(h3 also) header huffman encoding
func H2Base62Pad[T int32 | int64 | int](expectedLen T) string {
	actualLenFloat := float64(expectedLen) * h2packCorrectionFactor
	actualLen := int(actualLenFloat)
	result := make([]byte, actualLen)
	for i := range actualLen {
		result[i] = base62Chars[rand.N(base62TotalCharsNum)]
	}
	return string(result)
}

// SetH2Padding sets a random-length X-Padding header.
func SetH2Padding(header http.Header) {
	header.Set("X-Padding", H2Base62Pad(rand.N(h2PadMaxLen-100+1)+100))
}

// DoWithH2Padding sends req and, if the peer's SETTINGS_MAX_HEADER_LIST_SIZE
// rejects the padded header list, retries once with X-Padding dropped.
// It deletes X-Padding from req, so pass a request you own.
func DoWithH2Padding(client *http.Client, req *http.Request) (*http.Response, error) {
	resp, err := client.Do(req)
	if err == nil || req.Header.Get("X-Padding") == "" || !strings.Contains(err.Error(), errHeaderListTooLarge) {
		return resp, err
	}
	if req.Body != nil && req.GetBody == nil {
		return resp, err
	}
	if req.GetBody != nil {
		body, gerr := req.GetBody()
		if gerr != nil {
			return resp, err
		}
		req.Body = body
	}
	req.Header.Del("X-Padding")
	return client.Do(req)
}
