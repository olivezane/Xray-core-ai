// Code that is not generated: the settings of this package are plain Go values,
// and they say the name a configuration written for the API calls them.

package custom

import "encoding/json"

// Expr is an operation over its arguments.
type Expr struct {
	Op   string   `json:"op,omitempty"`
	Args []*ExprArg `json:"args,omitempty"`
}

// ExprArg is one argument of an operation: exactly one of the variants below.
type ExprArg struct {
	Value isExprArg_Value `json:"-"`
}

type isExprArg_Value interface {
	isExprArg_Value()
}

// The variants of ExprArg.
type (
	ExprArg_Bytes    struct{ Bytes []byte `json:"bytes,omitempty"` }
	ExprArg_U64      struct{ U64 uint64 `json:"u64,omitempty"` }
	ExprArg_Var      struct{ Var string `json:"var,omitempty"` }
	ExprArg_Metadata struct{ Metadata string `json:"metadata,omitempty"` }
	ExprArg_Expr     struct{ Expr *Expr `json:"expr,omitempty"` }
)

func (*ExprArg_Bytes) isExprArg_Value()    {}
func (*ExprArg_U64) isExprArg_Value()      {}
func (*ExprArg_Var) isExprArg_Value()      {}
func (*ExprArg_Metadata) isExprArg_Value() {}
func (*ExprArg_Expr) isExprArg_Value()     {}

// MarshalJSON writes an argument the way a configuration writes it: the variant of
// Value becomes the field it names, because JSON cannot hold an interface.
func (x *ExprArg) MarshalJSON() ([]byte, error) {
	return json.Marshal(x.Value)
}

// UnmarshalJSON reads an argument the way a configuration writes it.
func (x *ExprArg) UnmarshalJSON(b []byte) error {
	fields := struct {
		Bytes    []byte `json:"bytes"`
		U64      uint64 `json:"u64"`
		Var      string `json:"var"`
		Metadata string `json:"metadata"`
		Expr     *Expr  `json:"expr"`
	}{}
	if err := json.Unmarshal(b, &fields); err != nil {
		return err
	}
	switch {
	case fields.Expr != nil:
		x.Value = &ExprArg_Expr{Expr: fields.Expr}
	case fields.Metadata != "":
		x.Value = &ExprArg_Metadata{Metadata: fields.Metadata}
	case fields.Var != "":
		x.Value = &ExprArg_Var{Var: fields.Var}
	case fields.U64 != 0:
		x.Value = &ExprArg_U64{U64: fields.U64}
	default:
		x.Value = &ExprArg_Bytes{Bytes: fields.Bytes}
	}
	return nil
}

// TCPItem is one step of a TCP sequence.
type TCPItem struct {
	DelayMin int64  `json:"delayMin,omitempty"`
	DelayMax int64  `json:"delayMax,omitempty"`
	Rand     int32  `json:"rand,omitempty"`
	RandMin  int32  `json:"randMin,omitempty"`
	RandMax  int32  `json:"randMax,omitempty"`
	Packet   []byte `json:"packet,omitempty"`
	Save     string `json:"save,omitempty"`
	Var      string `json:"var,omitempty"`
	Expr     *Expr  `json:"expr,omitempty"`
}

// TCPSequence is a sequence of steps.
type TCPSequence struct {
	Sequence []*TCPItem `json:"sequence,omitempty"`
}

// TCPConfig is the settings of the custom TCP header.
type TCPConfig struct {
	Clients []*TCPSequence `json:"clients,omitempty"`
	Servers []*TCPSequence `json:"servers,omitempty"`
	Errors  []*TCPSequence `json:"errors,omitempty"`
}

// UDPItem is one step of a UDP sequence.
type UDPItem struct {
	Rand    int32  `json:"rand,omitempty"`
	RandMin int32  `json:"randMin,omitempty"`
	RandMax int32  `json:"randMax,omitempty"`
	Packet  []byte `json:"packet,omitempty"`
	Save    string `json:"save,omitempty"`
	Var     string `json:"var,omitempty"`
	Expr    *Expr  `json:"expr,omitempty"`
}

// UDPConfig is the settings of the custom UDP header.
type UDPConfig struct {
	Client []*UDPItem `json:"client,omitempty"`
	Server []*UDPItem `json:"server,omitempty"`
}

// UDPStandaloneConfig is the settings of the custom UDP header on its own.
type UDPStandaloneConfig struct {
	Client []*UDPItem `json:"client,omitempty"`
	Server []*UDPItem `json:"server,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*TCPConfig) SettingsName() string {
	return "xray.transport.internet.finalmask.header.custom.TCPConfig"
}

func (*UDPConfig) SettingsName() string {
	return "xray.transport.internet.finalmask.header.custom.UDPConfig"
}

func (*UDPStandaloneConfig) SettingsName() string {
	return "xray.transport.internet.finalmask.header.custom.UDPStandaloneConfig"
}
