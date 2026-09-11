package strmatcher

import (
	"cmp"
	"math/bits"
	"runtime"
	"slices"
	"strings"
	"unsafe"
)

// PrimeRK is the prime base used in Rabin-Karp algorithm.
const PrimeRK = 16777619

// RollingHash calculates the rolling murmurHash of given string based on a provided suffix hash.
func RollingHash(hash uint32, input string) uint32 {
	for i := len(input) - 1; i >= 0; i-- {
		hash = hash*PrimeRK + uint32(input[i])
	}
	return hash
}

// MemHash is the hash function used by go map, it utilizes available hardware instructions(behaves
// as aeshash if aes instruction is available).
// With different seed, each MemHash<seed> performs as distinct hash functions.
func MemHash(seed uint32, input string) uint32 {
	return uint32(strhash(unsafe.Pointer(&input), uintptr(seed))) // nosemgrep
}

const (
	mphMatchTypeCount = 2 // Full and Domain
)

// mphRuleIdx is the value stored in ruleInfos during building.
// It is the rule index into rules/values/rollingHashes. Keeping the map
// value small (4 bytes instead of a struct with two slice headers and a
// hash) reduces peak memory by tens of MB for large geosite rule sets.
type mphRuleIdx uint32

type MphMatcherGroup struct {
	rules      []string // RuleIdx -> pattern string, index 0 reserved for failed lookup
	values     []uint32 // Flat matcher values, grouped by rule index
	valueOff   []uint32 // RuleIdx -> start offset in values; len is len(rules)+1
	level0     []uint32 // RollingHash & Mask -> seed for Memhash
	level0Mask uint32   // Mask restricting RollingHash to 0 ~ len(level0)
	level1     []uint32 // Memhash<seed> & Mask -> stored index for rules
	level1Mask uint32   // Mask for restricting Memhash<seed> to 0 ~ len(level1)
	// Only used for building, destroyed after build completes.
	rollingHashes []uint32      // RuleIdx -> rollingHash, parallel to rules
	ruleValues    [2][][]uint32 // [matcherType]RuleIdx -> values; Full segment precedes Domain, matching MatcherGroup semantics
	ruleInfos     *map[string]mphRuleIdx
}

func NewMphMatcherGroup() *MphMatcherGroup {
	return &MphMatcherGroup{
		rules:         []string{""},
		values:        nil,
		valueOff:      []uint32{0},
		level0:        nil,
		level0Mask:    0,
		level1:        nil,
		level1Mask:    0,
		ruleValues:    [2][][]uint32{{nil}, {nil}},
		rollingHashes: []uint32{0},
		ruleInfos:     &map[string]mphRuleIdx{}, // Only used for building, destroyed after build complete
	}
}

// AddFullMatcher implements MatcherGroupForFull.
func (g *MphMatcherGroup) AddFullMatcher(matcher FullMatcher, value uint32) {
	pattern := strings.ToLower(matcher.Pattern())
	g.addPattern(0, "", pattern, matcher.Type(), value)
}

// AddDomainMatcher implements MatcherGroupForDomain.
func (g *MphMatcherGroup) AddDomainMatcher(matcher DomainMatcher, value uint32) {
	pattern := strings.ToLower(matcher.Pattern())
	hash := g.addPattern(0, "", pattern, matcher.Type(), value) // For full domain match
	g.addPattern(hash, pattern, ".", matcher.Type(), value)     // For partial domain match
}

func (g *MphMatcherGroup) addPattern(suffixHash uint32, suffixPattern string, pattern string, matcherType Type, value uint32) uint32 {
	fullPattern := pattern + suffixPattern
	idx, found := (*g.ruleInfos)[fullPattern]
	if !found {
		idx = mphRuleIdx(len(g.rules))
		g.rules = append(g.rules, fullPattern)
		g.ruleValues[0] = append(g.ruleValues[0], nil)
		g.ruleValues[1] = append(g.ruleValues[1], nil)
		g.rollingHashes = append(g.rollingHashes, RollingHash(suffixHash, pattern))
		(*g.ruleInfos)[fullPattern] = idx
	}
	// Values are kept in two segments (Full before Domain) so that the
	// concatenation order matches the previous matchers[Full]+matchers[Domain].
	g.ruleValues[matcherType][idx] = append(g.ruleValues[matcherType][idx], value)
	return g.rollingHashes[idx]
}

// Build builds a minimal perfect hash table for insert rules.
// Algorithm used: Hash, displace, and compress. See http://cmph.sourceforge.net/papers/esa09.pdf
func (g *MphMatcherGroup) Build() error {
	ruleCount := len(*g.ruleInfos)
	g.level0 = make([]uint32, nextPow2(ruleCount/4))
	g.level0Mask = uint32(len(g.level0) - 1)
	g.level1 = make([]uint32, nextPow2(ruleCount))
	g.level1Mask = uint32(len(g.level1) - 1)

	// Create buckets based on all rule's rolling hash
	buckets := make([][]uint32, len(g.level0))
	for ruleIdx := 1; ruleIdx < len(g.rules); ruleIdx++ { // Traverse rules starting from index 1 (0 reserved for failed lookup)
		bucketIdx := g.rollingHashes[ruleIdx] & g.level0Mask
		buckets[bucketIdx] = append(buckets[bucketIdx], uint32(ruleIdx))
	}

	// Flatten per-rule values into a single backing array with offset table.
	// This replaces the previous [][]uint32 (one slice header per rule, ~24B each
	// for 300k+ geosite rules) with 2x4B per rule, cutting steady-state heap
	// for the routing matcher by about 10MB.
	g.values = make([]uint32, 0, ruleCount)
	g.valueOff = make([]uint32, len(g.rules)+1)
	for ruleIdx := 1; ruleIdx < len(g.rules); ruleIdx++ {
		g.valueOff[ruleIdx] = uint32(len(g.values))
		g.values = append(g.values, g.ruleValues[Full][ruleIdx]...)
		g.values = append(g.values, g.ruleValues[Domain][ruleIdx]...)
	}
	g.valueOff[len(g.rules)] = uint32(len(g.values))
	g.ruleValues = [2][][]uint32{}

	g.ruleInfos = nil // Set ruleInfos nil to release memory
	g.rollingHashes = nil
	runtime.GC() // peak mem

	// Sort buckets in descending order with respect to each bucket's size
	bucketIdxs := make([]int, len(buckets))
	for bucketIdx := range buckets {
		bucketIdxs[bucketIdx] = bucketIdx
	}
	slices.SortFunc(bucketIdxs, func(i, j int) int {
		return cmp.Compare(len(buckets[j]), len(buckets[i]))
	})

	// Exercise Hash, Displace, and Compress algorithm to construct minimal perfect hash table
	occupied := make([]bool, len(g.level1)) // Whether a second-level hash has been already used
	hashedBucket := make([]uint32, 0, 4)    // Second-level hashes for each rule in a specific bucket
	for _, bucketIdx := range bucketIdxs {
		bucket := buckets[bucketIdx]
		hashedBucket = hashedBucket[:0]
		seed := uint32(0)
		for len(hashedBucket) != len(bucket) {
			for _, ruleIdx := range bucket {
				memHash := MemHash(seed, g.rules[ruleIdx]) & g.level1Mask
				if occupied[memHash] { // Collision occurred with this seed
					for _, hash := range hashedBucket { // Revert all values in this hashed bucket
						occupied[hash] = false
						g.level1[hash] = 0
					}
					hashedBucket = hashedBucket[:0]
					seed++ // Try next seed
					break
				}
				occupied[memHash] = true
				g.level1[memHash] = ruleIdx // The final value in the hash table
				hashedBucket = append(hashedBucket, memHash)
			}
		}
		g.level0[bucketIdx] = seed // Displacement value for this bucket
	}
	return nil
}

// Lookup searches for input in minimal perfect hash table and returns its index. 0 indicates not found.
func (g *MphMatcherGroup) Lookup(rollingHash uint32, input string) uint32 {
	i0 := rollingHash & g.level0Mask
	seed := g.level0[i0]
	i1 := MemHash(seed, input) & g.level1Mask
	if n := g.level1[i1]; g.rules[n] == input {
		return n
	}
	return 0
}

// Match implements MatcherGroup.Match.
func (g *MphMatcherGroup) Match(input string) []uint32 {
	matches := make([][]uint32, 0, 5)
	hash := uint32(0)
	for i := len(input) - 1; i >= 0; i-- {
		hash = hash*PrimeRK + uint32(input[i])
		if input[i] == '.' {
			if mphIdx := g.Lookup(hash, input[i:]); mphIdx != 0 {
				matches = append(matches, g.valueRange(mphIdx))
			}
		}
	}
	if mphIdx := g.Lookup(hash, input); mphIdx != 0 {
		matches = append(matches, g.valueRange(mphIdx))
	}
	return CompositeMatchesReverse(matches)
}

// valueRange returns the matcher values registered for the given rule index.
func (g *MphMatcherGroup) valueRange(ruleIdx uint32) []uint32 {
	return g.values[g.valueOff[ruleIdx]:g.valueOff[ruleIdx+1]]
}

// MatchAny implements MatcherGroup.MatchAny.
func (g *MphMatcherGroup) MatchAny(input string) bool {
	hash := uint32(0)
	for i := len(input) - 1; i >= 0; i-- {
		hash = hash*PrimeRK + uint32(input[i])
		if input[i] == '.' {
			if g.Lookup(hash, input[i:]) != 0 {
				return true
			}
		}
	}
	return g.Lookup(hash, input) != 0
}

func nextPow2(v int) int {
	if v <= 1 {
		return 1
	}
	const MaxUInt = ^uint(0)
	n := (MaxUInt >> bits.LeadingZeros(uint(v))) + 1
	return int(n)
}

//go:noescape
//go:linkname strhash runtime.strhash
func strhash(p unsafe.Pointer, h uintptr) uintptr
