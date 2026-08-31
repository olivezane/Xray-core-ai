package dice_test

import (
	"testing"

	. "github.com/xtls/xray-core/common/dice"
)

func TestRollRange(t *testing.T) {
	if got := Roll(1); got != 0 {
		t.Fatalf("Roll(1) = %d, want 0", got)
	}
	for i := 0; i < 1000; i++ {
		if got := Roll(10); got < 0 || got >= 10 {
			t.Fatalf("Roll(10) = %d, want in [0,10)", got)
		}
	}
}

func TestRollInt63nRange(t *testing.T) {
	if got := RollInt63n(1); got != 0 {
		t.Fatalf("RollInt63n(1) = %d, want 0", got)
	}
	const n = int64(1 << 40)
	for i := 0; i < 1000; i++ {
		if got := RollInt63n(n); got < 0 || got >= n {
			t.Fatalf("RollInt63n(%d) = %d, want in [0,%d)", n, got, n)
		}
	}
}

func TestRollDeterministicSameSeed(t *testing.T) {
	for seed := int64(0); seed < 20; seed++ {
		got1 := RollDeterministic(1000, seed)
		got2 := RollDeterministic(1000, seed)
		if got1 != got2 {
			t.Fatalf("seed %d: RollDeterministic returned %d then %d, want same", seed, got1, got2)
		}
		if got1 < 0 || got1 >= 1000 {
			t.Fatalf("seed %d: RollDeterministic = %d, want in [0,1000)", seed, got1)
		}
	}
}

func TestNewDeterministicDiceSameSeed(t *testing.T) {
	d1 := NewDeterministicDice(42)
	d2 := NewDeterministicDice(42)
	for i := 0; i < 1000; i++ {
		a, b := d1.Roll(1000), d2.Roll(1000)
		if a != b {
			t.Fatalf("iteration %d: %d != %d, want same sequence for same seed", i, a, b)
		}
		if a < 0 || a >= 1000 {
			t.Fatalf("iteration %d: Roll = %d, want in [0,1000)", i, a)
		}
	}
}

func TestRollUint16AndUint64(t *testing.T) {
	for i := 0; i < 1000; i++ {
		RollUint16() // must not panic; type guarantees range
		RollUint64()
	}
}
