package vars

import (
	sync "github.com/zeusync/zeusync/internal/core/syncv2"
	"github.com/zeusync/zeusync/pkg/sequence"
	"sync/atomic"
	"unsafe"
)

var _ sync.AtomicRoot[unsafe.Pointer] = (*AtomicPointer)(nil)

// AtomicPointer provides an optimized atomic implementation for unsafe.Pointer values
// It uses sync/atomic operations for maximum performance
type AtomicPointer struct {
	value   atomic.Pointer[unsafe.Pointer]
	version atomic.Uint64
	dirty   atomic.Bool
}

// NewAtomicPointer creates a new AtomicPointer with the given initial value
func NewAtomicPointer(initialValue unsafe.Pointer) *AtomicPointer {
	a := &AtomicPointer{}
	if initialValue != nil {
		a.value.Store(&initialValue)
	}
	a.version.Store(1)
	a.dirty.Store(false)
	return a
}

// Get returns the current pointer value atomically
func (a *AtomicPointer) Get() unsafe.Pointer {
	ptr := a.value.Load()
	if ptr == nil {
		return nil
	}
	return *ptr
}

// Set sets the pointer value atomically
func (a *AtomicPointer) Set(value unsafe.Pointer) {
	if value == nil {
		a.value.Store(nil)
	} else {
		a.value.Store(&value)
	}
	a.version.Add(1)
	a.dirty.Store(true)
}

// Swap atomically swaps the pointer value and returns the old value
func (a *AtomicPointer) Swap(new unsafe.Pointer) unsafe.Pointer {
	var newPtr *unsafe.Pointer
	if new != nil {
		newPtr = &new
	}

	oldPtr := a.value.Swap(newPtr)
	a.version.Add(1)
	a.dirty.Store(true)

	if oldPtr == nil {
		return nil
	}
	return *oldPtr
}

// ApplyDeltas applies multiple deltas atomically
func (a *AtomicPointer) ApplyDeltas(deltas ...sync.Delta[unsafe.Pointer]) unsafe.Pointer {
	deltas = sequence.From(deltas).
		Sort(func(a, b sync.Delta[unsafe.Pointer]) bool {
			return a.Version <= b.Version
		}).Collect()

	var setValue *unsafe.Pointer

	for _, delta := range deltas {
		switch delta.OpType {
		case sync.OpSet, sync.OpUpdate:
			setValue = &delta.Value
		default:
		}
	}

	if setValue != nil {
		a.Set(*setValue)
		return *setValue
	}

	return a.Get()
}

// CompareAndSwap performs an atomic compare-and-swap operation
func (a *AtomicPointer) CompareAndSwap(old, new unsafe.Pointer) bool {
	currentPtr := a.value.Load()

	var currentVal unsafe.Pointer
	if currentPtr != nil {
		currentVal = *currentPtr
	}

	if currentVal != old {
		return false
	}

	var newPtr *unsafe.Pointer
	if new != nil {
		newPtr = &new
	}

	if a.value.CompareAndSwap(currentPtr, newPtr) {
		a.version.Add(1)
		a.dirty.Store(true)
		return true
	}
	return false
}

// Version returns the current version number
func (a *AtomicPointer) Version() uint64 {
	return a.version.Load()
}

// IsDirty returns true if the value has been modified since last clean
func (a *AtomicPointer) IsDirty() bool {
	return a.dirty.Load()
}

// MarkClean marks the value as clean
func (a *AtomicPointer) MarkClean() {
	a.dirty.Store(false)
}
