package vars

import (
	sync "github.com/zeusync/zeusync/internal/core/syncv2"
	"github.com/zeusync/zeusync/pkg/sequence"
	"sync/atomic"
)

var _ sync.AtomicRoot[bool] = (*AtomicBool)(nil)

// AtomicBool provides an optimized atomic implementation for bool values
type AtomicBool struct {
	value   atomic.Bool
	version atomic.Uint64
	dirty   atomic.Bool
}

// NewAtomicBool creates a new AtomicBool with the given initial value
func NewAtomicBool(initialValue bool) *AtomicBool {
	a := &AtomicBool{}
	a.value.Store(initialValue)
	a.version.Store(1)
	a.dirty.Store(false)
	return a
}

// Get returns the current value atomically
func (a *AtomicBool) Get() bool {
	return a.value.Load()
}

// Set sets the value atomically
func (a *AtomicBool) Set(value bool) {
	a.value.Store(value)
	a.version.Add(1)
	a.dirty.Store(true)
}

// Swap atomically swaps the value and returns the old value
func (a *AtomicBool) Swap(new bool) bool {
	old := a.value.Swap(new)
	a.version.Add(1)
	a.dirty.Store(true)
	return old
}

// CompareAndSwap performs an atomic compare-and-swap operation
func (a *AtomicBool) CompareAndSwap(old, new bool) bool {
	if a.value.CompareAndSwap(old, new) {
		a.version.Add(1)
		a.dirty.Store(true)
		return true
	}
	return false
}

// ApplyDeltas applies multiple deltas atomically
func (a *AtomicBool) ApplyDeltas(deltas ...sync.Delta[bool]) bool {
	if last, is := sequence.From(deltas).
		Sort(func(a, b sync.Delta[bool]) bool {
			return a.Version <= b.Version
		}).Last(); is {
		a.Set(last.Value)
	}
	return a.Get()
}

// Version returns the current version number
func (a *AtomicBool) Version() uint64 {
	return a.version.Load()
}

// IsDirty returns true if the value has been modified since last clean
func (a *AtomicBool) IsDirty() bool {
	return a.dirty.Load()
}

// MarkClean marks the value as clean
func (a *AtomicBool) MarkClean() {
	a.dirty.Store(false)
}
