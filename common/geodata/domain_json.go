package geodata

import "google.golang.org/protobuf/encoding/protojson"

// A domain rule is carried inside settings that are plain Go values, and
// encoding/json cannot write a protobuf message whose value is a oneof: the rule
// itself is one, and it would be written as its wrapper type and could not be
// read back. The rule is therefore written the way protobuf writes JSON, which
// reads back into the oneof.

func (x *DomainRule) MarshalJSON() ([]byte, error) {
	return protojson.Marshal(x)
}

func (x *DomainRule) UnmarshalJSON(b []byte) error {
	return protojson.Unmarshal(b, x)
}
