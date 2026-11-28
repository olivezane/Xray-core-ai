// Code that is not generated: the settings of this package are plain Go values,
// and they say the name a configuration written for the API calls them.

package noise

import (
	"fmt"
)

// Segment_Kind is one of the values these settings take.
type Segment_Kind int32

const (
	Segment_BYTES Segment_Kind = 0
	Segment_RANDOM Segment_Kind = 1
	Segment_RANDOM_ASCII Segment_Kind = 2
	Segment_RANDOM_DIGIT Segment_Kind = 3
	Segment_TIMESTAMP Segment_Kind = 4
	Segment_COUNTER Segment_Kind = 5
	Segment_NONCE Segment_Kind = 6
)

func (x Segment_Kind) String() string {
	switch x {
	case Segment_BYTES:
		return "Segment_BYTES"
	case Segment_RANDOM:
		return "Segment_RANDOM"
	case Segment_RANDOM_ASCII:
		return "Segment_RANDOM_ASCII"
	case Segment_RANDOM_DIGIT:
		return "Segment_RANDOM_DIGIT"
	case Segment_TIMESTAMP:
		return "Segment_TIMESTAMP"
	case Segment_COUNTER:
		return "Segment_COUNTER"
	case Segment_NONCE:
		return "Segment_NONCE"
	default:
		return fmt.Sprint(int32(x))
	}
}

// Segment is settings this package reads.
type Segment struct {
	Kind Segment_Kind `json:"kind,omitempty"`
	Bytes []byte `json:"bytes,omitempty"`
	MinSize int64 `json:"minSize,omitempty"`
	MaxSize int64 `json:"maxSize,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Segment) SettingsName() string {
	return "xray.transport.internet.finalmask.noise.Segment"
}

// Item is settings this package reads.
type Item struct {
	RandMin int64 `json:"randMin,omitempty"`
	RandMax int64 `json:"randMax,omitempty"`
	RandRangeMin int32 `json:"randRangeMin,omitempty"`
	RandRangeMax int32 `json:"randRangeMax,omitempty"`
	Packet []byte `json:"packet,omitempty"`
	DelayMin int64 `json:"delayMin,omitempty"`
	DelayMax int64 `json:"delayMax,omitempty"`
	Segments []*Segment `json:"segments,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Item) SettingsName() string {
	return "xray.transport.internet.finalmask.noise.Item"
}

// Config is settings this package reads.
type Config struct {
	ResetMin int64 `json:"resetMin,omitempty"`
	ResetMax int64 `json:"resetMax,omitempty"`
	Items []*Item `json:"items,omitempty"`
	Script string `json:"script,omitempty"`
}

// SettingsName is what a configuration written for the API calls these settings.
func (*Config) SettingsName() string {
	return "xray.transport.internet.finalmask.noise.Config"
}
