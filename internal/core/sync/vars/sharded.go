package vars

import (
	"reflect"
	sc "sync"
	"sync/atomic"
	"time"

	"github.com/zeusync/zeusync/internal/core/sync"
)

type Sharded[T any] struct {
	shards    []shard[T]
	shardMask uint64
	hasher    func([]byte) uint64
	name      string
	collector sync.MetricsCollector
}

type shard[T any] struct {
	mu      sc.RWMutex
	value   T
	version atomic.Uint64
	dirty   atomic.Bool
}

func NewShardedVariable[T any](initialValue T, shardCunt int, cfg sync.VariableConfig, collector sync.MetricsCollector) *Sharded[T] {
	if shardCunt&(shardCunt-1) != 0 {
		shardCunt = nextPowerOf2(shardCunt)
	}

	shards := make([]shard[T], shardCunt)
	for i := range shards {
		shards[i].value = initialValue
		shards[i].version.Store(1)
	}

	return &Sharded[T]{
		shards:    shards,
		shardMask: uint64(shardCunt - 1),
		hasher:    fnvHash,
		name:      cfg.Name,
		collector: collector,
	}
}

func (v *Sharded[T]) Get() (T, error) {
	start := time.Now()
	defer func() {
		if v.collector != nil {
			v.collector.RecordRead(v.name, time.Since(start))
		}
	}()

	sd := &v.shards[0]
	sd.mu.RLock()
	value := sd.value
	sd.mu.RUnlock()
	return value, nil
}

func (v *Sharded[T]) GetWithKey(key []byte) (T, error) {
	start := time.Now()
	defer func() {
		if v.collector != nil {
			v.collector.RecordRead(v.name, time.Since(start))
		}
	}()

	sd := v.getShard(key)
	sd.mu.RLock()
	value := sd.value
	sd.mu.RUnlock()
	return value, nil
}

func (v *Sharded[T]) getShard(key []byte) *shard[T] {
	hash := v.hasher(key)
	return &v.shards[hash&v.shardMask]
}

func (v *Sharded[T]) Set(newValue T) error {
	start := time.Now()
	defer func() {
		if v.collector != nil {
			v.collector.RecordWrite(v.name, time.Since(start))
		}
	}()

	sd := &v.shards[0]
	sd.mu.Lock()
	sd.value = newValue
	sd.version.Add(1)
	sd.dirty.Store(true)
	sd.mu.Unlock()
	return nil
}

func (v *Sharded[T]) SetWithKey(key []byte, newValue T) error {
	start := time.Now()
	defer func() {
		if v.collector != nil {
			v.collector.RecordWrite(v.name, time.Since(start))
		}
	}()

	sd := v.getShard(key)
	sd.mu.Lock()
	sd.value = newValue
	sd.version.Add(1)
	sd.dirty.Store(true)
	sd.mu.Unlock()
	return nil
}

func (v *Sharded[T]) IsDirty() bool {
	return v.shards[0].dirty.Load()
}

func (v *Sharded[T]) MarkClean() {
	v.shards[0].dirty.Store(false)
}

func (v *Sharded[T]) GetVersion() uint64 {
	return v.shards[0].version.Load()
}

func (v *Sharded[T]) GetDelta(sinceVersion uint64) ([]sync.Delta, error) {
	return nil, nil
}

func (v *Sharded[T]) ApplyDelta(delta ...sync.Delta) error {
	// TODO implement me
	panic("implement me")
}

func (v *Sharded[T]) SetConflictResolver(resolver sync.ConflictResolver) {
	// TODO implement me
	panic("implement me")
}

func (v *Sharded[T]) OnChange(f func(oldValue any, newValue any)) {
	// TODO implement me
	panic("implement me")
}

func (v *Sharded[T]) OnConflict(f func(local any, remote any) any) {
	// TODO implement me
	panic("implement me")
}

func (v *Sharded[T]) GetPermissions() sync.PermissionMask {
	// TODO implement me
	panic("implement me")
}

func (v *Sharded[T]) SetPermissions(mask sync.PermissionMask) {
	// TODO implement me
	panic("implement me")
}

func (v *Sharded[T]) GetMetrics() sync.VariableMetrics {
	// TODO implement me
	panic("implement me")
}

func (v *Sharded[T]) GetStorageStrategy() sync.StorageStrategy {
	// TODO implement me
	panic("implement me")
}

func (v *Sharded[T]) CanMigrateTo(strategy sync.StorageStrategy) bool {
	// TODO implement me
	panic("implement me")
}

func (v *Sharded[T]) GetType() reflect.Type {
	// TODO implement me
	panic("implement me")
}

func (v *Sharded[T]) GetHistory() []sync.HistoryEntry {
	// TODO implement me
	panic("implement me")
}

func (v *Sharded[T]) Close() error {
	// TODO implement me
	panic("implement me")
}

func (v *Sharded[T]) Size() int64 {
	// TODO implement me
	panic("implement me")
}

func nextPowerOf2(n int) int {
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	n++
	return n
}

func fnvHash(data []byte) uint64 {
	const (
		offset64 = 14695981039346656037
		prime64  = 1099511628211
	)
	hash := uint64(offset64)
	for _, b := range data {
		hash ^= uint64(b)
		hash *= prime64
	}
	return hash
}

func crc32Checksum(data []byte) uint32 {
	// Simple checksum implementation
	var checksum uint32
	for _, b := range data {
		checksum = checksum<<8 ^ uint32(b)
	}
	return checksum
}
