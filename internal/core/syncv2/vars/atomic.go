package vars

import (
	"sync/atomic"

	"github.com/zeusync/zeusync/internal/core/syncv2"
	"github.com/zeusync/zeusync/pkg/sequence"
)

// AtomicValue provides a generic atomic implementation for any type
type AtomicValue[T any] struct {
	value   atomic.Value
	version atomic.Uint64
	dirty   atomic.Bool
}

// NewAtomicValue creates a new AtomicValue with the given initial value
func NewAtomicValue[T any](initialValue T) *AtomicValue[T] {
	a := &AtomicValue[T]{}
	a.value.Store(initialValue)
	a.version.Store(1)
	a.dirty.Store(false)
	return a
}

// Get returns the current value atomically
func (a *AtomicValue[T]) Get() T {
	return a.value.Load().(T)
}

// Set sets the value atomically
func (a *AtomicValue[T]) Set(value T) {
	a.value.Store(value)
	a.version.Add(1)
	a.dirty.Store(true)
}

// Swap atomically swaps the value and returns the old value
func (a *AtomicValue[T]) Swap(new T) T {
	old := a.value.Swap(new)
	a.version.Add(1)
	a.dirty.Store(true)
	return old.(T)
}

// CompareAndSwap performs an atomic compare-and-swap operation
func (a *AtomicValue[T]) CompareAndSwap(old, new T) bool {
	if a.value.CompareAndSwap(old, new) {
		a.version.Add(1)
		a.dirty.Store(true)
		return true
	}
	return false
}

// ApplyDeltas applies multiple deltas atomically
func (a *AtomicValue[T]) ApplyDeltas(deltas ...sync.Delta[T]) T {
	sequence.From(deltas).
		Sort(func(a, b sync.Delta[T]) bool {
			return a.Version > b.Version
		}).
		Each(func(d sync.Delta[T]) {
			if d.OpType == sync.OpSet {
				a.Set(d.Value)
				return
			}
		})

	return a.Get()
}

// Version returns the current version number
func (a *AtomicValue[T]) Version() uint64 {
	return a.version.Load()
}

// IsDirty returns true if the value has been modified since last clean
func (a *AtomicValue[T]) IsDirty() bool {
	return a.dirty.Load()
}

// MarkClean marks the value as clean
func (a *AtomicValue[T]) MarkClean() {
	a.dirty.Store(false)
}
