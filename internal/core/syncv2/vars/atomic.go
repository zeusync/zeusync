package vars

import (
	"github.com/zeusync/zeusync/internal/core/syncv2"
	"github.com/zeusync/zeusync/pkg/sequence"
	"sync/atomic"
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

var _ sync.AtomicRoot[string] = (*AtomicString)(nil)

// AtomicString provides an optimized atomic implementation for string values
// It uses FastString internally but exposes regular string interface
type AtomicString struct {
	value   atomic.Pointer[FastString]
	version atomic.Uint64
	dirty   atomic.Bool
}

// NewAtomicString creates a new AtomicString with the given initial value
func NewAtomicString(initialValue string) *AtomicString {
	a := &AtomicString{}
	fastStr := NewFastString(initialValue)
	a.value.Store(fastStr)
	a.version.Store(1)
	a.dirty.Store(false)
	return a
}

// Get returns the current value as string atomically
func (a *AtomicString) Get() string {
	fastStr := a.value.Load()
	if fastStr == nil {
		return ""
	}
	return fastStr.String()
}

// Set sets the value atomically
func (a *AtomicString) Set(value string) {
	fastStr := NewFastString(value)
	a.value.Store(fastStr)
	a.version.Add(1)
	a.dirty.Store(true)
}

// Swap atomically swaps the value and returns the old value as string
func (a *AtomicString) Swap(new string) string {
	newFastStr := NewFastString(new)
	oldFastStr := a.value.Swap(newFastStr)
	a.version.Add(1)
	a.dirty.Store(true)

	if oldFastStr == nil {
		return ""
	}
	return oldFastStr.String()
}

// ApplyDeltas applies multiple deltas atomically
func (a *AtomicString) ApplyDeltas(deltas ...sync.Delta[string]) string {
	deltas = sequence.From(deltas).
		Sort(func(a, b sync.Delta[string]) bool {
			return a.Version <= b.Version
		}).Collect()

	var setValue *string

	for _, delta := range deltas {
		switch delta.OpType {
		case sync.OpSet:
			setValue = &delta.Value
		case sync.OpUpdate:
			if setValue == nil {
				current := a.Get()
				newValue := current + delta.Value
				setValue = &newValue
			}
		default:
		}
	}

	if setValue != nil {
		a.Set(*setValue)
		return *setValue
	}

	return a.Get()
}

// Version returns the current version number
func (a *AtomicString) Version() uint64 {
	return a.version.Load()
}

// IsDirty returns true if the value has been modified since last clean
func (a *AtomicString) IsDirty() bool {
	return a.dirty.Load()
}

// MarkClean marks the value as clean
func (a *AtomicString) MarkClean() {
	a.dirty.Store(false)
}

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
