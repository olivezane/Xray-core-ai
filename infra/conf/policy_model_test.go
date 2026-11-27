package conf_test

import (
	"encoding/json"
	"testing"

	"github.com/xtls/xray-core/app/policy"
	"github.com/xtls/xray-core/core/config"
	. "github.com/xtls/xray-core/infra/conf"
)

// TestPolicyModelRoundTrip checks the boundary between the configuration model
// (plain Go, core/config) and the protobuf message the instance consumes: the
// units of the model (seconds, kilobytes) must come out as the units of the
// message (Second, bytes), and an absent value must stay absent instead of
// becoming zero.
func TestPolicyModelRoundTrip(t *testing.T) {
	const input = `{
		"levels": {
			"0": {"handshake": 4, "connIdle": 300, "uplinkOnly": 2, "downlinkOnly": 5,
			      "statsUserUplink": true, "statsUserDownlink": true, "bufferSize": 64},
			"1": {"bufferSize": -1}
		},
		"system": {"statsInboundUplink": true, "statsOutboundDownlink": true}
	}`

	// The model decodes straight from the user's JSON.
	var set config.PolicySet
	if err := json.Unmarshal([]byte(input), &set); err != nil {
		t.Fatalf("decoding the model failed: %v", err)
	}
	if set.Levels[0].Handshake == nil || *set.Levels[0].Handshake != 4 {
		t.Fatalf("model did not take handshake: %#v", set.Levels[0])
	}

	pc, err := BuildPolicy(&set)
	if err != nil {
		t.Fatalf("building the message failed: %v", err)
	}

	level0 := pc.Level[0]
	if level0.Timeout.Handshake.Value != 4 || level0.Timeout.ConnectionIdle.Value != 300 {
		t.Errorf("timeouts lost: %#v", level0.Timeout)
	}
	if level0.Timeout.UplinkOnly.Value != 2 || level0.Timeout.DownlinkOnly.Value != 5 {
		t.Errorf("timeouts lost: %#v", level0.Timeout)
	}
	if !level0.Stats.UserUplink || !level0.Stats.UserDownlink || level0.Stats.UserOnline {
		t.Errorf("stats switches lost: %#v", level0.Stats)
	}
	// The model is in kilobytes, the message in bytes.
	if level0.Buffer.Connection != 64*1024 {
		t.Errorf("buffer = %d, want %d", level0.Buffer.Connection, 64*1024)
	}
	// A negative size disables buffering instead of scaling.
	if pc.Level[1].Buffer.Connection != -1 {
		t.Errorf("negative buffer = %d, want -1", pc.Level[1].Buffer.Connection)
	}
	if !pc.System.Stats.InboundUplink || !pc.System.Stats.OutboundDownlink {
		t.Errorf("system policy lost: %#v", pc.System)
	}
	if pc.System.Stats.InboundDownlink || pc.System.Stats.OutboundUplink {
		t.Errorf("system policy gained a switch: %#v", pc.System)
	}
}

// TestPolicyKeepsUnsetValuesUnset checks the case the model has to express and
// the message cannot: a timeout the user did not write must not become 0.
func TestPolicyKeepsUnsetValuesUnset(t *testing.T) {
	set := &config.PolicySet{Levels: map[uint32]*config.Policy{0: {}}}

	pc, err := BuildPolicy(set)
	if err != nil {
		t.Fatalf("building the message failed: %v", err)
	}
	level := pc.Level[0]
	if level.Timeout.Handshake != nil || level.Timeout.ConnectionIdle != nil {
		t.Errorf("unset timeouts became values: %#v", level.Timeout)
	}
	if level.Buffer != nil {
		t.Errorf("an absent buffer size became %#v", level.Buffer)
	}

	// A level without a policy stays absent rather than becoming an empty policy.
	set = &config.PolicySet{Levels: map[uint32]*config.Policy{0: nil}}
	pc, err = BuildPolicy(set)
	if err != nil {
		t.Fatalf("building the message failed: %v", err)
	}
	if _, found := pc.Level[0]; found {
		t.Error("an empty level produced a policy")
	}

	// A configuration without a policy section produces no message at all.
	pc, err = BuildPolicy(nil)
	if err != nil || pc != nil {
		t.Errorf("empty policy section = %v, %v", pc, err)
	}

	var _ *policy.Config = pc
}
