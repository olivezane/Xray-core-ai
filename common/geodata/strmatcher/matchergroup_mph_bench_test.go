package strmatcher_test

import (
	"fmt"
	"testing"

	. "github.com/xtls/xray-core/common/geodata/strmatcher"
)

// 构造 128K 条 domain 规则, 模拟大型 geosite 规则集 (MPH 构建的内存热点量级).
func mphBuildRules(n int) []Matcher {
	rules := make([]Matcher, 0, n)
	for i := range n {
		m, err := Domain.New(fmt.Sprintf("sub%d.example%d.com", i%1024, i/1024))
		if err != nil {
			panic(err)
		}
		rules = append(rules, m)
	}
	return rules
}

func BenchmarkMphMatcherGroupAdd(b *testing.B) {
	rules := mphBuildRules(131072)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		g := NewMphMatcherGroup()
		for i, m := range rules {
			g.AddDomainMatcher(m.(DomainMatcher), uint32(i))
		}
		if err := g.Build(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMphMatcherGroupMatch(b *testing.B) {
	g := NewMphMatcherGroup()
	rules := mphBuildRules(131072)
	for i, m := range rules {
		g.AddDomainMatcher(m.(DomainMatcher), uint32(i))
	}
	if err := g.Build(); err != nil {
		b.Fatal(err)
	}
	inputs := []string{
		"sub7.example1234.com",            // hit: 父域规则
		"foo.sub1023.example127.com",      // hit: 带子域
		"www.unknown-zone.example127.com", // 部分命中
		"completely.unrelated.org",        // miss
	}
	b.ResetTimer()
	for b.Loop() {
		for _, in := range inputs {
			_ = g.Match(in)
		}
	}
}
