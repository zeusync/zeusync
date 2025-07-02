package vars

import (
	"sync/atomic"

	sync "github.com/zeusync/zeusync/internal/core/syncv2"
	"github.com/zeusync/zeusync/pkg/sequence"
)

var _ sync.AtomicRoot[int64] = (*AtomicInt64)(nil)

// AtomicInt64 provides an optimized atomic implementation for int64 values
// It uses sync/atomic.Int64 for maximum performance
type AtomicInt64 struct {
	value   atomic.Int64
	version atomic.Uint64
	dirty   atomic.Bool
}

// NewAtomicInt64 creates a new AtomicInt64 with the given initial value
func NewAtomicInt64(initialValue int64) *AtomicInt64 {
	a := &AtomicInt64{}
	a.value.Store(initialValue)
	a.version.Store(1)
	a.dirty.Store(false)
	return a
}

// Get returns the current value atomically
func (a *AtomicInt64) Get() int64 {
	return a.value.Load()
}

// Set sets the value atomically
func (a *AtomicInt64) Set(value int64) {
	a.value.Store(value)
	a.version.Add(1)
	a.dirty.Store(true)
}

// Swap atomically swaps the value and returns the old value
func (a *AtomicInt64) Swap(new int64) int64 {
	old := a.value.Swap(new)
	a.version.Add(1)
	a.dirty.Store(true)
	return old
}

// Add atomically adds delta to the value and returns the new value
func (a *AtomicInt64) Add(delta int64) int64 {
	newValue := a.value.Add(delta)
	a.version.Add(1)
	a.dirty.Store(true)
	return newValue
}

// CompareAndSwap performs an atomic compare-and-swap operation
func (a *AtomicInt64) CompareAndSwap(old, new int64) bool {
	if a.value.CompareAndSwap(old, new) {
		a.version.Add(1)
		a.dirty.Store(true)
		return true
	}
	return false
}

// ApplyDeltas applies multiple deltas atomically
func (a *AtomicInt64) ApplyDeltas(deltas ...sync.Delta[int64]) int64 {
	deltas = sequence.From(deltas).
		Sort(func(a, b sync.Delta[int64]) bool {
			return a.Version <= b.Version
		}).Collect()

	var (
		totalDelta int64
		setValue   *int64
	)

	for _, delta := range deltas {
		switch delta.OpType {
		case sync.OpSet:
			setValue = &delta.Value
			totalDelta = 0
		case sync.OpUpdate:
			if setValue == nil {
				totalDelta += delta.Value
			}
		default:
		}
	}

	if setValue != nil {
		a.Set(*setValue)
		return *setValue
	} else if totalDelta != 0 {
		return a.Add(totalDelta)
	}

	return a.Get()
}

// Version returns the current version number
func (a *AtomicInt64) Version() uint64 {
	return a.version.Load()
}

// IsDirty returns true if the value has been modified since last clean
func (a *AtomicInt64) IsDirty() bool {
	return a.dirty.Load()
}

// MarkClean marks the value as clean
func (a *AtomicInt64) MarkClean() {
	a.dirty.Store(false)
}

var _ sync.AtomicRoot[int32] = (*AtomicInt32)(nil)

// AtomicInt32 provides an optimized atomic implementation for int32 values
type AtomicInt32 struct {
	value   atomic.Int32
	version atomic.Uint64
	dirty   atomic.Bool
}

// NewAtomicInt32 creates a new AtomicInt32 with the given initial value
func NewAtomicInt32(initialValue int32) *AtomicInt32 {
	a := &AtomicInt32{}
	a.value.Store(initialValue)
	a.version.Store(1)
	a.dirty.Store(false)
	return a
}

// Get returns the current value atomically
func (a *AtomicInt32) Get() int32 {
	return a.value.Load()
}

// Set sets the value atomically
func (a *AtomicInt32) Set(value int32) {
	a.value.Store(value)
	a.version.Add(1)
	a.dirty.Store(true)
}

// Swap atomically swaps the value and returns the old value
func (a *AtomicInt32) Swap(new int32) int32 {
	old := a.value.Swap(new)
	a.version.Add(1)
	a.dirty.Store(true)
	return old
}

// Add atomically adds delta to the value and returns the new value
func (a *AtomicInt32) Add(delta int32) int32 {
	newValue := a.value.Add(delta)
	a.version.Add(1)
	a.dirty.Store(true)
	return newValue
}

// CompareAndSwap performs an atomic compare-and-swap operation
func (a *AtomicInt32) CompareAndSwap(old, new int32) bool {
	if a.value.CompareAndSwap(old, new) {
		a.version.Add(1)
		a.dirty.Store(true)
		return true
	}
	return false
}

// ApplyDeltas applies multiple deltas atomically
func (a *AtomicInt32) ApplyDeltas(deltas ...sync.Delta[int32]) int32 {
	deltas = sequence.From(deltas).
		Sort(func(a, b sync.Delta[int32]) bool {
			return a.Version <= b.Version
		}).Collect()

	var (
		totalDelta int32
		setValue   *int32
	)

	for _, delta := range deltas {
		switch delta.OpType {
		case sync.OpSet:
			setValue = &delta.Value
			totalDelta = 0
		case sync.OpUpdate:
			if setValue == nil {
				totalDelta += delta.Value
			}
		default:
		}
	}

	if setValue != nil {
		a.Set(*setValue)
		return *setValue
	} else if totalDelta != 0 {
		return a.Add(totalDelta)
	}

	return a.Get()
}

// Version returns the current version number
func (a *AtomicInt32) Version() uint64 {
	return a.version.Load()
}

// IsDirty returns true if the value has been modified since last clean
func (a *AtomicInt32) IsDirty() bool {
	return a.dirty.Load()
}

// MarkClean marks the value as clean
func (a *AtomicInt32) MarkClean() {
	a.dirty.Store(false)
}
