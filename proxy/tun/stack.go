package tun

import "time"

// StackOptions for the stack implementation
type StackOptions struct {
	Tun         Tun
	MTU         uint32
	IdleTimeout time.Duration
}
