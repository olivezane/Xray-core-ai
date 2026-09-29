package debug

import (
	"net/http"
	"time"
)

func init() {
	go func() {
		// Timeouts keep a stalled client from pinning a connection open forever.
		server := &http.Server{
			Addr:              ":6060",
			ReadHeaderTimeout: 10 * time.Second,
			IdleTimeout:       60 * time.Second,
		}
		_ = server.ListenAndServe()
	}()
}
