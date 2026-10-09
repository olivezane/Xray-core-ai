package tun

import (
	"context"
	"strings"
	"time"

	"github.com/xtls/xray-core/common/errors"
)

const (
	// StackGVisor is the gVisor Stack, selected when the configuration names none
	StackGVisor = "gvisor"
	// StackMipstack is the MIPS Stack, the pure-Go user-space IP stack of
	// mihomo. The short name "mips" is not accepted: it already means a CPU
	// architecture in this repository.
	StackMipstack = "mipstack"
)

// Stack interface implement ip protocol stack, bridging raw network packets and data streams
type Stack interface {
	Start() error
	Close() error
}

// StackOptions describes the stack a TUN inbound asks for
type StackOptions struct {
	Tun Tun
	MTU uint32
	// IdleTimeout is the connection idle timeout of the user level
	IdleTimeout time.Duration
	// Stack names the stack to build. Empty selects the gVisor Stack.
	Stack string
	// TCPCongestion names the TCP congestion control of the MIPS Stack
	TCPCongestion string
	// Gateway holds the interface address prefixes of the device, which are
	// the Local Addresses of the MIPS Stack
	Gateway []string
}

// StackSelection is a validated stack name and TCP congestion control
type StackSelection struct {
	Stack         string
	TCPCongestion string
}

// SelectStack validates and canonicalizes a configured stack name and TCP
// congestion control
func SelectStack(stack string, congestion string) (StackSelection, error) {
	switch strings.ToLower(strings.TrimSpace(stack)) {
	case "", StackGVisor:
		if strings.TrimSpace(congestion) != "" {
			return StackSelection{}, errors.New("tcp congestion control is only supported by the ", StackMipstack, " stack")
		}
		return StackSelection{Stack: StackGVisor}, nil
	case StackMipstack:
		control, err := normalizeTCPCongestion(congestion)
		if err != nil {
			return StackSelection{}, err
		}
		return StackSelection{Stack: StackMipstack, TCPCongestion: control}, nil
	default:
		return StackSelection{}, errors.New("unknown tun stack: ", stack)
	}
}

// NewStack builds the stack named by options.Stack
func NewStack(ctx context.Context, options StackOptions, handler ConnectionHandler) (Stack, error) {
	selection, err := SelectStack(options.Stack, options.TCPCongestion)
	if err != nil {
		return nil, err
	}
	options.Stack = selection.Stack
	options.TCPCongestion = selection.TCPCongestion

	switch selection.Stack {
	case StackGVisor:
		return newGVisorStack(ctx, options, handler)
	case StackMipstack:
		return newMipstack(ctx, options, handler)
	default:
		return nil, errors.New("unknown tun stack: ", options.Stack)
	}
}
