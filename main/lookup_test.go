package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLookupCommand(t *testing.T) {
	if os.Getenv("XRAY_TEST_LOOKUP_COMMAND") == "1" && len(flag.Args()) > 0 {
		os.Args = append([]string{os.Args[0]}, flag.Args()...)
		main()
		return
	}
	dir := t.TempDir()
	config := filepath.Join(dir, "config.json")
	body := `{
  "dns": {
    "hosts": {"host.test": "203.0.113.9"},
    "servers": ["8.8.8.8", {"address": "1.1.1.1", "domains": ["domain:cn"], "tag": "cn"}]
  },
  "routing": {
    "domainStrategy": "IPIfNonMatch",
    "rules": [
      {"type": "field", "ruleTag": "cn-sites", "domain": ["domain:cn"], "outboundTag": "direct"},
      {"type": "field", "ip": ["10.0.0.0/8"], "outboundTag": "block"},
      {"type": "field", "port": "53", "network": "udp", "outboundTag": "dns-out"},
      {"type": "field", "inboundTag": ["socks"], "outboundTag": "block"},
      {"type": "field", "process": ["curl"], "outboundTag": "block"}
    ]
  },
  "inbounds": [{"tag": "socks", "listen": "127.0.0.1", "port": 1080, "protocol": "socks", "settings": {}}],
  "outbounds": [
    {"protocol": "freedom", "tag": "direct"},
    {"protocol": "blackhole", "tag": "block"},
    {"protocol": "dns", "tag": "dns-out"}
  ]
}`
	balancer := filepath.Join(dir, "balancer.json")
	balancerBody := `{
  "routing": {
    "balancers": [{"tag": "relay", "selector": ["hk"], "strategy": {"type": "random"}}],
    "rules": [{"type": "field", "domain": ["domain:cn"], "balancerTag": "relay"}]
  },
  "outbounds": [{"protocol": "freedom", "tag": "hk1"}, {"protocol": "freedom", "tag": "hk2"}]
}`
	final := filepath.Join(dir, "final.json")
	finalBody := `{
  "routing": {"rules": [{"type": "field", "domain": ["domain:cn"], "outboundTag": "direct"}]},
  "outbounds": [{"protocol": "freedom", "tag": "direct", "settings": {"firewall": [
    {"action": "block", "port": "25"},
    {"action": "block", "ip": ["10.0.0.0/8"]}
  ]}}]
}`
	server := filepath.Join(dir, "server.json")
	serverBody := `{
  "inbounds": [
    {"tag": "vless-in", "listen": "127.0.0.1", "port": 443, "protocol": "vless",
     "settings": {"clients": [{"id": "93a160a0-3c4e-437b-9c15-2cbc1b1a6a45"}], "decryption": "none"}},
    {"tag": "socks-in", "listen": "127.0.0.1", "port": 1080, "protocol": "socks", "settings": {},
     "sniffing": {"enabled": true, "destOverride": ["http", "tls"]}}
  ],
  "outbounds": [{"protocol": "freedom", "tag": "direct"}]
}`
	script := filepath.Join(dir, "route.lua")
	scriptBody := `function HandleRoute(metadata)
  return "block", "lua-rule"
end`
	scriptConfig := filepath.Join(dir, "script.json")
	scriptConfigBody := `{
  "routing": {"script": "` + script + `"},
  "outbounds": [{"protocol": "freedom", "tag": "direct"}, {"protocol": "blackhole", "tag": "block"}]
}`
	untagged := filepath.Join(dir, "untagged.json")
	untaggedBody := `{
  "outbounds": [{"protocol": "freedom", "settings": {"firewall": [{"action": "block", "port": "443"}]}}]
}`
	replace := filepath.Join(dir, "replace.json")
	replaceBody := `{
  "dns": {"hosts": {"a.test": "b.test"}},
  "outbounds": [{"protocol": "freedom", "tag": "direct"}]
}`
	list := filepath.Join(dir, "list.txt")
	listBody := "# targets\nwww.baidu.cn\n\n10.1.2.3\n"
	fixtures := map[string]string{
		config: body, balancer: balancerBody, final: finalBody, server: serverBody, list: listBody,
		script: scriptBody, scriptConfig: scriptConfigBody, untagged: untaggedBody, replace: replaceBody,
	}
	for path, payload := range fixtures {
		if err := os.WriteFile(path, []byte(payload), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	for _, tc := range []struct {
		name      string
		args      []string
		wants     []string
		wantsNone []string
		json      bool
		code      int
	}{
		{
			name: "domain rule",
			args: []string{"lookup", "-c", config, "www.baidu.cn"},
			wants: []string{
				"target:   www.baidu.cn:443/tcp",
				"#0 ruleTag=cn-sites outbound=direct domain=domain:cn",
				"outbound: direct",
				"dns rule: domain:cn(DNS idx:1)",
				"dns:      UDP:1.1.1.1:53 (tag=cn)",
			},
		},
		{
			name: "ip rule",
			args: []string{"lookup", "-c", config, "10.1.2.3"},
			wants: []string{
				"target:   10.1.2.3:443/tcp",
				"#1 outbound=block ip=10.0.0.0/8",
				"outbound: block",
			},
		},
		{
			name: "port and network",
			args: []string{"lookup", "-c", config, "-port", "53", "-network", "udp", "1.1.1.1"},
			wants: []string{
				"target:   1.1.1.1:53/udp",
				"port=53 network=udp",
				"outbound: dns-out",
			},
		},
		{
			name: "hosts lookup",
			args: []string{"lookup", "-c", config, "host.test"},
			wants: []string{
				"none (no rule matched)",
				"outbound: direct (default outbound)",
				"dns:      static hosts -> 203.0.113.9 (no upstream query)",
				"note:     rule #4 needs process/localOS",
			},
		},
		{
			name: "inbound condition",
			args: []string{"lookup", "-c", config, "-inbound", "socks", "host.test"},
			wants: []string{
				"#3 outbound=block inboundTag=socks",
				"outbound: block",
			},
		},
		{
			name: "resolve from hosts",
			args: []string{"lookup", "-c", config, "-resolve", "host.test"},
			wants: []string{
				"answers:  203.0.113.9",
			},
		},
		{
			name: "balancer",
			args: []string{"lookup", "-c", balancer, "www.baidu.cn"},
			wants: []string{
				"#0 balancer=relay domain=domain:cn",
				"balancer: relay (strategy=random)",
				"members:  hk1, hk2",
			},
		},
		{
			name: "freedom firewall rule by port",
			args: []string{"lookup", "-c", final, "-port", "25", "www.baidu.cn"},
			wants: []string{
				"final:    blocked by freedom firewall rule #0",
			},
		},
		{
			name: "freedom firewall rule by ip",
			args: []string{"lookup", "-c", final, "10.1.2.3"},
			wants: []string{
				"outbound: direct (default outbound)",
				"final:    blocked by freedom firewall rule #1",
			},
		},
		{
			name: "freedom firewall rules pass",
			args: []string{"lookup", "-c", final, "www.baidu.cn"},
			wants: []string{
				"final:    passes 2 freedom firewall",
			},
		},
		{
			name: "implicit firewall rule",
			args: []string{"lookup", "-c", server, "-inbound", "vless-in", "10.1.2.3"},
			wants: []string{
				"final:    blocked by the implicit firewall rule of a vless inbound (private IP)",
			},
		},
		{
			name: "implicit firewall rule hint",
			args: []string{"lookup", "-c", server, "10.1.2.3"},
			wants: []string{
				"note:     inbound vless-in brings an implicit firewall rule, pass -inbound vless-in to apply it",
			},
		},
		{
			name: "sniffing note",
			args: []string{"lookup", "-c", server, "-inbound", "socks-in", "www.example.com"},
			wants: []string{
				"note:     sniffing on socks-in overrides the target with 2 protocol(s) before routing",
			},
		},
		{
			name: "json",
			args: []string{"lookup", "-c", config, "-json", "www.baidu.cn"},
			wants: []string{
				`"target":"www.baidu.cn"`,
				`"tag":"cn-sites"`,
				`"outbound":"direct"`,
				`"servers":["UDP:1.1.1.1:53 (tag=cn)"`,
			},
		},
		{
			name: "file targets",
			args: []string{"lookup", "-c", config, "-f", list},
			wants: []string{
				"target:   www.baidu.cn:443/tcp",
				"target:   10.1.2.3:443/tcp",
			},
			wantsNone: []string{"target:   #", "target:   :"},
		},
		{
			name: "json batch",
			args: []string{"lookup", "-c", config, "-json", "-f", list},
			json: true,
		},
		{
			name: "routing script",
			args: []string{"lookup", "-c", scriptConfig, "www.example.com"},
			wants: []string{
				"rule:     #-1 ruleTag=lua-rule",
				"outbound: block",
			},
		},
		{
			name: "untagged outbound",
			args: []string{"lookup", "-c", untagged, "www.example.com"},
			wants: []string{
				"outbound: <untagged> (default outbound)",
				"final:    blocked by freedom firewall rule #0",
			},
		},
		{
			name: "hosts replacement",
			args: []string{"lookup", "-c", replace, "a.test"},
			wants: []string{
				"dns:      static hosts replaced the domain with b.test",
			},
			wantsNone: []string{"no upstream query"},
		},
		{
			name: "bad attribute",
			args: []string{"lookup", "-c", config, "-attribute", "noequals", "www.baidu.cn"},
			code: 2,
		},
		{
			name: "unknown inbound",
			args: []string{"lookup", "-c", config, "-inbound", "nope", "www.baidu.cn"},
			code: 2,
		},
		{
			name: "bad port",
			args: []string{"lookup", "-c", config, "-port", "70000", "www.baidu.cn"},
			code: 2,
		},
		{
			name: "file and arguments",
			args: []string{"lookup", "-c", config, "-f", list, "www.baidu.cn"},
			code: 2,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			args := append([]string{"-test.run=^TestLookupCommand$", "--"}, tc.args...)
			cmd := exec.CommandContext(ctx, os.Args[0], args...)
			cmd.Env = append(os.Environ(), "XRAY_TEST_LOOKUP_COMMAND=1")
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			err := cmd.Run()
			if cmd.ProcessState == nil {
				t.Fatal(err)
			}
			output := stdout.String() + stderr.String()
			if code := cmd.ProcessState.ExitCode(); code != tc.code {
				t.Fatalf("exit code = %d, want %d: %v\n%s", code, tc.code, err, output)
			}
			for _, want := range tc.wants {
				if !strings.Contains(output, want) {
					t.Errorf("output missing %q:\n%s", want, output)
				}
			}
			for _, unwanted := range tc.wantsNone {
				if strings.Contains(output, unwanted) {
					t.Errorf("output contains %q:\n%s", unwanted, output)
				}
			}
			if tc.json {
				// Logs must not pollute stdout, and every line must be JSON.
				lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
				if len(lines) != 2 {
					t.Errorf("got %d json lines, want 2:\n%s", len(lines), stdout.String())
				}
				for _, line := range lines {
					var result map[string]any
					if err := json.Unmarshal([]byte(line), &result); err != nil {
						t.Errorf("stdout line is not json: %v\n%s", err, line)
					}
				}
			}
		})
	}
}
