package geodata

import (
	"strconv"

	"github.com/xtls/xray-core/common/net"
)

// FormatDomainRule renders a domain rule in the form ParseDomainRule accepts.
func FormatDomainRule(r *DomainRule) string {
	if r == nil {
		return ""
	}
	if g := r.GetGeosite(); g != nil {
		code := g.GetCode()
		if g.GetAttrs() != "" {
			code += "@" + g.GetAttrs()
		}
		if g.GetFile() == DefaultGeoSiteDat {
			return "geosite:" + code
		}
		return "ext:" + g.GetFile() + ":" + code
	}
	d := r.GetCustom()
	prefix := "domain:"
	switch d.GetType() {
	case Domain_Regex:
		prefix = "regexp:"
	case Domain_Full:
		prefix = "full:"
	case Domain_Substr:
		prefix = "keyword:"
	}
	return prefix + d.GetValue()
}

// FormatIPRule renders an IP rule in the form ParseIPRules accepts.
func FormatIPRule(r *IPRule) string {
	if r == nil {
		return ""
	}
	if g := r.GetGeoip(); g != nil {
		rule := "geoip:" + g.GetCode()
		if g.GetFile() != DefaultGeoIPDat {
			rule = "ext:" + g.GetFile() + ":" + g.GetCode()
		}
		if g.GetReverseMatch() {
			return "!" + rule
		}
		return rule
	}
	c := r.GetCustom()
	if c.GetCidr() == nil {
		return "ip:invalid"
	}
	rule := net.IP(c.GetCidr().GetIp()).String() + "/" + strconv.Itoa(int(c.GetCidr().GetPrefix()))
	if c.GetReverseMatch() {
		return "!" + rule
	}
	return rule
}
