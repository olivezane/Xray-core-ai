package bbr

import (
	"testing"
)

func TestWindowedFilterMax(t *testing.T) {
	filter := NewWindowedFilter[int, int64](100, MaxFilter[int])
	filter.Update(10, 10)
	if filter.GetBest() != 10 {
		t.Fatalf("expected 10, got %d", filter.GetBest())
	}

	filter.Update(20, 20)
	if filter.GetBest() != 20 {
		t.Fatalf("expected 20, got %d", filter.GetBest())
	}

	// In the second quarter of window (window=100, quarter=25), at t=50:
	filter.Update(15, 50)
	if filter.GetBest() != 20 {
		t.Fatalf("expected 20, got %d", filter.GetBest())
	}

	// In the second half of window, at t=80:
	filter.Update(12, 80)
	if filter.GetBest() != 20 {
		t.Fatalf("expected 20, got %d", filter.GetBest())
	}

	// Now advance time past window length of sample 20 (20 + 100 = 120):
	filter.Update(5, 125)
	if filter.GetBest() != 15 {
		t.Fatalf("expected 15, got %d", filter.GetBest())
	}
}

func TestWindowedFilterMin(t *testing.T) {
	filter := NewWindowedFilter[int, int64](100, MinFilter[int])
	filter.Update(20, 10)
	if filter.GetBest() != 20 {
		t.Fatalf("expected 20, got %d", filter.GetBest())
	}

	filter.Update(10, 20)
	if filter.GetBest() != 10 {
		t.Fatalf("expected 10, got %d", filter.GetBest())
	}

	filter.Update(15, 50)
	if filter.GetBest() != 10 {
		t.Fatalf("expected 10, got %d", filter.GetBest())
	}

	filter.Update(18, 80)
	if filter.GetBest() != 10 {
		t.Fatalf("expected 10, got %d", filter.GetBest())
	}

	// Now advance time past window length of sample 10 (20 + 100 = 120):
	filter.Update(25, 125)
	if filter.GetBest() != 15 {
		t.Fatalf("expected 15, got %d", filter.GetBest())
	}
}
