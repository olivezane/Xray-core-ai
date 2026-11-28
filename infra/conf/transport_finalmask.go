package conf

import (
	"sync"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/netip"
	"net/url"
	"os"
	"regexp"
	"strings"

	googleuuid "github.com/google/uuid"
	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/geodata"
	xlua "github.com/xtls/xray-core/common/lua"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/serial"
	coreconfig "github.com/xtls/xray-core/core/config"
	"github.com/xtls/xray-core/transport/internet"
	"github.com/xtls/xray-core/transport/internet/finalmask/filter"
	"github.com/xtls/xray-core/transport/internet/finalmask/fragment"
	"github.com/xtls/xray-core/transport/internet/finalmask/header/custom"
	"github.com/xtls/xray-core/transport/internet/finalmask/mkcp/aes128gcm"
	"github.com/xtls/xray-core/transport/internet/finalmask/mkcp/header"
	"github.com/xtls/xray-core/transport/internet/finalmask/mkcp/original"
	"github.com/xtls/xray-core/transport/internet/finalmask/noise"
	"github.com/xtls/xray-core/transport/internet/finalmask/proxyprotocol"
	"github.com/xtls/xray-core/transport/internet/finalmask/realm"
	"github.com/xtls/xray-core/transport/internet/finalmask/salamander"
	"github.com/xtls/xray-core/transport/internet/finalmask/sudoku"
	"github.com/xtls/xray-core/transport/internet/finalmask/udphop"
	"github.com/xtls/xray-core/transport/internet/finalmask/xdns"
	"github.com/xtls/xray-core/transport/internet/finalmask/xicmp"
	"github.com/xtls/xray-core/transport/internet/finalmask/xmc"
	"github.com/xtls/xray-core/transport/internet/hysteria/congestion/bbr"
	"github.com/xtls/xray-core/transport/internet/tls"
)

func PraseByteSlice(data json.RawMessage, typ string) ([]byte, error) {
	switch strings.ToLower(typ) {
	case "", "array":
		if len(data) == 0 {
			return data, nil
		}
		var packet []byte
		if err := json.Unmarshal(data, &packet); err != nil {
			return nil, err
		}
		return packet, nil
	case "str":
		var str string
		if err := json.Unmarshal(data, &str); err != nil {
			return nil, err
		}
		return []byte(str), nil
	case "hex":
		var str string
		if err := json.Unmarshal(data, &str); err != nil {
			return nil, err
		}
		return hex.DecodeString(str)
	case "base64":
		var str string
		if err := json.Unmarshal(data, &str); err != nil {
			return nil, err
		}
		return base64.StdEncoding.DecodeString(str)
	default:
		return nil, errors.New("unknown type")
	}
}

var (
	customVarNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

	tcpmaskLoader = NewJSONConfigLoader(ConfigCreatorCache{
		"filter":         func() interface{} { return new(Filter) },
		"header-custom":  func() interface{} { return new(HeaderCustomTCP) },
		"proxy-protocol": func() interface{} { return new(ProxyProtocol) },
		"fragment":       func() interface{} { return new(FragmentMask) },
		"sudoku":         func() interface{} { return new(Sudoku) },
		"xmc":            func() interface{} { return new(XMC) },
	}, "type", "settings")

	udpmaskLoader = NewJSONConfigLoader(ConfigCreatorCache{
		"filter":        func() interface{} { return new(Filter) },
		"header-custom": func() interface{} { return new(HeaderCustomUDP) },
		"mkcp-legacy":   func() interface{} { return new(MkcpLegacy) },
		"noise":         func() interface{} { return new(NoiseMask) },
		"salamander":    func() interface{} { return new(Salamander) },
		"sudoku":        func() interface{} { return new(Sudoku) },
		"xdns":          func() interface{} { return new(XDNS) },
		"xicmp":         func() interface{} { return new(Xicmp) },
		"realm":         func() interface{} { return new(Realm) },
		"udphop":        func() interface{} { return new(UDPHop) },
	}, "type", "settings")
)

// ProxyProtocol is the PROXY protocol header setting.
type ProxyProtocol struct {
	coreconfig.ProxyProtocol
}

// Build converts the setting to its message.
func (c *ProxyProtocol) Build() (any, error) {
	return buildProxyProtocol(&c.ProxyProtocol)
}

var maskSettingsTypesOnce sync.Once

// installMaskSettingsTypes makes the masks whose settings are a plain Go value
// readable by name. Building a mask needs it, so it is done here rather than
// only where a whole build is composed.
func installMaskSettingsTypes() {
	maskSettingsTypesOnce.Do(func() {
		common.Must(common.SetSettingsTypes(MaskSettingsTypes()))
	})
}

// MaskSettingsTypes is every mask whose settings are a plain Go value: a mask
// creates its settings itself, so the name each one crosses the API by is listed
// here rather than in the configuration table.
func MaskSettingsTypes() []common.SettingsType {
	return []common.SettingsType{
		{Config: (*filter.Config)(nil)},
		{Config: (*proxyprotocol.Config)(nil)},
		{Config: (*sudoku.Config)(nil)},
		{Config: (*xicmp.Config)(nil)},
		{Config: (*original.Config)(nil)},
		{Config: (*aes128gcm.Config)(nil)},
		{Config: (*header.Config)(nil)},
	}
}

func buildProxyProtocol(c *coreconfig.ProxyProtocol) (any, error) {
	version := c.Version
	if version == 0 {
		version = 2
	}
	if version != 1 && version != 2 {
		return nil, errors.New("PROXY protocol version must be 1 or 2")
	}
	return &proxyprotocol.Config{Version: version}, nil
}

func addProxyProtocolMask(config *internet.StreamConfig, version uint32) {
	mask := &proxyprotocol.Config{Version: version}
	for _, existing := range config.Tcpmasks {
		if serial.NameOf(existing) == serial.NameOf(mask) {
			return
		}
	}
	// Put the header directly on the socket, before the other masks.
	config.Tcpmasks = append(config.Tcpmasks, mask)
}

// Filter blocks or allows source addresses.
type Filter struct {
	coreconfig.Filter
}

// Build converts the filter to its message.
func (c *Filter) Build() (any, error) {
	return buildFilter(&c.Filter)
}

func buildFilter(c *coreconfig.Filter) (any, error) {
	rules, err := geodata.ParseIPRules(c.BlockedSourceIPs)
	if err != nil {
		return nil, errors.New(`invalid "blockedSourceIPs"`).Base(err)
	}
	allowed, err := geodata.ParseIPRules(c.AllowedSourceIPs)
	if err != nil {
		return nil, errors.New(`invalid "allowedSourceIPs"`).Base(err)
	}
	return &filter.Config{BlockedSourceIps: rules, AllowedSourceIps: allowed}, nil
}

// TCPItem is one item of the header-custom mask for TCP, from the model.
type TCPItem = coreconfig.TCPItem

// HeaderCustomTCP is the header-custom mask setting for TCP.
type HeaderCustomTCP struct {
	coreconfig.HeaderCustomTCP
}

// Build converts the mask to its message.
func (c *HeaderCustomTCP) Build() (any, error) {
	return buildHeaderCustomTCP(&c.HeaderCustomTCP)
}

func buildHeaderCustomTCP(c *coreconfig.HeaderCustomTCP) (any, error) {
	for _, value := range c.Clients {
		for _, item := range value {
			if err := validateCustomItemSpec(item.Capture, item.Packet, item.Rand, item.Reuse, item.Transform); err != nil {
				return nil, err
			}
		}
	}
	for _, value := range c.Servers {
		for _, item := range value {
			if err := validateCustomItemSpec(item.Capture, item.Packet, item.Rand, item.Reuse, item.Transform); err != nil {
				return nil, err
			}
		}
	}
	for _, value := range c.Errors {
		for _, item := range value {
			if err := validateCustomItemSpec(item.Capture, item.Packet, item.Rand, item.Reuse, item.Transform); err != nil {
				return nil, err
			}
		}
	}

	errInvalidRange := errors.New("invalid randRange")

	clients := make([]*custom.TCPSequence, len(c.Clients))
	for i, value := range c.Clients {
		clients[i] = &custom.TCPSequence{}
		for _, item := range value {
			if item.RandRange == nil {
				item.RandRange = &Int32Range{From: 0, To: 255}
			}
			if item.RandRange.From < 0 || item.RandRange.To > 255 {
				return nil, errInvalidRange
			}
			var err error
			if item.Packet, err = PraseByteSlice(item.Packet, item.Type); err != nil {
				return nil, err
			}
			transform, err := buildCustomTransform(item.Transform)
			if err != nil {
				return nil, err
			}
			clients[i].Sequence = append(clients[i].Sequence, &custom.TCPItem{
				DelayMin: int64(item.Delay.From),
				DelayMax: int64(item.Delay.To),
				Rand:     item.Rand,
				RandMin:  item.RandRange.From,
				RandMax:  item.RandRange.To,
				Packet:   item.Packet,
				Save:     item.Capture,
				Var:      item.Reuse,
				Expr:     transform,
			})
		}
	}

	servers := make([]*custom.TCPSequence, len(c.Servers))
	for i, value := range c.Servers {
		servers[i] = &custom.TCPSequence{}
		for _, item := range value {
			if item.RandRange == nil {
				item.RandRange = &Int32Range{From: 0, To: 255}
			}
			if item.RandRange.From < 0 || item.RandRange.To > 255 {
				return nil, errInvalidRange
			}
			var err error
			if item.Packet, err = PraseByteSlice(item.Packet, item.Type); err != nil {
				return nil, err
			}
			transform, err := buildCustomTransform(item.Transform)
			if err != nil {
				return nil, err
			}
			servers[i].Sequence = append(servers[i].Sequence, &custom.TCPItem{
				DelayMin: int64(item.Delay.From),
				DelayMax: int64(item.Delay.To),
				Rand:     item.Rand,
				RandMin:  item.RandRange.From,
				RandMax:  item.RandRange.To,
				Packet:   item.Packet,
				Save:     item.Capture,
				Var:      item.Reuse,
				Expr:     transform,
			})
		}
	}

	errors := make([]*custom.TCPSequence, len(c.Errors))
	for i, value := range c.Errors {
		errors[i] = &custom.TCPSequence{}
		for _, item := range value {
			if item.RandRange == nil {
				item.RandRange = &Int32Range{From: 0, To: 255}
			}
			if item.RandRange.From < 0 || item.RandRange.To > 255 {
				return nil, errInvalidRange
			}
			var err error
			if item.Packet, err = PraseByteSlice(item.Packet, item.Type); err != nil {
				return nil, err
			}
			transform, err := buildCustomTransform(item.Transform)
			if err != nil {
				return nil, err
			}
			errors[i].Sequence = append(errors[i].Sequence, &custom.TCPItem{
				DelayMin: int64(item.Delay.From),
				DelayMax: int64(item.Delay.To),
				Rand:     item.Rand,
				RandMin:  item.RandRange.From,
				RandMax:  item.RandRange.To,
				Packet:   item.Packet,
				Save:     item.Capture,
				Var:      item.Reuse,
				Expr:     transform,
			})
		}
	}

	return &custom.TCPConfig{
		Clients: clients,
		Servers: servers,
		Errors:  errors,
	}, nil
}

// FragmentMask is the fragment mask setting of the configuration.
type FragmentMask struct {
	coreconfig.FragmentMask
}

// Build converts the mask to its message.
func (c *FragmentMask) Build() (any, error) {
	return buildFragmentMask(&c.FragmentMask)
}

func buildFragmentMask(c *coreconfig.FragmentMask) (any, error) {

	config := &fragment.Config{}

	switch strings.ToLower(c.Packets) {
	case "tlshello":
		config.PacketsFrom = 0
		config.PacketsTo = 1
	case "":
		config.PacketsFrom = 0
		config.PacketsTo = 0
	default:
		from, to, err := ParseRangeString(c.Packets)
		if err != nil {
			return nil, errors.New("Invalid PacketsFrom").Base(err)
		}
		config.PacketsFrom = int64(from)
		config.PacketsTo = int64(to)
		if config.PacketsFrom == 0 {
			return nil, errors.New("PacketsFrom can't be 0")
		}
	}

	if len(c.Lengths) > 0 {
		for _, r := range c.Lengths {
			config.LengthsMin = append(config.LengthsMin, int64(r.From))
			config.LengthsMax = append(config.LengthsMax, int64(r.To))
		}
	} else {
		config.LengthsMin = append(config.LengthsMin, int64(c.Length.From))
		config.LengthsMax = append(config.LengthsMax, int64(c.Length.To))
	}

	if config.LengthsMin[len(config.LengthsMin)-1] == 0 {
		return nil, errors.New("last lengths entry min can't be 0")
	}

	if len(c.Delays) > 0 {
		for _, r := range c.Delays {
			config.DelaysMin = append(config.DelaysMin, int64(r.From))
			config.DelaysMax = append(config.DelaysMax, int64(r.To))
		}
	} else {
		config.DelaysMin = append(config.DelaysMin, int64(c.Delay.From))
		config.DelaysMax = append(config.DelaysMax, int64(c.Delay.To))
	}

	config.MaxSplitMin = int64(c.MaxSplit.From)
	config.MaxSplitMax = int64(c.MaxSplit.To)

	return config, nil
}

// NoiseItem is one noise item of the noise mask, from the configuration model.
type NoiseItem = coreconfig.NoiseItem

// NoiseMask is the noise mask setting of the configuration.
type NoiseMask struct {
	coreconfig.NoiseMask
}

// Build converts the mask to its message.
func (c *NoiseMask) Build() (any, error) {
	return buildNoiseMask(&c.NoiseMask)
}

func buildNoiseMask(c *coreconfig.NoiseMask) (any, error) {
	if c.Script != "" {
		if len(c.Noise) > 0 {
			return nil, errors.New(`"script" and "noise" can't be used together`)
		}
		if _, err := xlua.Compile(c.Script); err != nil {
			return nil, errors.New("failed to compile the noise script").Base(err)
		}
		return &noise.Config{
			ResetMin: int64(c.Reset.From),
			ResetMax: int64(c.Reset.To),
			Script:   c.Script,
		}, nil
	}

	noiseSlice := make([]*noise.Item, 0, len(c.Noise))
	for _, item := range c.Noise {
		if len(item.Packet) > 0 && item.Rand.To > 0 {
			return nil, errors.New("len(item.Packet) > 0 && item.Rand.To > 0")
		}
		if strings.ToLower(item.Type) == "exp" {
			var exp string
			if err := json.Unmarshal(item.Packet, &exp); err != nil {
				return nil, errors.New(`"packet" of noise "type": "exp" must be a string`).Base(err)
			}
			segments, err := parseNoiseExp(exp)
			if err != nil {
				return nil, err
			}
			noiseSlice = append(noiseSlice, &noise.Item{
				Segments: segments,
				DelayMin: int64(item.Delay.From),
				DelayMax: int64(item.Delay.To),
			})
			continue
		}
		if item.RandRange == nil {
			item.RandRange = &Int32Range{From: 0, To: 255}
		}
		if item.RandRange.From < 0 || item.RandRange.To > 255 {
			return nil, errors.New("invalid randRange")
		}
		var err error
		if item.Packet, err = PraseByteSlice(item.Packet, item.Type); err != nil {
			return nil, err
		}
		noiseSlice = append(noiseSlice, &noise.Item{
			RandMin:      int64(item.Rand.From),
			RandMax:      int64(item.Rand.To),
			RandRangeMin: item.RandRange.From,
			RandRangeMax: item.RandRange.To,
			Packet:       item.Packet,
			DelayMin:     int64(item.Delay.From),
			DelayMax:     int64(item.Delay.To),
		})
	}

	return &noise.Config{
		ResetMin: int64(c.Reset.From),
		ResetMax: int64(c.Reset.To),
		Items:    noiseSlice,
	}, nil
}

var noiseExpPattern = regexp.MustCompile(`<\s*([a-z]+)(?:\s+([^>]*?))?\s*>`)

func parseNoiseExp(exp string) ([]*noise.Segment, error) {
	var segments []*noise.Segment
	matches := noiseExpPattern.FindAllStringSubmatchIndex(exp, -1)
	last := 0
	for _, m := range matches {
		if strings.TrimSpace(exp[last:m[0]]) != "" {
			return nil, errors.New("invalid noise exp near ", exp[last:m[0]])
		}
		last = m[1]
		key := exp[m[2]:m[3]]
		arg := ""
		if m[4] >= 0 {
			arg = exp[m[4]:m[5]]
		}
		segment, err := buildNoiseSegment(key, arg)
		if err != nil {
			return nil, err
		}
		segments = append(segments, segment)
	}
	if strings.TrimSpace(exp[last:]) != "" {
		return nil, errors.New("invalid noise exp near ", exp[last:])
	}
	if len(segments) == 0 {
		return nil, errors.New("empty noise exp: ", exp)
	}
	return segments, nil
}

func buildNoiseSegment(key, arg string) (*noise.Segment, error) {
	sizeSegment := func(kind noise.Segment_Kind) (*noise.Segment, error) {
		if arg == "" {
			return nil, errors.New("<", key, "> in noise exp needs a size")
		}
		lo, hi, err := ParseRangeString(arg)
		if err != nil {
			return nil, err
		}
		if lo < 0 || hi < lo || hi > 65535 {
			return nil, errors.New("invalid size in noise exp: ", arg)
		}
		return &noise.Segment{Kind: kind, MinSize: int64(lo), MaxSize: int64(hi)}, nil
	}
	switch key {
	case "b":
		hexStr := strings.TrimPrefix(strings.TrimPrefix(strings.Join(strings.Fields(arg), ""), "0x"), "0X")
		if len(hexStr) == 0 {
			return nil, errors.New("empty bytes in noise exp")
		}
		raw, err := hex.DecodeString(hexStr)
		if err != nil {
			return nil, errors.New("invalid hex in noise exp: ", arg).Base(err)
		}
		return &noise.Segment{Kind: noise.Segment_BYTES, Bytes: raw}, nil
	case "r":
		return sizeSegment(noise.Segment_RANDOM)
	case "rc":
		return sizeSegment(noise.Segment_RANDOM_ASCII)
	case "rd":
		return sizeSegment(noise.Segment_RANDOM_DIGIT)
	case "t":
		if arg != "" {
			return nil, errors.New("<t> in noise exp takes no argument")
		}
		return &noise.Segment{Kind: noise.Segment_TIMESTAMP}, nil
	case "c":
		if arg != "" {
			return nil, errors.New("<c> in noise exp takes no argument")
		}
		return &noise.Segment{Kind: noise.Segment_COUNTER}, nil
	case "n":
		if arg != "" {
			return nil, errors.New("<n> in noise exp takes no argument")
		}
		return &noise.Segment{Kind: noise.Segment_NONCE}, nil
	default:
		return nil, errors.New("unknown <", key, "> in noise exp")
	}
}

// UDPItem is one item of the header-custom mask for UDP, from the model.
type UDPItem = coreconfig.UDPItem

// CustomTransform is one transform of a header-custom item, from the model.
type CustomTransform = coreconfig.CustomTransform

// CustomTransformArg is one argument of a transform, from the model.
type CustomTransformArg = coreconfig.CustomTransformArg

func validateCustomVarName(name string) error {
	if name == "" {
		return nil
	}
	if !customVarNamePattern.MatchString(name) {
		return errors.New("invalid variable name")
	}
	return nil
}

func validateCustomItemSpec(capture string, packet json.RawMessage, rand int32, reuse string, transform *CustomTransform) error {
	if err := validateCustomVarName(capture); err != nil {
		return err
	}
	if err := validateCustomVarName(reuse); err != nil {
		return err
	}

	kindCount := 0
	if len(packet) > 0 {
		kindCount++
	}
	if rand > 0 {
		kindCount++
	}
	if reuse != "" {
		kindCount++
	}
	if transform != nil {
		kindCount++
	}
	if kindCount > 1 {
		return errors.New("exactly one item kind must be set")
	}
	if kindCount == 0 && capture != "" {
		return errors.New("exactly one item kind must be set")
	}

	return nil
}

func buildCustomTransform(transform *CustomTransform) (*custom.Expr, error) {
	if transform == nil {
		return nil, nil
	}
	if transform.Op == "" {
		return nil, errors.New("transform op is required")
	}
	if len(transform.Args) == 0 {
		return nil, errors.New("transform args are required")
	}

	args := make([]*custom.ExprArg, 0, len(transform.Args))
	for _, arg := range transform.Args {
		parsedArg, err := buildCustomTransformArg(arg)
		if err != nil {
			return nil, err
		}
		args = append(args, parsedArg)
	}

	return &custom.Expr{
		Op:   transform.Op,
		Args: args,
	}, nil
}

func buildCustomTransformArg(arg CustomTransformArg) (*custom.ExprArg, error) {
	kindCount := 0
	if len(arg.Bytes) > 0 {
		kindCount++
	}
	if arg.U64 != nil {
		kindCount++
	}
	if arg.Reuse != "" {
		kindCount++
	}
	if arg.Metadata != "" {
		kindCount++
	}
	if arg.Transform != nil {
		kindCount++
	}
	if kindCount != 1 {
		return nil, errors.New("transform arg must set exactly one value")
	}

	if len(arg.Bytes) > 0 {
		value, err := PraseByteSlice(arg.Bytes, arg.Type)
		if err != nil {
			return nil, err
		}
		return &custom.ExprArg{
			Value: &custom.ExprArg_Bytes{
				Bytes: value,
			},
		}, nil
	}
	if arg.U64 != nil {
		return &custom.ExprArg{
			Value: &custom.ExprArg_U64{
				U64: *arg.U64,
			},
		}, nil
	}
	if arg.Reuse != "" {
		if err := validateCustomVarName(arg.Reuse); err != nil {
			return nil, err
		}
		return &custom.ExprArg{
			Value: &custom.ExprArg_Var{
				Var: arg.Reuse,
			},
		}, nil
	}
	if arg.Metadata != "" {
		return &custom.ExprArg{
			Value: &custom.ExprArg_Metadata{
				Metadata: arg.Metadata,
			},
		}, nil
	}

	parsedExpr, err := buildCustomTransform(arg.Transform)
	if err != nil {
		return nil, err
	}
	return &custom.ExprArg{
		Value: &custom.ExprArg_Expr{
			Expr: parsedExpr,
		},
	}, nil
}

// HeaderCustomUDP is the header-custom mask setting for UDP.
type HeaderCustomUDP struct {
	coreconfig.HeaderCustomUDP
}

// Build converts the mask to its message.
func (c *HeaderCustomUDP) Build() (any, error) {
	return buildHeaderCustomUDP(&c.HeaderCustomUDP)
}

func buildHeaderCustomUDP(c *coreconfig.HeaderCustomUDP) (any, error) {
	switch c.Mode {
	case "", "prefix", "standalone":
	default:
		return nil, errors.New("unknown udp mode")
	}

	for _, item := range c.Client {
		if err := validateCustomItemSpec(item.Capture, item.Packet, item.Rand, item.Reuse, item.Transform); err != nil {
			return nil, err
		}
	}
	for _, item := range c.Server {
		if err := validateCustomItemSpec(item.Capture, item.Packet, item.Rand, item.Reuse, item.Transform); err != nil {
			return nil, err
		}
	}

	client := make([]*custom.UDPItem, 0, len(c.Client))
	for _, item := range c.Client {
		if item.RandRange == nil {
			item.RandRange = &Int32Range{From: 0, To: 255}
		}
		if item.RandRange.From < 0 || item.RandRange.To > 255 {
			return nil, errors.New("invalid randRange")
		}
		var err error
		if item.Packet, err = PraseByteSlice(item.Packet, item.Type); err != nil {
			return nil, err
		}
		transform, err := buildCustomTransform(item.Transform)
		if err != nil {
			return nil, err
		}
		client = append(client, &custom.UDPItem{
			Rand:    item.Rand,
			RandMin: item.RandRange.From,
			RandMax: item.RandRange.To,
			Packet:  item.Packet,
			Save:    item.Capture,
			Var:     item.Reuse,
			Expr:    transform,
		})
	}

	server := make([]*custom.UDPItem, 0, len(c.Server))
	for _, item := range c.Server {
		if item.RandRange == nil {
			item.RandRange = &Int32Range{From: 0, To: 255}
		}
		if item.RandRange.From < 0 || item.RandRange.To > 255 {
			return nil, errors.New("invalid randRange")
		}
		var err error
		if item.Packet, err = PraseByteSlice(item.Packet, item.Type); err != nil {
			return nil, err
		}
		transform, err := buildCustomTransform(item.Transform)
		if err != nil {
			return nil, err
		}
		server = append(server, &custom.UDPItem{
			Rand:    item.Rand,
			RandMin: item.RandRange.From,
			RandMax: item.RandRange.To,
			Packet:  item.Packet,
			Save:    item.Capture,
			Var:     item.Reuse,
			Expr:    transform,
		})
	}

	if c.Mode == "standalone" {
		return &custom.UDPStandaloneConfig{
			Client: client,
			Server: server,
		}, nil
	} else {
		return &custom.UDPConfig{
			Client: client,
			Server: server,
		}, nil
	}
}

// MkcpLegacy is the legacy mkcp header setting.
type MkcpLegacy struct {
	coreconfig.MkcpLegacy
}

// Build converts the setting to its message.
func (c *MkcpLegacy) Build() (any, error) {
	return buildMkcpLegacy(&c.MkcpLegacy)
}

func buildMkcpLegacy(c *coreconfig.MkcpLegacy) (any, error) {
	if len(c.Header) == 0 {
		if len(c.Value) == 0 {
			return &original.Config{}, nil
		} else {
			return &aes128gcm.Config{Password: c.Value}, nil
		}
	}
	switch strings.ToLower(c.Header) {
	case "dns":
		domain := c.Value
		if len(domain) == 0 {
			domain = "www.baidu.com"
		}
		return &header.Config{ID: 0, Domain: domain}, nil
	case "dtls":
		return &header.Config{ID: 1}, nil
	case "srtp":
		return &header.Config{ID: 2}, nil
	case "utp":
		return &header.Config{ID: 3}, nil
	case "wechat":
		return &header.Config{ID: 4}, nil
	case "wireguard":
		return &header.Config{ID: 5}, nil
	default:
		return nil, errors.New("invalid header ", c.Header)
	}
}

// Salamander is the salamander mask setting.
type Salamander struct {
	coreconfig.Salamander
}

// Build converts the setting to its message.
func (c *Salamander) Build() (any, error) {
	return buildSalamander(&c.Salamander)
}

func buildSalamander(c *coreconfig.Salamander) (any, error) {
	if c.PacketSize.To > 0 {
		if c.PacketSize.From <= 0 || c.PacketSize.To > 2048 {
			return nil, errors.New("gecko: invalid min/max packet size")
		}
		return &salamander.GeckoConfig{
			Password:      c.Password,
			MinPacketSize: c.PacketSize.From,
			MaxPacketSize: c.PacketSize.To,
		}, nil
	}
	return &salamander.Config{
		Password: c.Password,
	}, nil
}

// Sudoku is the sudoku mask setting of the configuration.
type Sudoku struct {
	coreconfig.Sudoku
}

// Build converts the mask to its message.
func (c *Sudoku) Build() (any, error) {
	return buildSudoku(&c.Sudoku)
}

func buildSudoku(c *coreconfig.Sudoku) (any, error) {
	customTable := c.CustomTable
	if customTable == "" {
		customTable = c.LegacyCustomTable
	}
	customTables := c.CustomTables
	if len(customTables) == 0 {
		customTables = c.LegacyCustomSets
	}

	paddingMin := c.PaddingMin
	if paddingMin == 0 {
		paddingMin = c.LegacyPaddingMin
	}
	paddingMax := c.PaddingMax
	if paddingMax == 0 {
		paddingMax = c.LegacyPaddingMax
	}

	return &sudoku.Config{
		Password:     c.Password,
		Ascii:        c.ASCII,
		CustomTable:  customTable,
		CustomTables: customTables,
		PaddingMin:   paddingMin,
		PaddingMax:   paddingMax,
	}, nil
}

// XDNSDomain is one domain group of the xdns mask, from the configuration model.
type XDNSDomain = coreconfig.XDNSDomain

// XDNSResolver is one resolver of the xdns mask, from the configuration model.
type XDNSResolver = coreconfig.XDNSResolver

// XDNS is the xdns mask setting of the configuration.
type XDNS struct {
	coreconfig.XDNS
}

// Build converts the mask to its message.
func (c *XDNS) Build() (any, error) {
	return buildXDNS(&c.XDNS)
}

func buildXDNS(c *coreconfig.XDNS) (any, error) {
	var domains []*xdns.DomainProto
	var resolvers []*xdns.ResolverProto
	for i := range c.Domains {
		if c.Domains[i].LenLimit == 0 {
			c.Domains[i].LenLimit = 255
		}
		if c.Domains[i].LabelLimit == 0 {
			c.Domains[i].LabelLimit = 63
		}
		for j := range c.Domains[i].Names {
			domain, err := xdns.NewDomain(c.Domains[i].Names[j], int(c.Domains[i].LenLimit), int(c.Domains[i].LabelLimit), []uint16{1, 5, 16, 28}, uint16(c.Domains[i].Edns0), c.Domains[i].Alphabet)
			if err != nil {
				return nil, err
			}
			errors.LogInfo(context.Background(), domain.Show())
			domains = append(domains, &xdns.DomainProto{
				Name:       c.Domains[i].Names[j],
				LenLimit:   c.Domains[i].LenLimit,
				LabelLimit: c.Domains[i].LabelLimit,
				Types:      c.Domains[i].Types,
				Edns0:      c.Domains[i].Edns0,
				Alphabet:   c.Domains[i].Alphabet,
			})
		}
	}
	for i := range c.Resolvers {
		worker := c.Resolvers[i].Worker
		if worker == 0 {
			worker = 32
		}
		var tlsConfig *tls.Config
		var quicConfig *internet.QuicParams
		if c.Resolvers[i].TlsConfig != nil {
			proto, err := (&TLSConfig{*c.Resolvers[i].TlsConfig}).Build()
			if err != nil {
				return nil, err
			}
			tlsConfig = proto.(*tls.Config)
		}
		if c.Resolvers[i].QuicConfig != nil {
			proto, err := (&QuicParamsConfig{*c.Resolvers[i].QuicConfig}).Build()
			if err != nil {
				return nil, err
			}
			quicConfig = proto.(*internet.QuicParams)
		}
		for j := range c.Resolvers[i].Addrs {
			var u *url.URL
			var e error
			if !strings.Contains(c.Resolvers[i].Addrs[j], "://") {
				u, e = url.Parse("udp://" + c.Resolvers[i].Addrs[j])
			} else {
				u, e = url.Parse(c.Resolvers[i].Addrs[j])
			}
			if e != nil {
				return nil, e
			}
			var addr string
			switch u.Scheme {
			case "tcp", "udp":
				host := u.Hostname()
				port := u.Port()
				if port == "" {
					port = "53"
				}
				addr = net.JoinHostPort(host, port)
			case "https":
				addr = u.String()
			default:
				return nil, errors.New("invalid protocol")
			}
			resolvers = append(resolvers, &xdns.ResolverProto{Type: u.Scheme, Addr: addr, Worker: worker, TlsConfig: tlsConfig, QuicConfig: quicConfig})
		}
	}
	if c.ExtraPoll < 0 || c.ExtraPoll > 3 {
		return nil, errors.New("c.ExtraPoll < 0 || c.ExtraPoll > 3")
	}
	return &xdns.Config{Domains: domains, Resolvers: resolvers, ExtraPoll: c.ExtraPoll}, nil
}

// XMC is the xmc mask setting of the configuration.
type XMC struct {
	coreconfig.XMC
}

// XMCPaddingTurn is one turn of the xmc padding schedule, from the model.
type XMCPaddingTurn = coreconfig.XMCPaddingTurn

// XMCYggdrasil enables third-party login through a Yggdrasil API server such
// as Blessing Skin. The client authenticates with the API and the server
// verifies players through its hasJoined endpoint.
// XMCYggdrasil is the Yggdrasil account of the xmc mask, from the model.
type XMCYggdrasil = coreconfig.XMCYggdrasil

// XMCProfile is one Minecraft profile of the xmc mask, from the configuration
// model. The conversion to the message stays here, together with the pattern the
// user name has to match.
type XMCProfile = coreconfig.XMCProfile

var xmcUsernamePattern = regexp.MustCompile(`^[A-Za-z0-9_]{3,16}$`)

// buildXMCProfile converts the profile to its message.
func buildXMCProfile(c *coreconfig.XMCProfile) (*xmc.Profile, error) {
	if !xmcUsernamePattern.MatchString(c.Username) {
		return nil, fmt.Errorf("invalid minecraft profile username: %q", c.Username)
	}

	profileUUID, err := googleuuid.Parse(c.UUID)
	if err != nil {
		return nil, fmt.Errorf("invalid minecraft profile UUID: %w", err)
	}
	if c.TexturesValue == "" || c.TexturesSignature == "" {
		return nil, fmt.Errorf("incomplete minecraft profile textures")
	}

	return &xmc.Profile{
		Username:          c.Username,
		Uuid:              append([]byte(nil), profileUUID[:]...),
		TexturesValue:     c.TexturesValue,
		TexturesSignature: c.TexturesSignature,
	}, nil
}

// Build converts the mask to its message.
func (c *XMC) Build() (any, error) {
	return buildXMC(&c.XMC)
}

func buildXMC(c *coreconfig.XMC) (any, error) {
	// An explicit empty array is different from an absent field: json leaves
	// Paddings nil when it is absent, so the user can turn padding off without
	// a second switch.
	noPadding := c.NoPadding || (c.Paddings != nil && len(c.Paddings) == 0)
	if noPadding && len(c.Paddings) > 0 {
		return nil, fmt.Errorf("minecraft padding: noPadding and paddings are exclusive")
	}
	config := &xmc.Config{Hostname: c.Hostname, NoPadding: noPadding}
	for i, turn := range c.Paddings {
		minimum, maximum, err := ParseRangeString(turn.Length)
		if err != nil {
			return nil, fmt.Errorf("minecraft padding turn %d length: %w", i, err)
		}
		delayMinimum, delayMaximum, err := ParseRangeString(turn.Delay)
		if err != nil {
			return nil, fmt.Errorf("minecraft padding turn %d delay: %w", i, err)
		}

		var direction int32
		switch turn.Direction {
		case "c2s", "client-to-server":
			direction = 1
		case "s2c", "server-to-client":
			direction = 2
		default:
			return nil, fmt.Errorf("minecraft padding turn %d: invalid direction %q (must be \"c2s\" or \"s2c\")", i, turn.Direction)
		}

		config.Paddings = append(config.Paddings, &xmc.Padding{
			LengthMin: int64(minimum),
			LengthMax: int64(maximum),
			Direction: direction,
			DelayMin:  int64(delayMinimum),
			DelayMax:  int64(delayMaximum),
		})
	}
	if err := config.ValidatePadding(); err != nil {
		return nil, fmt.Errorf("minecraft padding: %w", err)
	}

	var yggdrasil *xmc.Yggdrasil
	if c.Yggdrasil != nil {
		if c.Yggdrasil.API == "" {
			return nil, fmt.Errorf("empty minecraft yggdrasil api")
		}

		apiURL, err := url.Parse(c.Yggdrasil.API)
		if err != nil {
			return nil, fmt.Errorf("invalid minecraft yggdrasil api: %w", err)
		}
		if apiURL.Scheme != "http" && apiURL.Scheme != "https" {
			return nil, fmt.Errorf("invalid minecraft yggdrasil api scheme: %q", apiURL.Scheme)
		}
		if apiURL.Host == "" {
			return nil, fmt.Errorf("invalid minecraft yggdrasil api: missing host")
		}
		if apiURL.RawQuery != "" || apiURL.Fragment != "" {
			return nil, fmt.Errorf("invalid minecraft yggdrasil api: query and fragment are not allowed")
		}
		if (c.Yggdrasil.Username == "") != (c.Yggdrasil.Password == "") {
			return nil, fmt.Errorf("minecraft yggdrasil username and password must be set together")
		}
		if c.Yggdrasil.Player != "" && !xmcUsernamePattern.MatchString(c.Yggdrasil.Player) {
			return nil, fmt.Errorf("invalid minecraft yggdrasil player: %q", c.Yggdrasil.Player)
		}

		yggdrasil = &xmc.Yggdrasil{
			Api:      strings.TrimSuffix(c.Yggdrasil.API, "/"),
			Username: c.Yggdrasil.Username,
			Password: c.Yggdrasil.Password,
			Player:   c.Yggdrasil.Player,
		}
	} else if len(c.Profiles) == 0 {
		return nil, fmt.Errorf("minecraft profiles are required")
	}

	var (
		rsaPrivateKey *rsa.PrivateKey
		password      string
	)
	if yggdrasil != nil {
		// Yggdrasil mode binds the transport through the session hash: the
		// client joins with the server's actual public key and the server
		// checks hasJoined. The key pair is therefore generated per process
		// instead of being derived from the password.
		generated, err := rsa.GenerateKey(rand.Reader, 1024)
		if err != nil {
			return nil, fmt.Errorf("generate minecraft rsa key: %w", err)
		}
		rsaPrivateKey = generated

		warning := `XMC: Yggdrasil mode is enabled. The RSA key pair is generated randomly and "password" is no longer used to derive it or to authenticate the transport; players are verified through ` + yggdrasil.Api + ` (client /join, server /hasJoined). Both ends must be able to reach the Yggdrasil API.`
		if c.Password != "" {
			warning += ` The configured "password" is ignored.`
		}
		errors.LogWarning(context.Background(), warning)
	} else {
		if c.Password == "" {
			return nil, fmt.Errorf("empty password")
		}
		if len(c.Password) > xmc.MaxLegacyPasswordLength {
			return nil, fmt.Errorf("minecraft password is longer than %d bytes", xmc.MaxLegacyPasswordLength)
		}
		derived, err := xmc.DeriveRSAKey(c.Password)
		if err != nil {
			return nil, fmt.Errorf("derive minecraft rsa key: %w", err)
		}
		rsaPrivateKey = derived
		password = c.Password
	}

	rsaPublicKey, err := x509.MarshalPKIXPublicKey(&rsaPrivateKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("marshal minecraft rsa public key: %w", err)
	}

	profiles := make([]*xmc.Profile, 0, len(c.Profiles))
	for i := range c.Profiles {
		profile, err := buildXMCProfile(&c.Profiles[i])
		if err != nil {
			return nil, fmt.Errorf("build minecraft profile %d: %w", i, err)
		}
		profiles = append(profiles, profile)
	}

	config.Password = password
	config.RsaPrivateKey = x509.MarshalPKCS1PrivateKey(rsaPrivateKey)
	config.RsaPublicKey = rsaPublicKey
	config.Profiles = profiles
	config.Yggdrasil = yggdrasil
	return config, nil
}

// Xicmp is the xicmp mask setting of the configuration.
type Xicmp struct {
	coreconfig.Xicmp
}

// Build converts the mask to its message.
func (c *Xicmp) Build() (any, error) {
	return buildXicmp(&c.Xicmp)
}

func buildXicmp(c *coreconfig.Xicmp) (any, error) {

	for _, ip := range c.IPs {
		if _, err := netip.ParseAddr(ip); err != nil {
			return nil, err
		}
	}

	config := &xicmp.Config{
		DGRAM: c.DGRAM,
		IPs:   c.IPs,
	}

	return config, nil
}

// Realm is the realm mask setting of the configuration.
type Realm struct {
	coreconfig.Realm
}

// Build converts the mask to its message.
func (c *Realm) Build() (any, error) {
	return buildRealm(&c.Realm)
}

func buildRealm(c *coreconfig.Realm) (any, error) {
	var scheme, host, port, token, id string
	var stunServers []string
	var tlsConfig *tls.Config

	u, err := url.Parse(c.Url)
	if err != nil {
		return nil, err
	}

	switch u.Scheme {
	case "realm":
		scheme = "https"
	case "realm+http":
		scheme = "http"
	default:
		return nil, errors.New("invalid scheme", u.Scheme)
	}

	host = u.Hostname()
	if host == "" {
		return nil, errors.New("invalid host", host)
	}

	port = u.Port()
	if port == "" {
		port = "443"
		if scheme == "http" {
			port = "80"
		}
	}

	token, err = url.PathUnescape(u.User.String())
	if err != nil {
		return nil, err
	}
	if token == "" {
		return nil, errors.New("invalid token", token)
	}

	id, err = url.PathUnescape(strings.TrimPrefix(u.EscapedPath(), "/"))
	if err != nil {
		return nil, err
	}
	if id == "" {
		return nil, errors.New("invalid id", id)
	}

	if len(c.StunServers) == 0 {
		return nil, errors.New("empty stunServers")
	}

	for _, s := range c.StunServers {
		_, _, err = net.SplitHostPort(s)
		if err != nil {
			return nil, err
		}
	}

	stunServers = c.StunServers

	if c.TlsConfig != nil {
		tc, err := (&TLSConfig{*c.TlsConfig}).Build()
		if err != nil {
			return nil, err
		}
		tlsConfig = tc.(*tls.Config)
	}

	return &realm.Config{
		Scheme:      scheme,
		Host:        host,
		Port:        port,
		Token:       token,
		ID:          id,
		StunServers: stunServers,
		TlsConfig:   tlsConfig,
		IPMode:      strings.ToLower(c.IPMode),
		PortMapping: c.PortMapping,
	}, nil
}

// UDPHop is the udphop mask setting of the configuration.
type UDPHop struct {
	coreconfig.UDPHop
}

// Build converts the mask to its message.
func (c *UDPHop) Build() (any, error) {
	return buildUDPHop(&c.UDPHop)
}

func buildUDPHop(c *coreconfig.UDPHop) (any, error) {

	var local, remote, remoteOnce bool
	for _, mode := range strings.Split(c.Mode, ",") {
		switch strings.ToLower(mode) {
		case "intervallocal":
			local = true
		case "intervalremote":
			remote = true
		case "perconnremote":
			remoteOnce = true
		default:
			return nil, errors.New("invalid mode ", mode)
		}
	}
	var remoteIPs []string
	for _, ip := range c.RemoteIPs {
		prefix, err := netip.ParsePrefix(ip)
		if err == nil {
			remoteIPs = append(remoteIPs, prefix.String())
			continue
		}
		addr, err := netip.ParseAddr(ip)
		if err == nil {
			remoteIPs = append(remoteIPs, netip.PrefixFrom(addr, addr.BitLen()).String())
			continue
		}
		return nil, errors.New("invalid ip ", ip)
	}
	interval := c.Interval
	if interval.From == 0 && interval.To == 0 {
		interval.From, interval.To = 30, 30
	}
	if interval.From < 5 {
		return nil, errors.New("interval must be at least 5")
	}
	return &udphop.Config{
		Local:       local,
		Remote:      remote,
		RemoteOnce:  remoteOnce,
		IntervalMin: int64(interval.From),
		IntervalMax: int64(interval.To),
		RemoteIPs:   remoteIPs,
		RemotePorts: BuildPortList(&c.RemotePorts).Ports(),
	}, nil
}

// Mask is one entry of a mask list, from the configuration model.
type Mask = coreconfig.Mask

// buildMask converts one mask entry to its message.
func buildMask(c *coreconfig.Mask, tcp bool) (any, error) {
	installMaskSettingsTypes()
	loader := udpmaskLoader
	if tcp {
		loader = tcpmaskLoader
	}

	settings := []byte("{}")
	if c.Settings != nil {
		settings = ([]byte)(*c.Settings)
	}
	rawConfig, err := loader.LoadWithID(settings, c.Type)
	if err != nil {
		return nil, err
	}
	ts, err := rawConfig.(Buildable).Build()
	if err != nil {
		return nil, err
	}
	return ts, nil
}

// QuicParamsConfig is the QUIC setting of the configuration.
type QuicParamsConfig struct {
	coreconfig.QuicParams
}

// the fields below are read by the conversion
type quicParamsFields struct {
	Congestion                    string    `json:"congestion"`
	Debug                         bool      `json:"debug"`
	BbrProfile                    string    `json:"bbrProfile"`
	BrutalUp                      Bandwidth `json:"brutalUp"`
	BrutalDown                    Bandwidth `json:"brutalDown"`
	BrutalDisableLossCompensation bool      `json:"brutalDisableLossCompensation"`
	InitStreamReceiveWindow       uint64    `json:"initStreamReceiveWindow"`
	MaxStreamReceiveWindow        uint64    `json:"maxStreamReceiveWindow"`
	InitConnectionReceiveWindow   uint64    `json:"initConnectionReceiveWindow"`
	MaxConnectionReceiveWindow    uint64    `json:"maxConnectionReceiveWindow"`
	MaxIdleTimeout                int64     `json:"maxIdleTimeout"`
	KeepAlivePeriod               int64     `json:"keepAlivePeriod"`
	DisablePathMTUDiscovery       bool      `json:"disablePathMTUDiscovery"`
	DisableChromeParrot           bool      `json:"disableChromeParrot"`
	DisableGSO                    bool      `json:"disableGSO"`
	MaxIncomingStreams            int64     `json:"maxIncomingStreams"`
	DisableStatelessReset         bool      `json:"disableStatelessReset"`
}

func (config *QuicParamsConfig) Build() (any, error) {
	profile := strings.ToLower(config.BbrProfile)
	switch profile {
	case "", string(bbr.ProfileConservative), string(bbr.ProfileStandard), string(bbr.ProfileAggressive):
		if profile == "" {
			profile = string(bbr.ProfileStandard)
		}
	default:
		return nil, errors.New("unknown bbr profile")
	}

	up, err := bandwidthBps(config.BrutalUp)
	if err != nil {
		return nil, err
	}
	down, err := bandwidthBps(config.BrutalDown)
	if err != nil {
		return nil, err
	}

	if up > 0 && up < 65536 {
		return nil, errors.New("BrutalUp must be at least 65536 bytes per second")
	}
	if down > 0 && down < 65536 {
		return nil, errors.New("BrutalDown must be at least 65536 bytes per second")
	}

	config.Congestion = strings.ToLower(config.Congestion)
	switch config.Congestion {
	case "", "brutal", "reno", "bbr":
	case "force-brutal":
		if up == 0 {
			return nil, errors.New("force-brutal requires up")
		}
	default:
		return nil, errors.New("unknown congestion control: ", config.Congestion, ", valid values: reno, bbr, brutal, force-brutal")
	}

	if config.InitStreamReceiveWindow > 0 && config.InitStreamReceiveWindow < 16384 {
		return nil, errors.New("InitStreamReceiveWindow must be at least 16384")
	}
	if config.MaxStreamReceiveWindow > 0 && config.MaxStreamReceiveWindow < 16384 {
		return nil, errors.New("MaxStreamReceiveWindow must be at least 16384")
	}
	if config.InitConnectionReceiveWindow > 0 && config.InitConnectionReceiveWindow < 16384 {
		return nil, errors.New("InitConnectionReceiveWindow must be at least 16384")
	}
	if config.MaxConnectionReceiveWindow > 0 && config.MaxConnectionReceiveWindow < 16384 {
		return nil, errors.New("MaxConnectionReceiveWindow must be at least 16384")
	}
	if config.MaxIdleTimeout != 0 && (config.MaxIdleTimeout < 4 || config.MaxIdleTimeout > 120) {
		return nil, errors.New("MaxIdleTimeout must be between 4 and 120")
	}
	if config.KeepAlivePeriod != 0 && (config.KeepAlivePeriod < 2 || config.KeepAlivePeriod > 60) {
		return nil, errors.New("KeepAlivePeriod must be between 2 and 60")
	}
	if config.MaxIncomingStreams != 0 && config.MaxIncomingStreams < 8 {
		return nil, errors.New("MaxIncomingStreams must be at least 8")
	}

	if config.Debug {
		os.Setenv("HYSTERIA_BBR_DEBUG", "true")
		os.Setenv("HYSTERIA_BRUTAL_DEBUG", "true")
	}

	return &internet.QuicParams{
		Congestion:                    config.Congestion,
		BbrProfile:                    profile,
		BrutalUp:                      up,
		BrutalDown:                    down,
		BrutalDisableLossCompensation: config.BrutalDisableLossCompensation,
		InitStreamReceiveWindow:       config.InitStreamReceiveWindow,
		MaxStreamReceiveWindow:        config.MaxStreamReceiveWindow,
		InitConnReceiveWindow:         config.InitConnectionReceiveWindow,
		MaxConnReceiveWindow:          config.MaxConnectionReceiveWindow,
		MaxIdleTimeout:                config.MaxIdleTimeout,
		KeepAlivePeriod:               config.KeepAlivePeriod,
		DisablePathMtuDiscovery:       config.DisablePathMTUDiscovery,
		DisableChromeParrot:           config.DisableChromeParrot,
		DisableGSO:                    config.DisableGSO,
		MaxIncomingStreams:            config.MaxIncomingStreams,
		DisableStatelessReset:         config.DisableStatelessReset,
	}, nil
}

type FinalMask struct {
	Tcp        []Mask            `json:"tcp"`
	Udp        []Mask            `json:"udp"`
	QuicParams *QuicParamsConfig `json:"quicParams"`
}
