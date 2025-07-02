package vars

import (
	"fmt"
	"github.com/cespare/xxhash/v2"
	"github.com/zeusync/zeusync/internal/core/syncv2"
	"github.com/zeusync/zeusync/pkg/sequence"
	"math/rand"
	"sort"
	sc "sync"
	"sync/atomic"
)

const defaultShardKey = "default"

var defaultRanges = []string{"a", "h", "o", "v"}

// HashSharded implements ShardedRoot using hash-based sharding
type HashSharded[T any] struct {
	shards   []shard[T]
	count    int
	version  atomic.Uint64
	dirty    atomic.Bool
	hashFunc func(string) uint32
}

// shard represents a single shard containing a value and its own mutex
type shard[T any] struct {
	value   atomic.Value // stores T
	version atomic.Uint64
	mx      sc.RWMutex
}

// newShard creates a new shard with the given value
func newShard[T any](value T) *shard[T] {
	s := shard[T]{
		mx: sc.RWMutex{},
	}
	s.value.Store(value)
	s.version.Store(1)
	return &s
}

var _ sync.ShardedRoot[string] = (*HashSharded[string])(nil)

// NewHashSharded creates a new HashSharded with the given number of shards
func NewHashSharded[T any](shardCount int, initialValue T) *HashSharded[T] {
	if shardCount <= 0 {
		shardCount = 16 // Default shard count
	}

	h := &HashSharded[T]{
		shards: make([]shard[T], shardCount),
		count:  shardCount,
		hashFunc: func(key string) uint32 {
			return xHash32(key)
		},
	}

	// Initialize all shards with the initial value
	for i := range h.shards {
		h.shards[i].value.Store(initialValue)
		h.shards[i].version.Store(1)
	}

	h.version.Store(1)
	h.dirty.Store(false)
	return h
}

// getShardIndex returns the shard index for a given key
func (h *HashSharded[T]) getShardIndex(shardKey string) int {
	return int(h.hashFunc(shardKey) % uint32(h.count))
}

// Get returns the value from the first shard (default behavior)
func (h *HashSharded[T]) Get() T {
	return h.GetShard(defaultShardKey)
}

// Set sets the value in the first shard (default behavior)
func (h *HashSharded[T]) Set(value T) {
	h.SetShard(defaultShardKey, value)
}

// GetShard returns the value from a specific shard
func (h *HashSharded[T]) GetShard(shardKey string) T {
	idx := h.getShardIndex(shardKey)
	sh := &h.shards[idx]

	sh.mx.RLock()
	defer sh.mx.RUnlock()

	return sh.value.Load().(T)
}

// SetShard sets the value in a specific shard
func (h *HashSharded[T]) SetShard(shardKey string, value T) {
	idx := h.getShardIndex(shardKey)
	sh := &h.shards[idx]

	sh.mx.Lock()
	defer sh.mx.Unlock()

	sh.value.Store(value)
	sh.version.Add(1)
	h.version.Add(1)
	h.dirty.Store(true)
}

// GetShardCount returns the number of shards
func (h *HashSharded[T]) GetShardCount() int {
	return h.count
}

// ForEachShard iterates over all shards
func (h *HashSharded[T]) ForEachShard(action func(shardIndex int, value T) bool) {
	for i := range h.shards {
		sh := &h.shards[i]
		sh.mx.RLock()
		value := sh.value.Load().(T)
		sh.mx.RUnlock()

		if !action(i, value) {
			break
		}
	}
}

// MergeShard merges values from all shards using the provided function
func (h *HashSharded[T]) MergeShard(merge func(values []T) T) T {
	values := make([]T, h.count)
	for i := range h.shards {
		sh := &h.shards[i]
		sh.mx.RLock()
		values[i] = sh.value.Load().(T)
		sh.mx.RUnlock()
	}
	return merge(values)
}

// Version returns the current version number
func (h *HashSharded[T]) Version() uint64 {
	return h.version.Load()
}

// IsDirty returns true if any shard has been modified since last clean
func (h *HashSharded[T]) IsDirty() bool {
	return h.dirty.Load()
}

// MarkClean marks all shards as clean
func (h *HashSharded[T]) MarkClean() {
	h.dirty.Store(false)
}

var _ sync.ShardedRoot[string] = (*ConsistentHashSharded[string])(nil)

// ConsistentHashSharded implements ShardedRoot using proper consistent hashing
type ConsistentHashSharded[T any] struct {
	ring         *consistentHashRing
	shards       []*shard[T]    // Fixed array of actual shards
	virtualNodes map[uint64]int // Maps virtual node hash to shard index
	count        int
	version      atomic.Uint64
	dirty        atomic.Bool
	mx           sc.RWMutex
}

// NewConsistentHashSharded creates a new NewConsistentHashSharded
func NewConsistentHashSharded[T any](shardCount int, initialValue T) *ConsistentHashSharded[T] {
	if shardCount < 0 {
		shardCount = 16
	}

	c := &ConsistentHashSharded[T]{
		ring:         newConsistentHashRing(),
		shards:       make([]*shard[T], shardCount),
		virtualNodes: make(map[uint64]int),
		count:        shardCount,
	}

	// Initialize actual shards
	for i := 0; i < shardCount; i++ {
		c.shards[i] = newShard(initialValue)
	}

	// Create virtual nodes for better distribution
	virtualNodesPerShard := shardCount * 2 // Number of virtual nodes per shard
	for shardIdx := 0; shardIdx < shardCount; shardIdx++ {
		for v := 0; v < virtualNodesPerShard; v++ {
			key := fmt.Sprintf("shard-%d-virtual-%d-%d", shardIdx, v, rand.Intn(100000))
			hash := xxhash.Sum64String(key)

			for _, exists := c.virtualNodes[hash]; exists; {
				key += "!"
				hash = xxhash.Sum64String(key)
			}

			c.ring.addNode(hash)
			c.virtualNodes[hash] = shardIdx
		}
	}

	c.version.Store(1)
	c.dirty.Store(false)
	return c
}

// Get returns the value from the default shard
func (c *ConsistentHashSharded[T]) Get() T {
	return c.GetShard(defaultShardKey)
}

// Set sets the value in the default shard
func (c *ConsistentHashSharded[T]) Set(value T) {
	c.SetShard(defaultShardKey, value)
}

// GetShard returns the value from a specific shard
func (c *ConsistentHashSharded[T]) GetShard(shardKey string) T {
	sh := c.getShardForKey(shardKey)

	sh.mx.RLock()
	defer sh.mx.RUnlock()
	return sh.value.Load().(T)
}

// SetShard sets the value in a specific shard
func (c *ConsistentHashSharded[T]) SetShard(shardKey string, value T) {
	sh := c.getShardForKey(shardKey)

	sh.mx.Lock()
	defer sh.mx.Unlock()

	sh.value.Store(value)
	sh.version.Add(1)
	c.version.Add(1)
	c.dirty.Store(true)
}

// GetShardCount returns the number of actual shards
func (c *ConsistentHashSharded[T]) GetShardCount() int {
	return c.count
}

// ForEachShard iterates over all shards
func (c *ConsistentHashSharded[T]) ForEachShard(action func(shardIndex int, value T) bool) {
	c.mx.RLock()
	defer c.mx.RUnlock()

	for i, sh := range c.shards {
		sh.mx.RLock()
		value := sh.value.Load().(T)
		sh.mx.RUnlock()

		if !action(i, value) {
			break
		}
	}
}

// MergeShard merges values from all shards using the provided function
func (c *ConsistentHashSharded[T]) MergeShard(merge func(values []T) T) T {
	c.mx.RLock()
	defer c.mx.RUnlock()

	values := make([]T, len(c.shards))

	for i, sh := range c.shards {
		sh.mx.RLock()
		values[i] = sh.value.Load().(T)
		sh.mx.RUnlock()
	}

	return merge(values)
}

// Version returns the current version number
func (c *ConsistentHashSharded[T]) Version() uint64 {
	return c.version.Load()
}

// IsDirty returns true if any shard has been modified since last clean
func (c *ConsistentHashSharded[T]) IsDirty() bool {
	return c.dirty.Load()
}

// MarkClean marks all shards as clean
func (c *ConsistentHashSharded[T]) MarkClean() {
	c.dirty.Store(false)
}

// getShardForKey returns the shard for a given key
func (c *ConsistentHashSharded[T]) getShardForKey(key string) *shard[T] {
	hash := xHash(key)
	nodeHash := c.ring.getNode(hash)
	shardIndex := c.virtualNodes[nodeHash]
	return c.shards[shardIndex]
}

// consistentHashRing implements a simple consistent hash ring
type consistentHashRing struct {
	nodes []uint64
	mx    sc.RWMutex
}

// newConsistentHashRing creates a new consistent hash ring
func newConsistentHashRing() *consistentHashRing {
	return &consistentHashRing{
		nodes: make([]uint64, 0),
	}
}

// addNode adds a node to the ring
func (r *consistentHashRing) addNode(hash uint64) {
	r.mx.Lock()
	defer r.mx.Unlock()

	r.nodes = append(r.nodes, hash)
	sequence.From(r.nodes).Sort(func(a, b uint64) bool { return a < b })
}

// getNode returns the node responsible for a given key
func (r *consistentHashRing) getNode(keyHash uint64) uint64 {
	r.mx.RLock()
	defer r.mx.RUnlock()

	if len(r.nodes) == 0 {
		return 0
	}

	idx := sort.Search(len(r.nodes), func(i int) bool {
		return r.nodes[i] >= keyHash
	})

	if idx == len(r.nodes) {
		return r.nodes[0]
	}

	return r.nodes[idx]
}

// xHash is a custom hash function for consistent hashing
func xHash(key string) uint64 {
	return xxhash.Sum64String(key)
}

func xHash32(key string) uint32 {
	return uint32(xxhash.Sum64String(key))
}

var _ sync.ShardedRoot[string] = (*RangeSharded[string])(nil)

// RangeSharded implements ShardedRoot using range-based sharding
type RangeSharded[T any] struct {
	shards    []shard[T]
	count     int
	version   atomic.Uint64
	dirty     atomic.Bool
	keyRanges []string // Sorted list of range boundaries
	mx        sc.RWMutex
}

// NewRangeSharded creates a new RangeSharded with predefined ranges
func NewRangeSharded[T any](ranges []string, initialValue T) *RangeSharded[T] {
	if len(ranges) == 0 {
		ranges = defaultRanges // Default alphabetical ranges
	}

	r := &RangeSharded[T]{
		shards:    make([]shard[T], len(ranges)),
		count:     len(ranges),
		keyRanges: make([]string, len(ranges)),
	}

	copy(r.keyRanges, ranges)
	sequence.From(r.keyRanges).Sort(func(a, b string) bool { return a < b })

	// Initialize all shards
	for i := range r.shards {
		r.shards[i].value.Store(initialValue)
		r.shards[i].version.Store(1)
	}

	r.version.Store(1)
	r.dirty.Store(false)
	return r
}

// getShardIndexForRange returns the shard index for a given key based on ranges
func (r *RangeSharded[T]) getShardIndexForRange(key string) int {
	r.mx.RLock()
	defer r.mx.RUnlock()

	for i, rangeKey := range r.keyRanges {
		if key < rangeKey {
			return i
		}
	}

	// Key is greater than all ranges, use last shard
	return len(r.keyRanges) - 1
}

// Get returns the value from the first shard
func (r *RangeSharded[T]) Get() T {
	return r.GetShard(defaultShardKey)
}

// Set sets the value in the appropriate shard based on key
func (r *RangeSharded[T]) Set(value T) {
	r.SetShard(defaultShardKey, value)
}

// GetShard returns the value from the shard responsible for the key
func (r *RangeSharded[T]) GetShard(shardKey string) T {
	idx := r.getShardIndexForRange(shardKey)
	sh := &r.shards[idx]

	sh.mx.RLock()
	defer sh.mx.RUnlock()

	return sh.value.Load().(T)
}

// SetShard sets the value in the shard responsible for the key
func (r *RangeSharded[T]) SetShard(shardKey string, value T) {
	idx := r.getShardIndexForRange(shardKey)
	sh := &r.shards[idx]

	sh.mx.Lock()
	defer sh.mx.Unlock()

	sh.value.Store(value)
	sh.version.Add(1)
	r.version.Add(1)
	r.dirty.Store(true)
}

// GetShardCount returns the number of shards
func (r *RangeSharded[T]) GetShardCount() int {
	return r.count
}

// ForEachShard iterates over all shards
func (r *RangeSharded[T]) ForEachShard(action func(shardIndex int, value T) bool) {
	for i := range r.shards {
		sh := &r.shards[i]
		sh.mx.RLock()
		value := sh.value.Load().(T)
		sh.mx.RUnlock()

		if !action(i, value) {
			break
		}
	}
}

// MergeShard merges values from all shards using the provided function
func (r *RangeSharded[T]) MergeShard(merge func(values []T) T) T {
	values := make([]T, r.count)

	for i := range r.shards {
		sh := &r.shards[i]
		sh.mx.RLock()
		values[i] = sh.value.Load().(T)
		sh.mx.RUnlock()
	}

	return merge(values)
}

// Version returns the current version number
func (r *RangeSharded[T]) Version() uint64 {
	return r.version.Load()
}

// IsDirty returns true if any shard has been modified since last clean
func (r *RangeSharded[T]) IsDirty() bool {
	return r.dirty.Load()
}

// MarkClean marks all shards as clean
func (r *RangeSharded[T]) MarkClean() {
	r.dirty.Store(false)
}

var _ sync.ShardedRoot[string] = (*AdaptiveSharded[string])(nil)

// AdaptiveSharded implements ShardedRoot with proper adaptive resizing
type AdaptiveSharded[T any] struct {
	shards  []*shard[T]
	count   atomic.Int32
	version atomic.Uint64
	dirty   atomic.Bool
	mx      sc.RWMutex

	// Adaptive resizing configuration
	minShards          int32 // Minimum number of shards
	maxShards          int32 // Maximum number of shards
	scaleUpThreshold   int64 // Operations per shard to trigger scale up
	scaleDownThreshold int64 // Operations per shard to trigger scale down

	// Load tracking
	totalOps  atomic.Int64 // Total operations counter
	sampleOps atomic.Int64 // Operations in current sample window
	resizing  atomic.Bool  // Flag to prevent concurrent resizing
}

// NewAdaptiveSharded creates a new AdaptiveSharded
func NewAdaptiveSharded[T any](initialShardCount int, initialValue T) *AdaptiveSharded[T] {
	if initialShardCount <= 0 {
		initialShardCount = 4
	}

	a := &AdaptiveSharded[T]{
		shards:             make([]*shard[T], initialShardCount),
		minShards:          int32(initialShardCount),
		maxShards:          int32(initialShardCount * 64), // Max 64x initial size
		scaleUpThreshold:   1000,                          // Scale up when >1000 ops per shard
		scaleDownThreshold: 100,                           // Scale down when <100 ops per shard
	}

	a.count.Store(int32(initialShardCount))

	// Initialize all shards
	for i := range a.shards {
		a.shards[i] = newShard(initialValue)
	}

	a.version.Store(1)
	a.dirty.Store(false)
	a.resizing.Store(false)
	a.totalOps.Store(0)
	a.sampleOps.Store(0)

	return a
}

// Get returns the value from the default shard
func (a *AdaptiveSharded[T]) Get() T {
	return a.GetShard(defaultShardKey)
}

// Set sets the value in the default shard
func (a *AdaptiveSharded[T]) Set(value T) {
	a.SetShard(defaultShardKey, value)
}

// GetShard returns the value from a specific shard
func (a *AdaptiveSharded[T]) GetShard(shardKey string) T {
	// Track operation for adaptive resizing
	a.trackOperation()

	// Calculate shard index
	idx := int(uint32(xHash(shardKey)) % uint32(a.count.Load()))

	a.mx.RLock()
	// Ensure index is within bounds
	if idx >= len(a.shards) {
		idx = len(a.shards) - 1
	}
	sh := a.shards[idx]
	a.mx.RUnlock()

	return sh.value.Load().(T)
}

// SetShard sets the value in a specific shard
func (a *AdaptiveSharded[T]) SetShard(shardKey string, value T) {
	// Track operation for adaptive resizing
	a.trackOperation()

	// Calculate shard index
	idx := int(xHash32(shardKey) % uint32(a.count.Load()))

	a.mx.RLock()
	// Ensure index is within bounds
	if idx >= len(a.shards) {
		idx = len(a.shards) - 1
	}
	sh := a.shards[idx]
	a.mx.RUnlock()

	// Set the value
	sh.mx.Lock()
	defer sh.mx.Unlock()

	sh.value.Store(value)
	sh.version.Add(1)
	a.version.Add(1)
	a.dirty.Store(true)
}

// GetShardCount returns the current number of shards
func (a *AdaptiveSharded[T]) GetShardCount() int {
	return int(a.count.Load())
}

// ForEachShard iterates over all shards
func (a *AdaptiveSharded[T]) ForEachShard(action func(shardIndex int, value T) bool) {
	a.mx.RLock()
	currentShards := make([]*shard[T], len(a.shards))
	copy(currentShards, a.shards)
	a.mx.RUnlock()

	for i := range currentShards {
		sh := currentShards[i]
		sh.mx.RLock()
		value := sh.value.Load().(T)
		sh.mx.RUnlock()

		if !action(i, value) {
			break
		}
	}
}

// MergeShard merges values from all shards using the provided function
func (a *AdaptiveSharded[T]) MergeShard(merge func(values []T) T) T {
	a.mx.RLock()
	currentShards := make([]*shard[T], len(a.shards))
	copy(currentShards, a.shards)
	a.mx.RUnlock()

	values := make([]T, len(currentShards))

	for i := range currentShards {
		sh := currentShards[i]
		sh.mx.RLock()
		values[i] = sh.value.Load().(T)
		sh.mx.RUnlock()
	}

	return merge(values)
}

// Version returns the current version number
func (a *AdaptiveSharded[T]) Version() uint64 {
	return a.version.Load()
}

// IsDirty returns true if any shard has been modified since last clean
func (a *AdaptiveSharded[T]) IsDirty() bool {
	return a.dirty.Load()
}

// MarkClean marks all shards as clean
func (a *AdaptiveSharded[T]) MarkClean() {
	a.dirty.Store(false)
}

// SetResizeThreshold sets the scale up threshold (for compatibility)
func (a *AdaptiveSharded[T]) SetResizeThreshold(threshold int64) {
	a.SetResizeThresholds(threshold, threshold/10)
}

// SetResizeThresholds sets both scale up and scale down thresholds
func (a *AdaptiveSharded[T]) SetResizeThresholds(scaleUp, scaleDown int64) {
	a.mx.Lock()
	defer a.mx.Unlock()
	a.scaleUpThreshold = scaleUp
	a.scaleDownThreshold = scaleDown
}

// SetShardLimits sets minimum and maximum shard counts
func (a *AdaptiveSharded[T]) SetShardLimits(min, max int32) {
	a.mx.Lock()
	defer a.mx.Unlock()
	a.minShards = min
	a.maxShards = max
}

// GetAccessCounts returns current load statistics
func (a *AdaptiveSharded[T]) GetAccessCounts() map[int]int64 {
	result := make(map[int]int64)
	result[0] = a.totalOps.Load()     // Total operations
	result[1] = a.sampleOps.Load()    // Current sample operations
	result[2] = int64(a.count.Load()) // Current shard count
	result[3] = a.scaleUpThreshold    // Scale up threshold
	result[4] = a.scaleDownThreshold  // Scale down threshold
	return result
}

// GetResizeStats returns detailed resize statistics
func (a *AdaptiveSharded[T]) GetResizeStats() map[string]int64 {
	result := make(map[string]int64)
	result["total_ops"] = a.totalOps.Load()
	result["sample_ops"] = a.sampleOps.Load()
	result["current_shards"] = int64(a.count.Load())
	result["min_shards"] = int64(a.minShards)
	result["max_shards"] = int64(a.maxShards)
	result["scale_up_threshold"] = a.scaleUpThreshold
	result["scale_down_threshold"] = a.scaleDownThreshold
	result["is_resizing"] = 0
	if a.resizing.Load() {
		result["is_resizing"] = 1
	}
	return result
}

// trackOperation tracks an operation and checks if resizing is needed
func (a *AdaptiveSharded[T]) trackOperation() {
	a.totalOps.Add(1)
	ops := a.sampleOps.Add(1)

	// Check if we need to evaluate resizing (every 1000 operations)
	if ops%1000 == 0 {
		go a.maybeResize() // Run in background to avoid blocking
	}
}

// maybeResize evaluates if resizing is needed and performs it
func (a *AdaptiveSharded[T]) maybeResize() {
	// Prevent concurrent resizing
	if !a.resizing.CompareAndSwap(false, true) {
		return
	}
	defer a.resizing.Store(false)

	currentCount := int(a.count.Load())
	currentOps := a.sampleOps.Swap(0) // Reset and get current sample

	// Calculate operations per shard
	opsPerShard := currentOps / int64(currentCount)

	// Decide if we need to scale up or down
	shouldScaleUp := opsPerShard > a.scaleUpThreshold && currentCount < int(a.maxShards)
	shouldScaleDown := opsPerShard < a.scaleDownThreshold && currentCount > int(a.minShards)

	if shouldScaleUp {
		a.scaleUp()
	} else if shouldScaleDown {
		a.scaleDown()
	}
}

// scaleUp doubles the number of shards
func (a *AdaptiveSharded[T]) scaleUp() {
	a.mx.Lock()
	defer a.mx.Unlock()

	currentCount := int(a.count.Load())
	newCount := currentCount * 2

	// Don't exceed max shards
	if newCount > int(a.maxShards) {
		newCount = int(a.maxShards)
	}

	if newCount == currentCount {
		return // No change needed
	}

	// Create new shards array
	newShards := make([]*shard[T], newCount)

	// Copy existing shards
	copy(newShards, a.shards)

	// Initialize new shards with default value from first shard
	defaultValue := a.shards[0].value.Load().(T)
	for i := currentCount; i < newCount; i++ {
		newShards[i] = newShard(defaultValue)
	}

	// Update the structure atomically
	a.shards = newShards
	a.count.Store(int32(newCount))
	a.version.Add(1)
}

// scaleDown halves the number of shards
func (a *AdaptiveSharded[T]) scaleDown() {
	a.mx.Lock()
	defer a.mx.Unlock()

	currentCount := int(a.count.Load())
	newCount := currentCount / 2

	// Don't go below min shards
	if newCount < int(a.minShards) {
		newCount = int(a.minShards)
	}

	if newCount == currentCount {
		return // No change needed
	}

	// Create new shards array (just truncate)
	newShards := make([]*shard[T], newCount)
	copy(newShards, a.shards[:newCount])

	// Update the structure atomically
	a.shards = newShards
	a.count.Store(int32(newCount))
	a.version.Add(1)
}
