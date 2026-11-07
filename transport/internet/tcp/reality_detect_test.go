package tcp

import (
	"net"
	"strconv"
	"testing"
	"time"

	goreality "github.com/xtls/reality"
)

func TestClassifyRealityProbe(t *testing.T) {
	for _, tc := range []struct {
		name    string
		present bool
		value   any
		state   int
		records int
	}{
		{"absent", false, nil, realityProbePending, 0},
		{"probing", true, false, realityProbePending, 0},
		{"drifted", true, "unknown", realityProbePending, 0},
		{"empty", true, []int{}, realityProbeEmpty, 0},
		{"done", true, []int{100, 200}, realityProbeOK, 2},
	} {
		state, records := classifyRealityProbe(tc.present, tc.value)
		if state != tc.state || records != tc.records {
			t.Errorf("%s: got (%d, %d), want (%d, %d)", tc.name, state, records, tc.state, tc.records)
		}
	}
}

// TestRealityProbeKeyFormat pins the key that upstream records probe results
// under, which reality_detect.go builds by hand: a format change upstream would
// otherwise turn every probe into a "no probe result" warning.
func TestRealityProbeKeyFormat(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	if err := l.Close(); err != nil { // the probe fails fast on a closed port, which is fine: it still records a result
		t.Fatal(err)
	}
	config := &goreality.Config{
		Type:        "tcp",
		Dest:        addr,
		ServerNames: map[string]bool{"a.example.com": true},
	}
	t.Cleanup(func() {
		for alpn := range 3 {
			goreality.GlobalPostHandshakeRecordsLens.Delete(addr + " a.example.com " + strconv.Itoa(alpn))
		}
	})

	goreality.DetectPostHandshakeRecordsLens(config)

	key := probeKey(addr, "a.example.com")
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if state, _ := realityProbeState(key); state != realityProbePending {
			return
		}
	}
	t.Fatalf("upstream recorded no probe result under %q, its key format changed", key)
}
