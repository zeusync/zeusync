package wrappers

import (
	"encoding/binary"
	"github.com/zeusync/zeusync/pkg/concurrent"
	"github.com/zeusync/zeusync/pkg/sequence"
	sc "sync"
	"time"

	"github.com/zeusync/zeusync/internal/core/syncv2"
	"github.com/zeusync/zeusync/internal/core/syncv2/metrics"
)

// ShardedWrapper wraps any ShardedRoot implementation to provide the full Core interface
type ShardedWrapper[T any] struct {
	root        sync.ShardedRoot[T]
	subscribers []func(T, T)
	permissions sync.Permissions
	metrics     metrics.VariableMetrics
	resolver    sync.ConflictResolver[T]
	marshaler   sync.Marshaller[T]
	mx          sc.RWMutex
}

// NewShardedWrapper creates a new ShardedWrapper around the given ShardedRoot
func NewShardedWrapper[T any](root sync.ShardedRoot[T]) *ShardedWrapper[T] {
	return &ShardedWrapper[T]{
		root:        root,
		subscribers: make([]func(T, T), 0),
		permissions: sync.Permissions{
			Read:  map[string]struct{}{},
			Write: map[string]struct{}{},
		},
		metrics: metrics.VariableMetrics{},
	}
}

// Root interface implementation

func (w *ShardedWrapper[T]) Get() T {
	w.metrics.Reads++
	return w.root.Get()
}

func (w *ShardedWrapper[T]) Set(value T) {
	old := w.root.Get()
	w.root.Set(value)
	w.metrics.Writes++

	// Notify subscribers
	concurrent.Batch(sequence.From(w.subscribers), 3, func(batch []func(T, T)) {
		for _, callback := range batch {
			if callback != nil {
				go callback(old, value)
			}
		}
	})
}

func (w *ShardedWrapper[T]) Version() uint64 {
	return w.root.Version()
}

func (w *ShardedWrapper[T]) IsDirty() bool {
	return w.root.IsDirty()
}

func (w *ShardedWrapper[T]) MarkClean() {
	w.root.MarkClean()
}

// Networked interface implementation

func (w *ShardedWrapper[T]) Delta(since uint64) sync.Delta[T] {
	w.mx.Lock()
	defer w.mx.Unlock()
	if since < w.Version() {
		w.metrics.DeltasSent++
		return sync.Delta[T]{
			Version:   w.Version(),
			Value:     w.Get(),
			Timestamp: time.Now().UnixNano(),
			OpType:    sync.OpSet,
		}
	}
	return sync.Delta[T]{}
}

func (w *ShardedWrapper[T]) ApplyDelta(deltas ...sync.Delta[T]) error {
	w.mx.Lock()
	defer w.mx.Unlock()

	// For sharded variables, we apply deltas to the default shard
	for _, delta := range deltas {
		switch delta.OpType {
		case sync.OpSet:
			w.root.Set(delta.Value)
		case sync.OpUpdate:
			w.root.Set(delta.Value)
		case sync.OpDelete:
			// Handle delete operation if needed
		}
	}

	w.metrics.DeltasReceived++
	return nil
}

func (w *ShardedWrapper[T]) SetResolver(resolver sync.ConflictResolver[T]) {
	w.mx.Lock()
	defer w.mx.Unlock()
	w.resolver = resolver
}

func (w *ShardedWrapper[T]) Marshal() ([]byte, error) {
	return w.marshaler.Marshal(w.Get())
}

func (w *ShardedWrapper[T]) Unmarshal(data []byte) error {
	if value, err := w.marshaler.Unmarshal(data); err == nil {
		w.Set(value)
		return nil
	} else {
		w.metrics.Errors++
		return err
	}
}

// Serializable interface implementation

func (w *ShardedWrapper[T]) Serialize() ([]byte, error) {
	return w.Marshal()
}

func (w *ShardedWrapper[T]) Deserialize(data []byte) error {
	return w.Unmarshal(data)
}

// Observable interface implementation

func (w *ShardedWrapper[T]) OnChange(callback func(old, new T)) {
	w.mx.Lock()
	defer w.mx.Unlock()
	w.subscribers = append(w.subscribers, callback)
}

func (w *ShardedWrapper[T]) OnConflict(callback func(local, remote T) T) {
	w.mx.Lock()
	defer w.mx.Unlock()
	// Convert callback to ConflictResolver
	w.resolver = &callbackResolver[T]{callback: callback}
}

func (w *ShardedWrapper[T]) Metrics() metrics.VariableMetrics {
	return w.metrics
}

// Permissioned interface implementation

func (w *ShardedWrapper[T]) CheckRead(clientID string) bool {
	w.mx.RLock()
	defer w.mx.RUnlock()
	if len(w.permissions.Read) == 0 {
		return true // No restrictions
	}
	_, has := w.permissions.Read[clientID]
	return has
}

func (w *ShardedWrapper[T]) CheckWrite(clientID string) bool {
	w.mx.RLock()
	defer w.mx.RUnlock()
	if len(w.permissions.Write) == 0 {
		return true // No restrictions
	}
	_, has := w.permissions.Write[clientID]
	return has
}

func (w *ShardedWrapper[T]) SetPermissions(permissions sync.Permissions) {
	w.mx.Lock()
	w.permissions = permissions
	w.mx.Unlock()
}

// Core interface implementation

func (w *ShardedWrapper[T]) Close() error {
	w.mx.Lock()
	w.subscribers = nil
	w.resolver = nil
	w.mx.Unlock()
	return nil
}

func (w *ShardedWrapper[T]) Size() int {
	return binary.Size(w.Get())
}

// ShardedRoot specific methods delegation

func (w *ShardedWrapper[T]) GetShard(shardKey string) T {
	w.metrics.Reads++
	return w.root.GetShard(shardKey)
}

func (w *ShardedWrapper[T]) SetShard(shardKey string, value T) {
	old := w.root.GetShard(shardKey)
	w.root.SetShard(shardKey, value)
	w.metrics.Writes++

	// Notify subscribers for shard-specific changes
	go func() {
		w.mx.RLock()
		for _, callback := range w.subscribers {
			if callback != nil {
				callback(old, value)
			}
		}
		w.mx.RUnlock()
	}()
}

func (w *ShardedWrapper[T]) GetShardCount() int {
	return w.root.GetShardCount()
}

func (w *ShardedWrapper[T]) ForEachShard(action func(shardIndex int, value T) bool) {
	w.root.ForEachShard(action)
}

func (w *ShardedWrapper[T]) MergeShard(merge func(values []T) T) T {
	w.metrics.Reads++
	return w.root.MergeShard(merge)
}
