package config

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/platform"
)

// StringList is a list of strings that a configuration may write either as a
// JSON array or as one comma-separated string.
type StringList []string

// NewStringList returns a StringList holding raw.
func NewStringList(raw []string) *StringList {
	list := StringList(raw)
	return &list
}

// Len implements sort.Interface.
func (v StringList) Len() int {
	return len(v)
}

// UnmarshalJSON implements encoding/json.Unmarshaler.
func (v *StringList) UnmarshalJSON(data []byte) error {
	var strarray []string
	if err := json.Unmarshal(data, &strarray); err == nil {
		*v = *NewStringList(strarray)
		return nil
	}

	var rawstr string
	if err := json.Unmarshal(data, &rawstr); err == nil {
		*v = *NewStringList(strings.Split(rawstr, ","))
		return nil
	}
	return errors.New("unknown format of a string list: " + string(data))
}

// Address is an address a configuration writes as a string. It accepts the
// "env:NAME" form, which reads the address from the environment.
type Address struct {
	net.Address
}

// MarshalJSON implements encoding/json.Marshaler.
func (v *Address) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.Address.String())
}

// UnmarshalJSON implements encoding/json.Unmarshaler.
func (v *Address) UnmarshalJSON(data []byte) error {
	var rawStr string
	if err := json.Unmarshal(data, &rawStr); err != nil {
		return errors.New("invalid address: ", string(data)).Base(err)
	}
	if strings.HasPrefix(rawStr, "env:") {
		rawStr = platform.NewEnvFlag(rawStr[4:]).GetValue(func() string { return "" })
	}
	v.Address = net.ParseAddress(rawStr)
	return nil
}

// Network is one of "tcp", "udp", "unix".
type Network string

// NetworkList is a list of networks that a configuration may write either as a
// JSON array or as one comma-separated string.
type NetworkList []Network

// UnmarshalJSON implements encoding/json.Unmarshaler.
func (v *NetworkList) UnmarshalJSON(data []byte) error {
	var strarray []Network
	if err := json.Unmarshal(data, &strarray); err == nil {
		*v = NetworkList(strarray)
		return nil
	}

	var rawstr Network
	if err := json.Unmarshal(data, &rawstr); err == nil {
		strlist := strings.Split(string(rawstr), ",")
		nl := make([]Network, len(strlist))
		for idx, network := range strlist {
			nl[idx] = Network(network)
		}
		*v = nl
		return nil
	}
	return errors.New("unknown format of a string list: " + string(data))
}

// PortRange is a port or a range of ports, written as 443 or "443-500".
type PortRange struct {
	From uint32
	To   uint32
}

// MarshalJSON implements encoding/json.Marshaler.
func (v *PortRange) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.String())
}

// String implements fmt.Stringer.
func (v *PortRange) String() string {
	if v.From == v.To {
		return strconv.Itoa(int(v.From))
	}
	return fmt.Sprintf("%d-%d", v.From, v.To)
}

// UnmarshalJSON implements encoding/json.Unmarshaler.
func (v *PortRange) UnmarshalJSON(data []byte) error {
	port, err := ParseIntPort(data)
	if err == nil {
		v.From = uint32(port)
		v.To = uint32(port)
		return nil
	}

	from, to, err := ParseJSONStringPort(data)
	if err == nil {
		v.From = uint32(from)
		v.To = uint32(to)
		if v.From > v.To || v.To > math.MaxUint16 {
			return errors.New("invalid port range ", v.From, " -> ", v.To)
		}
		return nil
	}

	return errors.New("invalid port range: ", string(data))
}

// PortList is a list of ports and port ranges, written as "80,443,1000-2000".
type PortList struct {
	Range []PortRange
}

// MarshalJSON implements encoding/json.Marshaler.
func (v *PortList) MarshalJSON() ([]byte, error) {
	portStr := v.String()
	if port, err := strconv.Atoi(portStr); err == nil {
		return json.Marshal(port)
	}
	return json.Marshal(portStr)
}

// String implements fmt.Stringer.
func (v PortList) String() string {
	ports := []string{}
	for _, port := range v.Range {
		ports = append(ports, port.String())
	}
	return strings.Join(ports, ",")
}

// UnmarshalJSON implements encoding/json.Unmarshaler.
func (list *PortList) UnmarshalJSON(data []byte) error {
	var listStr string
	var number uint32
	if err := json.Unmarshal(data, &listStr); err != nil {
		if err2 := json.Unmarshal(data, &number); err2 != nil {
			return errors.New("invalid port: ", string(data)).Base(err2)
		}
	}
	for _, rangeStr := range strings.Split(listStr, ",") {
		trimmed := strings.TrimSpace(rangeStr)
		if len(trimmed) == 0 {
			continue
		}
		if strings.Contains(trimmed, "-") || strings.Contains(trimmed, "env:") {
			from, to, err := ParseStringPort(trimmed)
			if err != nil {
				return errors.New("invalid port range: ", trimmed).Base(err)
			}
			list.Range = append(list.Range, PortRange{From: uint32(from), To: uint32(to)})
			continue
		}
		port, err := ParseIntPort([]byte(trimmed))
		if err != nil {
			return errors.New("invalid port: ", trimmed).Base(err)
		}
		list.Range = append(list.Range, PortRange{From: uint32(port), To: uint32(port)})
	}
	if number != 0 {
		list.Range = append(list.Range, PortRange{From: number, To: number})
	}
	return nil
}

// User is one authenticated user of an inbound.
type User struct {
	EmailString string `json:"email"`
	LevelByte   byte   `json:"level"`
}

// Int32Range is an integer or a range of integers, written as 1 or "1-2".
// Negative integers may be sentinels: they do not parse as a range. From and To
// are always ordered; Left and Right keep what the user wrote.
type Int32Range struct {
	Left  int32
	Right int32
	From  int32
	To    int32
}

// MarshalJSON implements encoding/json.Marshaler.
func (v *Int32Range) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.String())
}

// String implements fmt.Stringer.
func (v Int32Range) String() string {
	if v.Left == v.Right {
		return strconv.Itoa(int(v.Left))
	}
	return fmt.Sprintf("%d-%d", v.Left, v.Right)
}

// UnmarshalJSON implements encoding/json.Unmarshaler.
func (v *Int32Range) UnmarshalJSON(data []byte) error {
	defer v.ensureOrder()
	var str string
	var rawint int32
	if err := json.Unmarshal(data, &str); err == nil {
		left, right, err := ParseRangeString(str)
		if err == nil {
			v.Left, v.Right = int32(left), int32(right)
			return nil
		}
	} else if err := json.Unmarshal(data, &rawint); err == nil {
		v.Left = rawint
		v.Right = rawint
		return nil
	}
	return errors.New(`Invalid integer range, expected either string of form "1-2" or plain integer.`)
}

// ensureOrder gives From and To their values and orders them.
func (v *Int32Range) ensureOrder() {
	v.From, v.To = v.Left, v.Right
	if v.From > v.To {
		v.From, v.To = v.To, v.From
	}
}
