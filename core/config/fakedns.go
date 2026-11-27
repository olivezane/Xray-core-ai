package config

import (
	"encoding/json"

	"github.com/xtls/xray-core/common/errors"
)

// FakeDNSPool is one fake DNS pool: the address range it hands out and how many
// mappings it keeps.
type FakeDNSPool struct {
	IPPool  string `json:"ipPool"`
	LRUSize int64  `json:"poolSize"`
}

// FakeDNS is the "fakedns" section. A configuration may write one pool or a list
// of them, so the field it is decoded into is kept unexported and reached through
// the accessors below.
type FakeDNS struct {
	pool  *FakeDNSPool
	pools []*FakeDNSPool
}

// MarshalJSON implements encoding/json.Marshaler.
func (f *FakeDNS) MarshalJSON() ([]byte, error) {
	if (f.pool != nil) != (f.pools != nil) {
		if f.pool != nil {
			return json.Marshal(f.pool)
		}
		return json.Marshal(f.pools)
	}
	return nil, errors.New("unexpected config state")
}

// UnmarshalJSON implements encoding/json.Unmarshaler.
func (f *FakeDNS) UnmarshalJSON(data []byte) error {
	var pool FakeDNSPool
	var pools []*FakeDNSPool
	switch {
	case json.Unmarshal(data, &pool) == nil:
		f.pool = &pool
	case json.Unmarshal(data, &pools) == nil:
		f.pools = pools
	default:
		return errors.New("invalid fakedns config")
	}
	return nil
}

// Pools returns the pools of the section, whichever form it was written in.
func (f *FakeDNS) Pools() []*FakeDNSPool {
	if f.pool != nil {
		return []*FakeDNSPool{f.pool}
	}
	return f.pools
}

// SetPools makes the section a list of pools.
func (f *FakeDNS) SetPools(pools []*FakeDNSPool) {
	f.pools = pools
	f.pool = nil
}

// SetPool makes the section one pool.
func (f *FakeDNS) SetPool(pool *FakeDNSPool) {
	f.pool = pool
	f.pools = nil
}

// Single returns the one pool of the section, and reports whether the section was
// written as a single pool rather than a list.
func (f *FakeDNS) Single() (*FakeDNSPool, bool) {
	return f.pool, f.pool != nil
}

// Listed returns the pools of a section written as a list, and reports whether it
// was written that way.
func (f *FakeDNS) Listed() ([]*FakeDNSPool, bool) {
	return f.pools, f.pools != nil
}
