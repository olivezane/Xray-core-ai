package protocol

import "google.golang.org/protobuf/encoding/protojson"

// A server endpoint is carried inside a settings value that is a plain Go
// struct, and encoding/json cannot write a protobuf message whose value is a
// oneof: the address of an endpoint is one, and it would be written as its
// wrapper type and could not be read back. The endpoint is therefore written the
// way protobuf writes JSON, which reads back into the oneof.

func (x *ServerEndpoint) MarshalJSON() ([]byte, error) {
	return protojson.Marshal(x)
}

func (x *ServerEndpoint) UnmarshalJSON(b []byte) error {
	return protojson.Unmarshal(b, x)
}
