package tun

import (
	"context"
	stdnet "net"
	"strings"
	"testing"

	"github.com/xtls/xray-core/common/net"
)

// stubConnectionHandler only satisfies ConnectionHandler; the stack selection
// seam never delivers a connection through it.
type stubConnectionHandler struct{}

func (stubConnectionHandler) HandleConnection(stdnet.Conn, net.Destination) {}

func TestSelectStackCanonicalizesTheSelection(t *testing.T) {
	testCases := []struct {
		stack      string
		congestion string
		want       StackSelection
	}{
		{stack: "", want: StackSelection{Stack: StackGVisor}},
		{stack: "gvisor", want: StackSelection{Stack: StackGVisor}},
		{stack: " GVisor ", want: StackSelection{Stack: StackGVisor}},
		{stack: "mipstack", want: StackSelection{Stack: StackMipstack, TCPCongestion: "cubic"}},
		{stack: "mipstack", congestion: "bbr", want: StackSelection{Stack: StackMipstack, TCPCongestion: "bbr"}},
		{stack: "MIPSTACK", congestion: " BBR3 ", want: StackSelection{Stack: StackMipstack, TCPCongestion: "bbr3"}},
		{stack: " mipstack ", congestion: "reno", want: StackSelection{Stack: StackMipstack, TCPCongestion: "reno"}},
	}

	for _, testCase := range testCases {
		got, err := SelectStack(testCase.stack, testCase.congestion)
		if err != nil {
			t.Fatalf("SelectStack(%q, %q) failed: %v", testCase.stack, testCase.congestion, err)
		}
		if got != testCase.want {
			t.Fatalf("SelectStack(%q, %q) = %+v, want %+v", testCase.stack, testCase.congestion, got, testCase.want)
		}
	}
}

func TestSelectStackRejectsAnInvalidSelection(t *testing.T) {
	testCases := []struct {
		stack      string
		congestion string
	}{
		{stack: "tun2socks"},
		{stack: "gvisor", congestion: "bbr"},
		{congestion: "cubic"},
		{stack: "mipstack", congestion: "vegas"},
		// the short name is no stack name: this repository already uses it for a
		// CPU architecture
		{stack: "mips"},
	}

	for _, testCase := range testCases {
		if _, err := SelectStack(testCase.stack, testCase.congestion); err == nil {
			t.Fatalf("SelectStack(%q, %q) must fail", testCase.stack, testCase.congestion)
		}
	}
}

func TestNewStackRejectsAnUnknownStack(t *testing.T) {
	_, err := NewStack(context.Background(), StackOptions{
		Tun:   &gvisorFakeTun{},
		Stack: "tun2socks",
	}, stubConnectionHandler{})
	if err == nil {
		t.Fatal("an unknown stack must not fall back to another stack")
	}
	if !strings.Contains(err.Error(), "tun2socks") {
		t.Fatalf("error must name the unknown stack, got %v", err)
	}
}

func TestNewStackSelectsTheMipstackStack(t *testing.T) {
	for _, configured := range []string{"mipstack", "MIPSTACK", " mipstack "} {
		t.Run("stack="+configured, func(t *testing.T) {
			stack, err := NewStack(context.Background(), StackOptions{
				Tun:   newBridgeDevice(),
				Stack: configured,
			}, stubConnectionHandler{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			defer stack.Close()

			if _, ok := stack.(*stackMipstack); !ok {
				t.Fatalf("selected %T, want the MIPS Stack", stack)
			}
		})
	}
}

func TestNewStackRejectsACongestionControlTheStackCannotUse(t *testing.T) {
	_, err := NewStack(context.Background(), StackOptions{
		Tun:           &gvisorFakeTun{},
		Stack:         StackGVisor,
		TCPCongestion: "bbr3",
	}, stubConnectionHandler{})
	if err == nil {
		t.Fatal("the gVisor Stack must not accept a TCP congestion control it ignores")
	}
}

func TestNewStackSelectsTheGVisorStackByDefault(t *testing.T) {
	for _, configured := range []string{"", "gvisor", "GVisor", " gvisor "} {
		t.Run("stack="+configured, func(t *testing.T) {
			stack, err := NewStack(context.Background(), StackOptions{
				Tun:   &gvisorFakeTun{},
				Stack: configured,
			}, stubConnectionHandler{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			defer stack.Close()

			if _, ok := stack.(*stackGVisor); !ok {
				t.Fatalf("selected %T, want the gVisor Stack", stack)
			}
		})
	}
}
