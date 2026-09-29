package stats

import "sync/atomic"

// Counter is an implementation of stats.Counter.
type Counter struct {
	value atomic.Int64
}

// Value implements stats.Counter.
func (c *Counter) Value() int64 {
	return c.value.Load()
}

// Set implements stats.Counter.
func (c *Counter) Set(newValue int64) int64 {
	return c.value.Swap(newValue)
}

// Add implements stats.Counter.
func (c *Counter) Add(delta int64) int64 {
	return c.value.Add(delta)
}
