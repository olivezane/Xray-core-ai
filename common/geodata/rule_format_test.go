package geodata_test

import (
	"testing"

	"github.com/xtls/xray-core/common/geodata"
)

// Codes from geosite.dat/geoip.dat are upper-cased by the parser, so the
// formatter renders them back that way.
func TestFormatNilRule(t *testing.T) {
	if got := geodata.FormatDomainRule(nil); got != "" {
		t.Errorf("nil domain rule formats to %q", got)
	}
	if got := geodata.FormatIPRule(&geodata.IPRule{Value: &geodata.IPRule_Custom{Custom: &geodata.CIDRRule{}}}); got != "ip:invalid" {
		t.Errorf("IP rule without CIDR formats to %q", got)
	}
	// A non-default dat file is only readable while parsing, so build it directly.
	rule := &geodata.IPRule{Value: &geodata.IPRule_Geoip{Geoip: &geodata.GeoIPRule{File: "my.dat", Code: "CN"}}}
	if got := geodata.FormatIPRule(rule); got != "ext:my.dat:CN" {
		t.Errorf("external geoip file formats to %q", got)
	}
	domain := &geodata.DomainRule{Value: &geodata.DomainRule_Geosite{Geosite: &geodata.GeoSiteRule{File: "my.dat", Code: "CN"}}}
	if got := geodata.FormatDomainRule(domain); got != "ext:my.dat:CN" {
		t.Errorf("external geosite file formats to %q", got)
	}
}

func TestFormatDomainRule(t *testing.T) {
	for rule, want := range map[string]string{
		"domain:example.com":       "domain:example.com",
		"full:example.com":         "full:example.com",
		"regexp:^abc\\d+$":         "regexp:^abc\\d+$",
		"keyword:example":          "keyword:example",
		"geosite:cn":               "geosite:CN",
		"geosite:cn@ads":           "geosite:CN@ads",
		"geosite:category-ads-all": "geosite:CATEGORY-ADS-ALL",
	} {
		r, err := geodata.ParseDomainRule(rule, geodata.Domain_Substr)
		if err != nil {
			t.Fatalf("%s: %v", rule, err)
		}
		if got := geodata.FormatDomainRule(r); got != want {
			t.Errorf("%s formats to %s, want %s", rule, got, want)
		}
	}
}

func TestFormatIPRule(t *testing.T) {
	for rule, want := range map[string]string{
		"10.0.0.0/8":      "10.0.0.0/8",
		"!192.168.0.0/16": "!192.168.0.0/16",
		"2001:db8::1/32":  "2001:db8::1/32",
		"geoip:cn":        "geoip:CN",
		"!geoip:private":  "!geoip:PRIVATE",
	} {
		rules, err := geodata.ParseIPRules([]string{rule})
		if err != nil {
			t.Fatalf("%s: %v", rule, err)
		}
		if got := geodata.FormatIPRule(rules[0]); got != want {
			t.Errorf("%s formats to %s, want %s", rule, got, want)
		}
	}
}
