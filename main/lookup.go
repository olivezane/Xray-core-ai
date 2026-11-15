package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	gonet "net"
	"os"
	"sort"
	"strconv"
	"strings"

	appdns "github.com/xtls/xray-core/app/dns"
	"github.com/xtls/xray-core/app/proxyman"
	"github.com/xtls/xray-core/app/router"
	routercommand "github.com/xtls/xray-core/app/router/command"
	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/cmdarg"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/geodata"
	"github.com/xtls/xray-core/common/log"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/features/dns"
	"github.com/xtls/xray-core/features/outbound"
	"github.com/xtls/xray-core/features/routing"
	"github.com/xtls/xray-core/main/commands/base"
	"github.com/xtls/xray-core/proxy/freedom"
	"github.com/xtls/xray-core/transport/internet"
)

var cmdLookup = &base.Command{
	UsageLine: `{{.Exec}} lookup [-c config.json] [-port 443] [-network tcp] <domain or ip>`,
	Short:     `Show the routing rule, outbound and DNS a domain or IP would use`,
	Long: `
Show which routing rule matches a domain or IP in the given config, which
outbound it goes to, and which DNS would resolve it.

The config is loaded the same way as "{{.Exec}} run", so -c, -confdir and
-format all work. Configs are not started, no inbound is opened.

Conditions that come from the live connection (inbound tag, sniffed protocol,
user, source and local IP) are empty unless given with the matching flags, so
rules relying on them never match.

Examples:
  {{.Exec}} lookup -c config.json example.com
  {{.Exec}} lookup -c config.json -port 80 1.1.1.1
  {{.Exec}} lookup -c config.json -network udp example.com:53
  {{.Exec}} lookup -c config.json -inbound socks -protocol http example.com
  {{.Exec}} lookup -c config.json -resolve example.com
  {{.Exec}} lookup -c config.json -json -f domains.txt
`,
}

var (
	lookupPort     = cmdLookup.Flag.Int("port", 443, "Target port.")
	lookupNetwork  = cmdLookup.Flag.String("network", "tcp", "Target network: tcp or udp.")
	lookupInbound  = cmdLookup.Flag.String("inbound", "", "Inbound tag the connection comes from.")
	lookupProtocol = cmdLookup.Flag.String("protocol", "", "Sniffed protocol of the connection, e.g. http.")
	lookupUser     = cmdLookup.Flag.String("user", "", "User email of the connection.")
	lookupSource   = cmdLookup.Flag.String("source", "", "Source IP of the connection.")
	lookupLocal    = cmdLookup.Flag.String("local", "", "Local IP of the connection.")
	lookupAttrs    cmdarg.Arg
	lookupJSON     = cmdLookup.Flag.Bool("json", false, "Print one JSON object per target.")
	lookupResolve  = cmdLookup.Flag.Bool("resolve", false, "Also resolve the domain with the config's DNS.")
	lookupFile     = cmdLookup.Flag.String("f", "", "Read targets from a file, one per line.")
)

func init() {
	cmdLookup.Flag.Var(&configFiles, "config", "Config path for Xray.")
	cmdLookup.Flag.Var(&configFiles, "c", "Short alias of -config")
	cmdLookup.Flag.StringVar(&configDir, "confdir", "", "A dir with multiple json config")
	cmdLookup.Flag.StringVar(format, "format", "auto", "Format of input file.")
	cmdLookup.Flag.Var(&lookupAttrs, "attribute", "Extra connection attribute k=v, repeatable.")
	cmdLookup.Run = executeLookup // break init loop
}

type lookupResult struct {
	Target        string        `json:"target"`
	Port          int           `json:"port"`
	Network       string        `json:"network"`
	Rule          *lookupRule   `json:"rule"`
	Balancer      *lookupBalanc `json:"balancer,omitempty"`
	Outbound      string        `json:"outbound,omitempty"`
	OutboundState string        `json:"outboundState,omitempty"`
	Resolved      []string      `json:"resolved,omitempty"`
	Answers       []string      `json:"answers,omitempty"`
	AnswerErr     string        `json:"answerError,omitempty"`
	Final         *lookupFinal  `json:"finalRule,omitempty"`
	DNS           *lookupDNS    `json:"dns,omitempty"`
	PickError     string        `json:"pickError,omitempty"`
	Notes         []string      `json:"notes,omitempty"`
}

type lookupRule struct {
	Index int    `json:"index"`
	Tag   string `json:"tag,omitempty"`
	Match string `json:"match,omitempty"`
}

type lookupBalanc struct {
	Tag      string   `json:"tag"`
	Strategy string   `json:"strategy"`
	Fallback string   `json:"fallback,omitempty"`
	Members  []string `json:"members,omitempty"`
}

type lookupFinal struct {
	Index   int    `json:"index"`
	Block   bool   `json:"block"`
	Total   int    `json:"total"`
	Default bool   `json:"implicit,omitempty"`
	Inbound string `json:"inbound,omitempty"`
}

type lookupDNS struct {
	Rules         []string `json:"rules,omitempty"`
	Hosts         []string `json:"hosts,omitempty"`
	Servers       []string `json:"servers,omitempty"`
	QueryStrategy string   `json:"queryStrategy,omitempty"`
	FakeIP        bool     `json:"fakeip,omitempty"`
}

func executeLookup(cmd *base.Command, args []string) {
	targets, err := lookupTargets(args)
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}
	network := net.Network_TCP
	switch strings.ToLower(*lookupNetwork) {
	case "", "tcp":
	case "udp":
		network = net.Network_UDP
	default:
		fmt.Println("Unsupported network:", *lookupNetwork)
		os.Exit(2)
	}
	if *lookupPort < 1 || *lookupPort > 65535 {
		fmt.Println("Illegal port:", *lookupPort)
		os.Exit(2)
	}
	for name, value := range map[string]string{"-source": *lookupSource, "-local": *lookupLocal} {
		if value != "" && net.ParseIP(value) == nil {
			fmt.Println("Illegal "+name+" IP:", value)
			os.Exit(2)
		}
	}
	attrs, err := lookupAttributes()
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}
	if *lookupJSON {
		// Keep stdout parseable: config loading and DNS decisions log to stderr.
		log.RegisterHandler(log.NewLogger(log.CreateStderrLogWriter()))
	}

	c, err := core.LoadConfig(getConfigFormat(), getConfigFilePath(true))
	if err != nil {
		fmt.Println("Failed to load config:", err)
		os.Exit(1)
	}
	inst, err := core.New(c)
	if err != nil {
		fmt.Println("Failed to create instance:", err)
		os.Exit(1)
	}
	defer inst.Close()
	if *lookupInbound != "" {
		found := false
		for _, in := range c.Inbound {
			if in.GetTag() == *lookupInbound {
				found = true
				break
			}
		}
		if !found {
			inst.Close()
			fmt.Println("Unknown inbound tag:", *lookupInbound)
			os.Exit(2)
		}
	}
	if rc := routerConfig(c); rc != nil && rc.GetScript() != "" {
		// Router.Start only compiles the routing script, no listener is opened.
		r, _ := inst.GetFeature(routing.RouterType()).(*router.Router)
		if r == nil {
			inst.Close()
			fmt.Println("Failed to initialize routing script: no router")
			os.Exit(1)
		}
		if err := r.Start(); err != nil {
			inst.Close()
			fmt.Println("Failed to initialize routing script:", err)
			os.Exit(1)
		}
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	for i, target := range targets {
		result := lookup(inst, c, target, network, attrs)
		if *lookupJSON {
			if err := encoder.Encode(result); err != nil {
				fmt.Println("Failed to write json:", err)
				os.Exit(1)
			}
			continue
		}
		if i > 0 {
			fmt.Println()
		}
		result.print()
	}
}

func lookupTargets(args []string) ([]string, error) {
	switch {
	case *lookupFile != "" && len(args) > 0:
		return nil, errors.New("-f and target arguments are mutually exclusive")
	case *lookupFile != "":
		file, err := os.Open(*lookupFile)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		var targets []string
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			if line := strings.TrimSpace(scanner.Text()); line != "" && !strings.HasPrefix(line, "#") {
				targets = append(targets, line)
			}
		}
		if err := scanner.Err(); err != nil {
			return nil, err
		}
		if len(targets) == 0 {
			return nil, errors.New("no target in " + *lookupFile)
		}
		return targets, nil
	case len(args) == 1:
		return args, nil
	default:
		return nil, errors.New("exactly one target or -f is required")
	}
}

func lookupAttributes() (map[string]string, error) {
	if len(lookupAttrs) == 0 {
		return nil, nil
	}
	attrs := make(map[string]string, len(lookupAttrs))
	for _, attr := range lookupAttrs {
		k, v, ok := strings.Cut(attr, "=")
		if !ok {
			return nil, errors.New("illegal attribute: ", attr)
		}
		attrs[k] = v
	}
	return attrs, nil
}

func lookup(inst *core.Instance, c *core.Config, target string, network net.Network, attrs map[string]string) *lookupResult {
	result := &lookupResult{Target: target, Port: *lookupPort, Network: strings.ToLower(network.String())}
	// Allow "host:port" as target, overriding -port.
	if host, port, err := gonet.SplitHostPort(target); err == nil {
		if n, err := strconv.Atoi(port); err == nil && n > 0 && n <= 65535 {
			result.Target, result.Port = host, n
		}
	}
	// Accept a bracketed IPv6 literal as typed on the command line.
	result.Target = strings.TrimSuffix(strings.TrimPrefix(result.Target, "["), "]")

	ctx := &routercommand.RoutingContext{
		TargetPort: uint32(result.Port),
		Network:    network,
		InboundTag: *lookupInbound,
		Protocol:   *lookupProtocol,
		User:       *lookupUser,
		Attributes: attrs,
	}
	if ip := net.ParseIP(*lookupSource); ip != nil {
		ctx.SourceIPs = [][]byte{ip}
	}
	if ip := net.ParseIP(*lookupLocal); ip != nil {
		ctx.LocalIPs = [][]byte{ip}
	}
	if ip := net.ParseIP(result.Target); ip != nil {
		ctx.TargetIPs = [][]byte{ip}
	} else {
		ctx.TargetDomain = result.Target
	}

	route, err := pickRoute(inst, ctx)
	switch {
	case err != nil:
		result.PickError = err.Error()
	case route == nil:
		// No rule matched: the dispatcher falls back to the default outbound.
	default:
		result.fillRule(c, route)
	}
	result.fillOutbound(inst, c, route)
	// freedom may still drop the connection after routing chose it.
	if result.OutboundState != "" || result.Outbound != "" {
		result.fillFinal(c)
	}
	if ctx.TargetDomain != "" {
		result.fillDNS(inst, c, ctx.TargetDomain)
	}
	if rc := routerConfig(c); rc != nil {
		if rc.GetScript() != "" {
			result.Notes = append(result.Notes, "routing.script decides the outbound, the matched rule may be bypassed")
		}
		for _, i := range unverifiableRules(rc) {
			result.Notes = append(result.Notes, fmt.Sprintf("rule #%d needs process/localOS/sourcePort/localPort/vlessRoute, not simulated by lookup", i))
		}
	}
	for _, note := range sniffingNotes(c) {
		result.Notes = append(result.Notes, note)
	}
	return result
}

func (r *lookupResult) fillRule(c *core.Config, route routing.Route) {
	r.Rule = &lookupRule{Index: -1}
	if rr, ok := route.(*router.Route); ok {
		r.Rule.Index = rr.GetRuleIndex()
	}
	r.Rule.Tag = route.GetRuleTag()
	if rc := routerConfig(c); rc != nil && r.Rule.Index >= 0 && r.Rule.Index < len(rc.Rule) {
		r.Rule.Match = describeRule(rc.Rule[r.Rule.Index])
		if tag := rc.Rule[r.Rule.Index].GetBalancingTag(); tag != "" {
			r.Balancer = balancingRule(c, tag)
		}
	}
}

func (r *lookupResult) fillOutbound(inst *core.Instance, c *core.Config, route routing.Route) {
	ohm, _ := inst.GetFeature(outbound.ManagerType()).(outbound.Manager)
	if b := r.Balancer; b != nil {
		if hs, ok := ohm.(outbound.HandlerSelector); ok {
			if tags := hs.Select(b.Members); len(tags) > 0 {
				b.Members = tags
			}
		}
	}
	if route == nil {
		var h outbound.Handler
		if ohm != nil {
			h = ohm.GetDefaultHandler()
		}
		if h == nil {
			r.OutboundState = "no default outbound, connection closed"
			return
		}
		r.Outbound, r.OutboundState = h.Tag(), "default outbound"
		return
	}
	r.Outbound = route.GetOutboundTag()
	// A rule whose outbound tag does not exist closes the connection instead
	// of falling back to the default outbound.
	if r.Outbound != "" && ohm != nil && ohm.GetHandler(r.Outbound) == nil {
		r.OutboundState = "tag not found, connection closed"
	}
	if ips := route.GetTargetIPs(); len(ips) > 0 && route.GetTargetDomain() != "" {
		for _, ip := range ips {
			r.Resolved = append(r.Resolved, ip.String())
		}
	}
}

// senderConfig returns the outbound's stream settings, nil when it has none.
func senderConfig(ob *core.OutboundHandlerConfig) *proxyman.SenderConfig {
	settings := ob.GetSenderSettings()
	if settings == nil {
		return nil
	}
	inst, err := settings.GetInstance()
	if err != nil {
		return nil
	}
	sc, _ := inst.(*proxyman.SenderConfig)
	return sc
}

// allIPs reports whether every entry parses as an IP address.
func allIPs(values []string) bool {
	for _, v := range values {
		if net.ParseIP(v) == nil {
			return false
		}
	}
	return true
}

func (r *lookupResult) fillFinal(c *core.Config) {
	for _, ob := range c.Outbound {
		if ob.GetTag() != r.Outbound {
			continue
		}
		if ob.GetProxySettings() == nil {
			return
		}
		inst, err := ob.GetProxySettings().GetInstance()
		if err != nil {
			return
		}
		fc, ok := inst.(*freedom.Config)
		if !ok {
			return
		}
		domainStrategy := fc.GetDomainStrategy()
		if sc := senderConfig(ob); sc != nil {
			if sc.GetStreamSettings().GetSocketSettings().GetDialerProxy() != "" {
				r.Notes = append(r.Notes, "freedom is not the final outbound (dialerProxy), finalRules are ignored")
				return
			}
			// The runtime prefers the socket settings over the legacy alias.
			if ds := sc.GetStreamSettings().GetSocketSettings().GetDomainStrategy(); ds != internet.DomainStrategy_AS_IS {
				domainStrategy = ds
			}
		}
		network := net.Network_TCP
		if r.Network == "udp" {
			network = net.Network_UDP
		}
		address := net.ParseAddress(r.Target)
		inbound := inboundProtocol(c, *lookupInbound)
		if *lookupInbound == "" {
			for _, in := range c.Inbound {
				if kind := inboundProtocol(c, in.GetTag()); kind != "" && freedom.HasImplicitFinalRule(kind) {
					r.Notes = append(r.Notes, fmt.Sprintf("inbound %s brings an implicit final rule, pass -inbound %s to apply it", in.GetTag(), in.GetTag()))
					break
				}
			}
		}
		verdict := freedom.FinalRuleVerdict(fc, inbound, network, address, net.Port(r.Port))
		if verdict.Total > 0 || verdict.Default {
			r.Final = &lookupFinal{Index: verdict.Index, Block: verdict.Block, Total: verdict.Total, Default: verdict.Default, Inbound: inbound}
		}
		if address.Family().IsDomain() && domainStrategy != internet.DomainStrategy_AS_IS {
			for _, rc := range fc.GetFinalRules() {
				if len(rc.GetIp()) > 0 {
					r.Notes = append(r.Notes, "freedom resolves the domain before matching ip finalRules, so ip rules above are not applied here")
					break
				}
			}
		}
		return
	}
}

func (r *lookupResult) fillDNS(inst *core.Instance, c *core.Config, domain string) {
	client, ok := inst.GetFeature(dns.ClientType()).(*appdns.DNS)
	if !ok {
		r.DNS = &lookupDNS{Servers: []string{"system resolver (no dns config)"}}
		return
	}
	hosts, rules, nameServers := client.Explain(domain)
	r.DNS = &lookupDNS{Rules: rules, Servers: nameServers}
	for _, host := range hosts {
		r.DNS.Hosts = append(r.DNS.Hosts, host.String())
	}
	for _, name := range nameServers {
		if strings.Contains(name, "Fake") {
			r.DNS.FakeIP = true
		}
	}
	dc := dnsConfig(c)
	if dc != nil && dc.GetQueryStrategy() != appdns.QueryStrategy_USE_IP {
		r.DNS.QueryStrategy = dc.GetQueryStrategy().String()
	}
	if dc != nil && dc.GetScript() != "" {
		r.Notes = append(r.Notes, "dns.script picks the servers, the list above may not apply")
	}
	if *lookupResolve {
		r.fillAnswers(client, domain)
	}
}

// fillAnswers queries the configured DNS, unlike the rule/IP resolution above
// which only runs when domainStrategy asks for IPs.
func (r *lookupResult) fillAnswers(client *appdns.DNS, domain string) {
	// Both families requested: DNS.LookupIP narrows them by the config's
	// queryStrategy itself.
	ips, _, err := client.LookupIP(domain, dns.IPOption{IPv4Enable: true, IPv6Enable: true})
	if err != nil {
		r.AnswerErr = err.Error()
		return
	}
	for _, ip := range ips {
		r.Answers = append(r.Answers, ip.String())
	}
	if len(r.Answers) == 0 {
		r.AnswerErr = "empty response"
	}
}

// inboundProtocol returns the protocol of an inbound tag, e.g. "vless".
// shortcut: the protocol comes from the proxy settings type name, so a vless
// inbound in reverse mode is reported as plain vless; upgrade if reverse mode
// ever needs its own block-everything verdict.
func inboundProtocol(c *core.Config, tag string) string {
	for _, in := range c.Inbound {
		if in.GetTag() != tag || in.GetProxySettings() == nil {
			continue
		}
		kind := strings.ToLower(in.GetProxySettings().GetType())
		for _, name := range []string{"vless", "vmess", "trojan", "hysteria", "wireguard", "masque", "shadowsocks"} {
			if strings.Contains(kind, name) {
				return name
			}
		}
	}
	return ""
}

// sniffingNotes warns that an inbound rewrites the target before routing.
func sniffingNotes(c *core.Config) []string {
	var notes []string
	for _, in := range c.Inbound {
		if in.GetTag() != *lookupInbound || in.GetReceiverSettings() == nil {
			continue
		}
		rs, err := in.GetReceiverSettings().GetInstance()
		if err != nil {
			continue
		}
		sc, ok := rs.(*proxyman.ReceiverConfig)
		if !ok || !sc.GetSniffingSettings().GetEnabled() {
			continue
		}
		if n := len(sc.GetSniffingSettings().GetDestinationOverride()); n > 0 {
			notes = append(notes, fmt.Sprintf("sniffing on %s overrides the target with %d protocol(s) before routing", in.GetTag(), n))
		}
	}
	return notes
}

// unverifiableRules lists rules depending on conditions lookup cannot provide.
func unverifiableRules(rc *router.Config) []int {
	var indices []int
	for i, rule := range rc.Rule {
		if rule.GetProcess() != nil || rule.GetLocalOs() != nil || rule.GetSourcePortList() != nil ||
			rule.GetLocalPortList() != nil || rule.GetVlessRouteList() != nil {
			indices = append(indices, i)
		}
	}
	return indices
}

func (r *lookupResult) print() {
	fmt.Printf("target:   %s/%s\n", gonet.JoinHostPort(r.Target, strconv.Itoa(r.Port)), r.Network)
	switch {
	case r.PickError != "":
		fmt.Println("rule:     pick failed: " + r.PickError)
	case r.Rule != nil:
		line := fmt.Sprintf("rule:     #%d", r.Rule.Index)
		if r.Rule.Tag != "" {
			line += " ruleTag=" + r.Rule.Tag
		}
		if r.Rule.Match != "" {
			line += " " + r.Rule.Match
		}
		fmt.Println(line)
	default:
		fmt.Println("rule:     none (no rule matched)")
	}
	if b := r.Balancer; b != nil {
		line := fmt.Sprintf("balancer: %s (strategy=%s", b.Tag, b.Strategy)
		if b.Fallback != "" {
			line += ", fallback=" + b.Fallback
		}
		fmt.Println(line + ")")
		fmt.Println("members:  " + strings.Join(b.Members, ", "))
	}
	if r.Outbound != "" || r.OutboundState != "" {
		tag := r.Outbound
		if tag == "" {
			tag = "<untagged>"
		}
		if r.OutboundState != "" {
			fmt.Printf("outbound: %s (%s)\n", tag, r.OutboundState)
		} else {
			fmt.Println("outbound: " + tag)
		}
	}
	if len(r.Resolved) > 0 {
		fmt.Println("resolved: " + strings.Join(r.Resolved, ", "))
	}
	if f := r.Final; f != nil {
		switch {
		case f.Default && f.Inbound == "vless-reverse":
			fmt.Println("final:    blocked by the implicit final rule of a vless-reverse inbound (all destinations)")
		case f.Default:
			fmt.Printf("final:    blocked by the implicit final rule of a %s inbound (private IP)\n", f.Inbound)
		case f.Index < 0:
			fmt.Printf("final:    passes %d freedom finalRules\n", f.Total)
		case f.Block:
			fmt.Printf("final:    blocked by freedom finalRule #%d\n", f.Index)
		default:
			fmt.Printf("final:    allowed by freedom finalRule #%d\n", f.Index)
		}
	}
	if d := r.DNS; d != nil {
		if len(d.Rules) > 0 {
			fmt.Println("dns rule: " + strings.Join(d.Rules, ", "))
		}
		switch {
		case len(d.Hosts) > 0 && allIPs(d.Hosts):
			fmt.Println("dns:      static hosts -> " + strings.Join(d.Hosts, ", ") + " (no upstream query)")
		case len(d.Hosts) > 0:
			fmt.Println("dns:      static hosts replaced the domain with " + strings.Join(d.Hosts, ", "))
		default:
			line := "dns:      " + strings.Join(d.Servers, ", ")
			if d.QueryStrategy != "" {
				line += " (" + d.QueryStrategy + ")"
			}
			fmt.Println(line)
		}
		if d.FakeIP {
			fmt.Println("note:     fakeip answers take part in IP-based routing too")
		}
	}
	switch {
	case r.AnswerErr != "":
		fmt.Println("answers:  failed: " + r.AnswerErr)
	case len(r.Answers) > 0:
		fmt.Println("answers:  " + strings.Join(r.Answers, ", "))
	}
	for _, note := range r.Notes {
		fmt.Println("note:     " + note)
	}
}

func pickRoute(inst *core.Instance, ctx *routercommand.RoutingContext) (routing.Route, error) {
	r, ok := inst.GetFeature(routing.RouterType()).(routing.Router)
	if !ok {
		return nil, nil
	}
	route, err := r.PickRoute(routercommand.AsRoutingContext(ctx))
	if errors.Cause(err) == common.ErrNoClue {
		return nil, nil
	}
	return route, err
}

// describeRule renders a routing rule as key=value pairs.
func describeRule(r *router.RoutingRule) string {
	var kv []string
	add := func(k, v string) {
		if v != "" {
			kv = append(kv, k+"="+v)
		}
	}
	add("outbound", r.GetTag())
	add("balancer", r.GetBalancingTag())
	add("domain", describeAll(r.GetDomain(), geodata.FormatDomainRule))
	add("ip", describeAll(r.GetIp(), geodata.FormatIPRule))
	add("sourceIp", describeAll(r.GetSourceIp(), geodata.FormatIPRule))
	add("localIp", describeAll(r.GetLocalIp(), geodata.FormatIPRule))
	add("port", describePortList(r.GetPortList()))
	add("sourcePort", describePortList(r.GetSourcePortList()))
	add("localPort", describePortList(r.GetLocalPortList()))
	add("vlessRoute", describePortList(r.GetVlessRouteList()))
	add("network", describeAll(r.GetNetworks(), func(n net.Network) string { return strings.ToLower(n.String()) }))
	add("inboundTag", strings.Join(r.GetInboundTag(), ","))
	add("protocol", strings.Join(r.GetProtocol(), ","))
	add("user", strings.Join(r.GetUserEmail(), ","))
	add("process", strings.Join(r.GetProcess(), ","))
	add("localOS", strings.Join(r.GetLocalOs(), ","))
	if r.GetWebhook() != nil {
		kv = append(kv, "webhook")
	}
	if attrs := r.GetAttributes(); len(attrs) > 0 {
		pairs := make([]string, 0, len(attrs))
		for k, v := range attrs {
			pairs = append(pairs, k+"="+v)
		}
		sort.Strings(pairs)
		add("attrs", strings.Join(pairs, ";"))
	}
	return strings.Join(kv, " ")
}

func describeAll[T any](items []T, format func(T) string) string {
	ss := make([]string, len(items))
	for i, item := range items {
		ss[i] = format(item)
	}
	return strings.Join(ss, ",")
}

func describePortList(l *net.PortList) string {
	ss := make([]string, 0, len(l.GetRange()))
	for _, r := range l.GetRange() {
		if r.GetFrom() == r.GetTo() {
			ss = append(ss, strconv.Itoa(int(r.GetFrom())))
		} else {
			ss = append(ss, strconv.Itoa(int(r.GetFrom()))+"-"+strconv.Itoa(int(r.GetTo())))
		}
	}
	return strings.Join(ss, ",")
}

func balancingRule(c *core.Config, tag string) *lookupBalanc {
	rc := routerConfig(c)
	if rc == nil {
		return nil
	}
	for _, br := range rc.BalancingRule {
		if br.GetTag() != tag {
			continue
		}
		strategy := br.GetStrategy()
		if strategy == "" {
			strategy = "random" // same default as app/router BalancingRule.Build
		}
		return &lookupBalanc{
			Tag:      tag,
			Strategy: strategy,
			Fallback: br.GetFallbackTag(),
			Members:  br.GetOutboundSelector(),
		}
	}
	return nil
}

func routerConfig(c *core.Config) *router.Config {
	for _, app := range c.App {
		if inst, err := app.GetInstance(); err == nil {
			if rc, ok := inst.(*router.Config); ok {
				return rc
			}
		}
	}
	return nil
}

func dnsConfig(c *core.Config) *appdns.Config {
	for _, app := range c.App {
		if inst, err := app.GetInstance(); err == nil {
			if dc, ok := inst.(*appdns.Config); ok {
				return dc
			}
		}
	}
	return nil
}
