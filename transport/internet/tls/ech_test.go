package tls

import (
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
)

func TestECHDial(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		server string
	}{
		{"udp", "udp://1.1.1.1"},
		{"doh", "https://cloudflare-dns.com/dns-query"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			server := testCase.server
			config := &Config{
				ServerName:    "cloudflare.com",
				EchConfigList: "encryptedsni.com+" + server,
			}
			// The ECH config comes from the resolver and the dial goes to
			// cloudflare.com, so both need the internet. A resolver that
			// strips HTTPS records leaves ApplyECH with an invalid config on
			// purpose, so ask before dialing and skip instead of failing.
			if _, _, err := dnsQuery(server, "encryptedsni.com", nil); err != nil {
				t.Skip("no ECH config from ", server, ": ", err)
			}

			// test concurrent Dial(to test cache problem)
			wg := sync.WaitGroup{}
			for range 10 {
				wg.Go(func() {
					TLSConfig := config.GetTLSConfig()
					TLSConfig.NextProtos = []string{"http/1.1"}
					client := &http.Client{
						Transport: &http.Transport{
							TLSClientConfig: TLSConfig,
						},
					}
					resp, err := client.Get("https://cloudflare.com/cdn-cgi/trace")
					if err != nil {
						t.Error("ECH dial failed: ", err)
						return
					}
					defer resp.Body.Close()
					body, err := io.ReadAll(resp.Body)
					if err != nil {
						t.Error("failed to read the response body: ", err)
						return
					}
					if !strings.Contains(string(body), "sni=encrypted") {
						t.Error("ECH Dial success but SNI is not encrypted")
					}
				})
			}
			wg.Wait()

			// check cache
			echConfigCache, ok := GlobalECHConfigCache.Load(ECHCacheKey(server, "encryptedsni.com", nil))
			if !ok {
				t.Fatal("ECH config cache not found")
			}
			ok = echConfigCache.UpdateLock.TryLock()
			if !ok {
				t.Error("ECH config cache dead lock detected")
			}
			echConfigCache.UpdateLock.Unlock()
			configRecord := echConfigCache.configRecord.Load()
			if configRecord == nil {
				t.Error("ECH config record not found in cache")
			}
		})
	}
}

func TestECHDialFail(t *testing.T) {
	config := &Config{
		ServerName:    "cloudflare.com",
		EchConfigList: "udp://0.0.0.0",
	}
	tlsConfig := config.GetTLSConfig()
	ApplyECH(config, tlsConfig)
	if !slices.Equal(tlsConfig.EncryptedClientHelloConfigList, []byte{1, 1, 4, 5, 1, 4}) {
		t.Error("ECH config should be invalid when query failed", " but got ", tlsConfig.EncryptedClientHelloConfigList)
	}
}
