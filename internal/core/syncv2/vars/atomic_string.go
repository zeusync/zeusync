package vars

import (
	"sync/atomic"

	"github.com/zeusync/zeusync/internal/core/syncv2"
	"github.com/zeusync/zeusync/pkg/sequence"
)

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
