package vars

import (
	"sync/atomic"

	sync "github.com/zeusync/zeusync/internal/core/syncv2"
	"github.com/zeusync/zeusync/pkg/sequence"
)

var _ sync.AtomicRoot[uint64] = (*AtomicUint64)(nil)

// AtomicUint64 provides an optimized atomic implementation for uint64 values
type AtomicUint64 struct {
	value   atomic.Uint64
	version atomic.Uint64
	dirty   atomic.Bool
}

// NewAtomicUint64 creates a new AtomicUint64 with the given initial value
func NewAtomicUint64(initialValue uint64) *AtomicUint64 {
	a := &AtomicUint64{}
	a.value.Store(initialValue)
	a.version.Store(1)
	a.dirty.Store(false)
	return a
}

// Get returns the current value atomically
func (a *AtomicUint64) Get() uint64 {
	return a.value.Load()
}

// Set sets the value atomically
func (a *AtomicUint64) Set(value uint64) {
	a.value.Store(value)
	a.version.Add(1)
	a.dirty.Store(true)
}

// Swap atomically swaps the value and returns the old value
func (a *AtomicUint64) Swap(new uint64) uint64 {
	old := a.value.Swap(new)
	a.version.Add(1)
	a.dirty.Store(true)
	return old
}

// Add atomically adds delta to the value and returns the new value
func (a *AtomicUint64) Add(delta uint64) uint64 {
	newValue := a.value.Add(delta)
	a.version.Add(1)
	a.dirty.Store(true)
	return newValue
}

// CompareAndSwap performs an atomic compare-and-swap operation
func (a *AtomicUint64) CompareAndSwap(old, new uint64) bool {
	if a.value.CompareAndSwap(old, new) {
		a.version.Add(1)
		a.dirty.Store(true)
		return true
	}
	return false
}

// ApplyDeltas applies multiple deltas atomically
func (a *AtomicUint64) ApplyDeltas(deltas ...sync.Delta[uint64]) uint64 {
	deltas = sequence.From(deltas).
		Sort(func(a, b sync.Delta[uint64]) bool {
			return a.Version <= b.Version
		}).Collect()

	var (
		totalDelta uint64
		setValue   *uint64
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
func (a *AtomicUint64) Version() uint64 {
	return a.version.Load()
}

// IsDirty returns true if the value has been modified since last clean
func (a *AtomicUint64) IsDirty() bool {
	return a.dirty.Load()
}

// MarkClean marks the value as clean
func (a *AtomicUint64) MarkClean() {
	a.dirty.Store(false)
}

var _ sync.AtomicRoot[uint32] = (*AtomicUint32)(nil)

// AtomicUint32 provides an optimized atomic implementation for uint32 values
type AtomicUint32 struct {
	value   atomic.Uint32
	version atomic.Uint64
	dirty   atomic.Bool
}

// NewAtomicUint32 creates a new AtomicUint32 with the given initial value
func NewAtomicUint32(initialValue uint32) *AtomicUint32 {
	a := &AtomicUint32{}
	a.value.Store(initialValue)
	a.version.Store(1)
	a.dirty.Store(false)
	return a
}

// Get returns the current value atomically
func (a *AtomicUint32) Get() uint32 {
	return a.value.Load()
}

// Set sets the value atomically
func (a *AtomicUint32) Set(value uint32) {
	a.value.Store(value)
	a.version.Add(1)
	a.dirty.Store(true)
}

// Swap atomically swaps the value and returns the old value
func (a *AtomicUint32) Swap(new uint32) uint32 {
	old := a.value.Swap(new)
	a.version.Add(1)
	a.dirty.Store(true)
	return old
}

// Add atomically adds delta to the value and returns the new value
func (a *AtomicUint32) Add(delta uint32) uint32 {
	newValue := a.value.Add(delta)
	a.version.Add(1)
	a.dirty.Store(true)
	return newValue
}

// CompareAndSwap performs an atomic compare-and-swap operation
func (a *AtomicUint32) CompareAndSwap(old, new uint32) bool {
	if a.value.CompareAndSwap(old, new) {
		a.version.Add(1)
		a.dirty.Store(true)
		return true
	}
	return false
}

// ApplyDeltas applies multiple deltas atomically
func (a *AtomicUint32) ApplyDeltas(deltas ...sync.Delta[uint32]) uint32 {
	deltas = sequence.From(deltas).
		Sort(func(a, b sync.Delta[uint32]) bool {
			return a.Version <= b.Version
		}).Collect()

	var (
		totalDelta uint32
		setValue   *uint32
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
func (a *AtomicUint32) Version() uint64 {
	return a.version.Load()
}

// IsDirty returns true if the value has been modified since last clean
func (a *AtomicUint32) IsDirty() bool {
	return a.dirty.Load()
}

// MarkClean marks the value as clean
func (a *AtomicUint32) MarkClean() {
	a.dirty.Store(false)
}
