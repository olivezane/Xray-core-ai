package geodata

import (
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/xtls/xray-core/common/geodata/strmatcher"
	"github.com/xtls/xray-core/common/utils"
)

func TestCompactDomainMatcher_PreservesCustomRuleIndices(t *testing.T) {
	factory := &CompactDomainMatcherFactory{shared: utils.NewWeakCacheMap[string, strmatcher.LinearAnyMatcher]()}
	matcher, err := factory.BuildMatcher([]*DomainRule{
		{Value: &DomainRule_Custom{Custom: &Domain{Type: Domain_Full, Value: "example.com"}}},
		{Value: &DomainRule_Custom{Custom: &Domain{Type: Domain_Domain, Value: "example.com"}}},
	})
	if err != nil {
		t.Fatalf("BuildMatcher() failed: %v", err)
	}

	got := matcher.Match("example.com")
	slices.Sort(got)

	want := []uint32{0, 1}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Match() = %v, want %v", got, want)
	}
}

func TestCompactDomainMatcher_PreservesMixedRuleIndices(t *testing.T) {
	t.Setenv("xray.location.asset", filepath.Join("..", "..", "resources"))

	factory := &CompactDomainMatcherFactory{shared: utils.NewWeakCacheMap[string, strmatcher.LinearAnyMatcher]()}
	matcher, err := factory.BuildMatcher([]*DomainRule{
		{Value: &DomainRule_Geosite{Geosite: &GeoSiteRule{File: DefaultGeoSiteDat, Code: "CN"}}},
		{Value: &DomainRule_Custom{Custom: &Domain{Type: Domain_Full, Value: "163.com"}}},
	})
	if err != nil {
		t.Fatalf("BuildMatcher() failed: %v", err)
	}

	got := matcher.Match("163.com")
	slices.Sort(got)

	want := []uint32{0, 1}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Match() = %v, want %v", got, want)
	}
}

func TestMphDomainMatcher_MatchReturnsDetachedSlice(t *testing.T) {
	matcher, err := (&MphDomainMatcherFactory{shared: utils.NewWeakCacheMap[string, strmatcher.MphValueMatcher]()}).
		BuildMatcher([]*DomainRule{
			{Value: &DomainRule_Custom{Custom: &Domain{Type: Domain_Full, Value: "example.com"}}},
			{Value: &DomainRule_Custom{Custom: &Domain{Type: Domain_Domain, Value: "example.com"}}},
		})
	if err != nil {
		t.Fatalf("BuildMatcher() failed: %v", err)
	}

	got := matcher.Match("example.com")
	if !reflect.DeepEqual(got, []uint32{0, 1}) {
		t.Fatalf("Match() = %v, want %v", got, []uint32{0, 1})
	}

	got[0] = 1

	gotAgain := matcher.Match("example.com")
	if !reflect.DeepEqual(gotAgain, []uint32{0, 1}) {
		t.Fatalf("Match() after caller mutation = %v, want %v", gotAgain, []uint32{0, 1})
	}
}

// Upstream default: desktop/server platforms select the MPH matcher; mobile
// selects the compact one. The fork's env override may only refine this, never
// change the default selection.
func TestSelectDomainMatcherFactory_upstreamGOOSDefault(t *testing.T) {
	if _, ok := selectDomainMatcherFactory("linux").(*MphDomainMatcherFactory); !ok {
		t.Fatalf("linux: expected MPH matcher factory (upstream default), got %T", selectDomainMatcherFactory("linux"))
	}
	if _, ok := selectDomainMatcherFactory("android").(*CompactDomainMatcherFactory); !ok {
		t.Fatalf("android: expected Compact matcher factory (upstream default), got %T", selectDomainMatcherFactory("android"))
	}
	if _, ok := selectDomainMatcherFactory("ios").(*CompactDomainMatcherFactory); !ok {
		t.Fatalf("ios: expected Compact matcher factory (upstream default), got %T", selectDomainMatcherFactory("ios"))
	}
}

func TestNewDomainMatcherFactory_envOverridesOnly(t *testing.T) {
	t.Setenv("XRAY_GEODATA_MATCHER", "mph")
	if _, ok := newDomainMatcherFactory().(*MphDomainMatcherFactory); !ok {
		t.Fatalf("XRAY_GEODATA_MATCHER=mph: expected MPH factory, got %T", newDomainMatcherFactory())
	}
	t.Setenv("XRAY_GEODATA_MATCHER", "compact")
	if _, ok := newDomainMatcherFactory().(*CompactDomainMatcherFactory); !ok {
		t.Fatalf("XRAY_GEODATA_MATCHER=compact: expected Compact factory, got %T", newDomainMatcherFactory())
	}
}
