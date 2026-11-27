package config

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/platform"
)

// ParseIntPort parses a JSON number as a port.
func ParseIntPort(data []byte) (net.Port, error) {
	var intPort uint32
	if err := json.Unmarshal(data, &intPort); err != nil {
		return net.Port(0), err
	}
	return net.PortFromInt(intPort)
}

// ParseStringPort parses "443" or "443-500", resolving the "env:NAME" form.
func ParseStringPort(s string) (net.Port, net.Port, error) {
	if strings.HasPrefix(s, "env:") {
		s = platform.NewEnvFlag(s[4:]).GetValue(func() string { return "" })
	}

	pair := strings.SplitN(s, "-", 2)
	if len(pair) == 0 {
		return net.Port(0), net.Port(0), errors.New("invalid port range: ", s)
	}
	if len(pair) == 1 {
		port, err := net.PortFromString(pair[0])
		return port, port, err
	}

	fromPort, err := net.PortFromString(pair[0])
	if err != nil {
		return net.Port(0), net.Port(0), err
	}
	toPort, err := net.PortFromString(pair[1])
	if err != nil {
		return net.Port(0), net.Port(0), err
	}
	return fromPort, toPort, nil
}

// ParseJSONStringPort parses a JSON string as a port or a port range.
func ParseJSONStringPort(data []byte) (net.Port, net.Port, error) {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return net.Port(0), net.Port(0), err
	}
	return ParseStringPort(s)
}

// SplitFromSecondDash splits a string at its second dash, so that a negative
// left end of a range is not mistaken for the separator.
func SplitFromSecondDash(s string) []string {
	parts := strings.SplitN(s, "-", 3)
	if len(parts) < 3 {
		return []string{s}
	}
	return []string{parts[0] + "-" + parts[1], parts[2]}
}

// ParseRangeString parses a range written as a string, including negative
// numbers: "114-514", "-114-514", "-1919--810", "114514", "" (which is 0).
func ParseRangeString(str string) (int, int, error) {
	if value, err := strconv.Atoi(str); err == nil {
		return value, value, nil
	}
	if str == "" {
		return 0, 0, nil
	}
	var pair []string
	if strings.HasPrefix(str, "-") {
		pair = SplitFromSecondDash(str)
	} else {
		pair = strings.SplitN(str, "-", 2)
	}
	if len(pair) == 2 {
		left, err := strconv.Atoi(pair[0])
		right, err2 := strconv.Atoi(pair[1])
		if err == nil && err2 == nil {
			return left, right, nil
		}
	}
	return 0, 0, errors.New("invalid range string: ", str)
}
