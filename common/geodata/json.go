package geodata

import "google.golang.org/protobuf/encoding/protojson"

// An IP rule is carried inside a settings value that is a plain Go struct, and
// encoding/json cannot write a protobuf message whose value is a oneof: it would
// write the wrapper type and could not read it back. The rule is therefore
// written the way protobuf writes JSON, which reads back into the oneof.

func (x *IPRule) MarshalJSON() ([]byte, error) {
	return protojson.Marshal(x)
}

func (x *IPRule) UnmarshalJSON(b []byte) error {
	return protojson.Unmarshal(b, x)
}
