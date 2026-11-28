package net

import "google.golang.org/protobuf/encoding/protojson"

// An address is carried inside settings that are plain Go values, and
// encoding/json cannot write a protobuf message whose value is a oneof: the
// address itself is one, and it would be written as its wrapper type and could
// not be read back. The address is therefore written the way protobuf writes
// JSON, which reads back into the oneof.

func (x *IPOrDomain) MarshalJSON() ([]byte, error) {
	return protojson.Marshal(x)
}

func (x *IPOrDomain) UnmarshalJSON(b []byte) error {
	return protojson.Unmarshal(b, x)
}
