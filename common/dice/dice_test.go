package dice_test

import (
	"math/rand"
	"testing"

	. "github.com/xtls/xray-core/common/dice"
)

func BenchmarkRoll1(b *testing.B) {
	for b.Loop() {
		Roll(1)
	}
}

func BenchmarkRoll20(b *testing.B) {
	for b.Loop() {
		Roll(20)
	}
}

func BenchmarkIntn1(b *testing.B) {
	for b.Loop() {
		rand.Intn(1) //nolint:staticcheck // SA4030: this benchmark measures exactly that degenerate call
	}
}

func BenchmarkIntn20(b *testing.B) {
	for b.Loop() {
		rand.Intn(20)
	}
}

func BenchmarkInt63(b *testing.B) {
	for b.Loop() {
		_ = uint16(rand.Int63() >> 47) //nolint:gosec // G115: the value is bounded by the fixture built above
	}
}

func BenchmarkInt31(b *testing.B) {
	for b.Loop() {
		_ = uint16(rand.Int31() >> 15) //nolint:gosec // G115: the value is bounded by the fixture built above
	}
}

func BenchmarkIntn(b *testing.B) {
	for b.Loop() {
		_ = uint16(rand.Intn(65536)) //nolint:gosec // G115: the value is bounded by the fixture built above
	}
}
