package wrappers

import (
	"encoding/binary"
	sc "sync"
	"time"

	"github.com/zeusync/zeusync/internal/core/syncv2"
	"github.com/zeusync/zeusync/internal/core/syncv2/metrics"
)

// AtomicWrapper wraps any AtomicRoot implementation to provide the full Core interface
type AtomicWrapper[T any] struct {
	root        sync.AtomicRoot[T]
	subscribers []func(T, T)
	permissions sync.Permissions
	metrics     metrics.VariableMetrics
	resolver    sync.ConflictResolver[T]
	marshaler   sync.Marshaller[T]
	mx          sc.RWMutex
}

// NewAtomicWrapper creates a new AtomicWrapper around the given AtomicRoot
func NewAtomicWrapper[T any](root sync.AtomicRoot[T]) *AtomicWrapper[T] {
	return &AtomicWrapper[T]{
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

func (w *AtomicWrapper[T]) Get() T {
	w.metrics.Reads++
	return w.root.Get()
}

func (w *AtomicWrapper[T]) Set(value T) {
	old := w.root.Get()
	w.root.Set(value)
	w.metrics.Writes++

	// Notify subscribers

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

func (w *AtomicWrapper[T]) Version() uint64 {
	return w.root.Version()
}

func (w *AtomicWrapper[T]) IsDirty() bool {
	return w.root.IsDirty()
}

func (w *AtomicWrapper[T]) MarkClean() {
	w.root.MarkClean()
}

// Networked interface implementation

func (w *AtomicWrapper[T]) Delta(since uint64) sync.Delta[T] {
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

func (w *AtomicWrapper[T]) ApplyDelta(deltas ...sync.Delta[T]) error {
	for _, delta := range deltas {
		if delta.Version > w.Version() {
			if w.resolver != nil {
				// Use conflict resolver if available
				resolved := w.resolver.Resolve(w.Get(), delta.Value, sync.ConflictMetadata{
					LocalVersion:  w.Version(),
					RemoteVersion: delta.Version,
					LocalTime:     time.Now().UnixNano(),
					RemoteTime:    delta.Timestamp,
					ClientID:      delta.ClientID,
				})
				w.Set(resolved)
				w.metrics.DeltasReceived++
			} else {
				// Last-write-wins by default
				w.Set(delta.Value)
			}
			w.metrics.Conflicts++
		}
	}
	return nil
}

func (w *AtomicWrapper[T]) SetResolver(resolver sync.ConflictResolver[T]) {
	w.mx.RLock()
	defer w.mx.RUnlock()
	w.resolver = resolver
}

func (w *AtomicWrapper[T]) Marshal() ([]byte, error) {
	return w.marshaler.Marshal(w.Get())
}

func (w *AtomicWrapper[T]) Unmarshal(data []byte) error {
	if value, err := w.marshaler.Unmarshal(data); err == nil {
		w.Set(value)
		return nil
	} else {
		w.metrics.Errors++
		return err
	}
}

// Serializable interface implementation

func (w *AtomicWrapper[T]) Serialize() ([]byte, error) {
	return w.Marshal()
}

func (w *AtomicWrapper[T]) Deserialize(data []byte) error {
	return w.Unmarshal(data)
}

// Observable interface implementation

func (w *AtomicWrapper[T]) OnChange(callback func(old, new T)) {
	w.mx.Lock()
	defer w.mx.Unlock()
	w.subscribers = append(w.subscribers, callback)
}

func (w *AtomicWrapper[T]) OnConflict(callback func(local, remote T) T) {
	w.mx.Lock()
	defer w.mx.Unlock()
	// Convert callback to ConflictResolver
	w.resolver = &callbackResolver[T]{callback: callback}
}

func (w *AtomicWrapper[T]) Metrics() metrics.VariableMetrics {
	return w.metrics
}

// Permissioned interface implementation

func (w *AtomicWrapper[T]) CheckRead(clientID string) bool {
	w.mx.RLock()
	defer w.mx.RUnlock()
	if len(w.permissions.Read) == 0 {
		return true // No restrictions
	}
	_, has := w.permissions.Read[clientID]
	return has
}

func (w *AtomicWrapper[T]) CheckWrite(clientID string) bool {
	w.mx.RLock()
	defer w.mx.RUnlock()
	if len(w.permissions.Write) == 0 {
		return true // No restrictions
	}
	_, has := w.permissions.Write[clientID]
	return has
}

func (w *AtomicWrapper[T]) SetPermissions(permissions sync.Permissions) {
	w.mx.Lock()
	w.permissions = permissions
	w.mx.Unlock()
}

// Core interface implementation

func (w *AtomicWrapper[T]) Close() error {
	w.mx.Lock()
	w.subscribers = nil
	w.resolver = nil
	w.mx.Unlock()
	return nil
}

func (w *AtomicWrapper[T]) Size() int {
	return binary.Size(w.Get())
}
