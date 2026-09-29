package bytespool

import "sync"

func createAllocFunc(size int32) func() any {
	return func() any {
		return make([]byte, size)
	}
}

// The following parameters controls the size of buffer pools.
// There are numPools pools. Starting from 2k size, the size of each pool is sizeMulti of the previous one.
// Package buf is guaranteed to not use buffers larger than the largest pool.
// Other packets may use larger buffers.
const (
	numPools  = 4
	sizeMulti = 4
)

var (
	pool     [numPools]sync.Pool
	poolSize [numPools]int32
)

func init() {
	size := int32(2048)
	for i := range numPools {
		pool[i] = sync.Pool{
			New: createAllocFunc(size),
		}
		poolSize[i] = size
		size *= sizeMulti
	}
}

// GetPool returns a sync.Pool that generates bytes array with at least the given size.
// It may return nil if no such pool exists.
//
// xray:api:stable
func GetPool(size int32) *sync.Pool {
	for idx, ps := range poolSize {
		if size <= ps {
			return &pool[idx]
		}
	}
	return nil
}

// Alloc returns a byte slice with at least the given size. Minimum size of returned slice is 2048.
//
// xray:api:stable
func Alloc(size int32) []byte {
	pool := GetPool(size)
	if pool != nil {
		//nolint:forcetypeassert // the pool only ever stores []byte
		return pool.Get().([]byte)
	}
	return make([]byte, size)
}

// Free puts a byte slice into the internal pool.
//
// xray:api:stable
func Free(b []byte) {
	size := int32(cap(b)) //nolint:gosec // pool buffers are bounded by the pool's size classes
	b = b[0:cap(b)]
	for i := numPools - 1; i >= 0; i-- {
		if size >= poolSize[i] {
			pool[i].Put(b) //nolint:staticcheck // SA6002: the pool is typed []byte by its exported API; switching to *[]byte is a benchmark-first change
			return
		}
	}
}
