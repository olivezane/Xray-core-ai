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
	// 1000 次从 uint32 空间抽样,生日悖论碰撞概率约 1.2e-4
	// (1-exp(-C(1000,2)/2^32)),不会像 10000 次(约 1.2%)那样随机误报。
	seen := make(map[uint32]struct{}, 1000)
	for i := 0; i < 1000; i++ {
		id := uint32(NewID())
		if _, ok := seen[id]; ok {
			t.Fatalf("duplicate NewID() = %d at iteration %d", id, i)
		}
		seen[id] = struct{}{}
	}
}
