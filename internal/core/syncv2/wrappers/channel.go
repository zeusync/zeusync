package wrappers

import (
	"encoding/binary"
	sc "sync"
	"time"

	"github.com/zeusync/zeusync/internal/core/syncv2"
	"github.com/zeusync/zeusync/internal/core/syncv2/metrics"
)

// ChannelWrapper wraps any ChannelRoot implementation to provide the full Core interface
type ChannelWrapper[T any] struct {
	root        sync.ChannelRoot[T]
	subscribers []func(T, T)
	permissions sync.Permissions
	metrics     metrics.VariableMetrics
	resolver    sync.ConflictResolver[T]
	marshaler   sync.Marshaller[T]
	mx          sc.RWMutex
}

// NewChannelWrapper creates a new ChannelWrapper around the given ChannelRoot
func NewChannelWrapper[T any](root sync.ChannelRoot[T]) *ChannelWrapper[T] {
	return &ChannelWrapper[T]{
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

func (w *ChannelWrapper[T]) Get() T {
	w.metrics.Reads++
	return w.root.Get()
}

func (w *ChannelWrapper[T]) Set(value T) {
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

func (w *ChannelWrapper[T]) Version() uint64 {
	return w.root.Version()
}

func (w *ChannelWrapper[T]) IsDirty() bool {
	return w.root.IsDirty()
}

func (w *ChannelWrapper[T]) MarkClean() {
	w.root.MarkClean()
}

// Networked interface implementation

func (w *ChannelWrapper[T]) Delta(since uint64) sync.Delta[T] {
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

func (w *ChannelWrapper[T]) ApplyDelta(deltas ...sync.Delta[T]) error {
	w.mx.Lock()
	defer w.mx.Unlock()

	// For channel-based variables, we apply deltas by sending them through the channel
	for _, delta := range deltas {
		switch delta.OpType {
		case sync.OpSet:
			w.root.Send(delta.Value)
		case sync.OpUpdate:
			// For update operations, we might need to get current value and modify it
			// This depends on the specific implementation
			w.root.Send(delta.Value)
		case sync.OpDelete:
		}
	}

	w.metrics.DeltasReceived++
	return nil
}

func (w *ChannelWrapper[T]) SetResolver(resolver sync.ConflictResolver[T]) {
	w.mx.Lock()
	defer w.mx.Unlock()
	w.resolver = resolver
}

func (w *ChannelWrapper[T]) Marshal() ([]byte, error) {
	return w.marshaler.Marshal(w.Get())
}

func (w *ChannelWrapper[T]) Unmarshal(data []byte) error {
	if value, err := w.marshaler.Unmarshal(data); err == nil {
		w.Set(value)
		return nil
	} else {
		w.metrics.Errors++
		return err
	}
}

// Serializable interface implementation

func (w *ChannelWrapper[T]) Serialize() ([]byte, error) {
	return w.Marshal()
}

func (w *ChannelWrapper[T]) Deserialize(data []byte) error {
	return w.Unmarshal(data)
}

// Observable interface implementation

func (w *ChannelWrapper[T]) OnChange(callback func(old, new T)) {
	w.mx.Lock()
	defer w.mx.Unlock()
	w.subscribers = append(w.subscribers, callback)
}

func (w *ChannelWrapper[T]) OnConflict(callback func(local, remote T) T) {
	w.mx.Lock()
	defer w.mx.Unlock()
	// Convert callback to ConflictResolver
	w.resolver = &callbackResolver[T]{callback: callback}
}

func (w *ChannelWrapper[T]) Metrics() metrics.VariableMetrics {
	return w.metrics
}

// Permissioned interface implementation

func (w *ChannelWrapper[T]) CheckRead(clientID string) bool {
	w.mx.RLock()
	defer w.mx.RUnlock()
	if len(w.permissions.Read) == 0 {
		return true // No restrictions
	}
	_, has := w.permissions.Read[clientID]
	return has
}

func (w *ChannelWrapper[T]) CheckWrite(clientID string) bool {
	w.mx.RLock()
	defer w.mx.RUnlock()
	if len(w.permissions.Write) == 0 {
		return true // No restrictions
	}
	_, has := w.permissions.Write[clientID]
	return has
}

func (w *ChannelWrapper[T]) SetPermissions(permissions sync.Permissions) {
	w.mx.Lock()
	w.permissions = permissions
	w.mx.Unlock()
}

// Core interface implementation

func (w *ChannelWrapper[T]) Close() error {
	w.mx.Lock()
	defer w.mx.Unlock()
	w.subscribers = nil
	w.resolver = nil
	return w.root.Close()
}

func (w *ChannelWrapper[T]) Size() int {
	return binary.Size(w.Get())
}

// ChannelRoot specific methods delegation

func (w *ChannelWrapper[T]) Send(value T) bool {
	w.metrics.Writes++
	return w.root.Send(value)
}

func (w *ChannelWrapper[T]) Receive() (T, bool) {
	w.metrics.Reads++
	return w.root.Receive()
}

func (w *ChannelWrapper[T]) SendBlocking(value T) {
	w.metrics.Writes++
	w.root.SendBlocking(value)
}

func (w *ChannelWrapper[T]) ReceiveBlocking() T {
	w.metrics.Reads++
	return w.root.ReceiveBlocking()
}

func (w *ChannelWrapper[T]) BufferSize() int {
	return w.root.BufferSize()
}

func (w *ChannelWrapper[T]) SetBufferSize(size int) error {
	return w.root.SetBufferSize(size)
}
