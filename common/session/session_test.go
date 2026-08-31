package session

import (
	"testing"
)

func TestNewIDNeverZero(t *testing.T) {
	for i := 0; i < 10000; i++ {
		if got := NewID(); got == 0 {
			t.Fatalf("NewID() = 0 at iteration %d, want non-zero", i)
		}
	}
}

func TestNewIDUniqueness(t *testing.T) {
	// 10000 draws from uint32 space: collision probability ~ 1e-5.
	seen := make(map[uint32]struct{}, 10000)
	for i := 0; i < 10000; i++ {
		id := uint32(NewID())
		if _, ok := seen[id]; ok {
			t.Fatalf("duplicate NewID() = %d at iteration %d", id, i)
		}
		seen[id] = struct{}{}
	}
}
