package tcp

import (
	"context"
	"time"

	goreality "github.com/xtls/reality"
	"github.com/xtls/xray-core/common/errors"
)

const (
	// no usable result: upstream retries every 5s on each connection, so handshakes hang.
	realityProbePending = iota
	// recorded but empty: nothing to mimic.
	realityProbeEmpty
	// recorded and usable.
	realityProbeOK
)

// reportRealityProbe reports what DetectPostHandshakeRecordsLens collected from
// the target, because upstream falls back to plain Go TLS behaviour silently.
// One goroutine per inbound, so several inbounds sharing a target log it twice.
func reportRealityProbe(ctx context.Context, config *goreality.Config) {
	// shortcut: report only the h2 key suffix (" 2"), the ALPN Xray clients send;
	// revisit if REALITY grows a probe result API worth reporting per ALPN.
	type target struct{ sni, key string }
	targets := make([]target, 0, len(config.ServerNames))
	for sni := range config.ServerNames {
		targets = append(targets, target{sni, probeKey(config.Dest, sni)})
	}
	if len(targets) == 0 {
		errors.LogWarning(ctx, "REALITY: no serverNames configured, every connection will be forwarded to the target and REALITY itself will not be used")
		return
	}
	for range 20 { // the probe dials the target without a timeout, give it up to a minute
		time.Sleep(3 * time.Second)
		done := true
		for _, t := range targets {
			if state, _ := realityProbeState(t.key); state == realityProbePending {
				done = false
				break
			}
		}
		if done {
			break
		}
	}
	for _, t := range targets {
		state, records := realityProbeState(t.key)
		switch state {
		case realityProbePending:
			errors.LogWarning(ctx, "REALITY: no probe result for ", t.sni, " (h2) on ", config.Dest, ", REALITY retries every 5s per connection, so handshakes may hang; the probe probably failed (target unreachable, its certificate does not cover this serverName, no TLS 1.3) or the probe result format changed upstream")
		case realityProbeEmpty:
			errors.LogWarning(ctx, "REALITY: the probe for ", t.sni, " (h2) collected no post-handshake records from ", config.Dest, ", so REALITY will not mimic them; the target may send none, or the probe failed")
		case realityProbeOK:
			errors.LogInfo(ctx, "REALITY: target probe for ", t.sni, " (h2) collected ", records, " post-handshake records")
		}
	}
}

// probeKey builds the key upstream records probe results under, see
// GlobalPostHandshakeRecordsLens in xtls/reality.
func probeKey(dest, sni string) string {
	return dest + " " + sni + " 2" // " 2" is the h2 entry
}

func realityProbeState(key string) (state, records int) {
	value, ok := goreality.GlobalPostHandshakeRecordsLens.Load(key)
	return classifyRealityProbe(ok, value)
}

// classifyRealityProbe maps one GlobalPostHandshakeRecordsLens entry to a state.
// Upstream stores bool while probing and []int once done, so an unknown type is
// reported as pending: upstream's handshake loop waits for a []int forever.
func classifyRealityProbe(present bool, value any) (state, records int) {
	if !present {
		return realityProbePending, 0
	}
	switch v := value.(type) {
	case bool:
		return realityProbePending, 0
	case []int:
		if len(v) == 0 {
			return realityProbeEmpty, 0
		}
		return realityProbeOK, len(v)
	default:
		return realityProbePending, 0
	}
}
