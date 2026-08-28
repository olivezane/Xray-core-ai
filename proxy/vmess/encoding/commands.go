package encoding

import (
	"io"

	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/protocol"
)

var ErrUnknownCommand = errors.New("Unknown command.")

// All response command types were removed in
// "Outbound: One endpoint and at most one user only (#5144)";
// restore the command switch here when new commands are added.
func MarshalCommand(command interface{}, writer io.Writer) error {
	return ErrUnknownCommand
}

func UnmarshalCommand(cmdID byte, data []byte) (protocol.ResponseCommand, error) {
	return nil, ErrUnknownCommand
}
