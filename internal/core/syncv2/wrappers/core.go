package wrappers

import sync "github.com/zeusync/zeusync/internal/core/syncv2"

// VariableWrapper wraps a Core implementation to provide the simple Variable interface
type VariableWrapper[T any] struct {
	core        sync.Core[T]
	subscribers []func(T, T)
}

// NewVariableWrapper creates a new VariableWrapper around the given Core
func NewVariableWrapper[T any](core sync.Core[T]) *VariableWrapper[T] {
	return &VariableWrapper[T]{
		core:        core,
		subscribers: make([]func(T, T), 0),
	}
}

// Variable interface implementation

func (v *VariableWrapper[T]) Get() T {
	return v.core.Get()
}

func (v *VariableWrapper[T]) Set(value T) {
	old := v.core.Get()
	v.core.Set(value)

	// Notify local subscribers
	for _, callback := range v.subscribers {
		if callback != nil {
			callback(old, value)
		}
	}
}

func (v *VariableWrapper[T]) Update(callback func(current T) T) T {
	for {
		current := v.core.Get()
		newValue := callback(current)

		// Try to update atomically using the core's capabilities
		if coreWithCAS, ok := v.core.(interface{ CompareAndSwap(old, new T) bool }); ok {
			if coreWithCAS.CompareAndSwap(current, newValue) {
				// Notify subscribers
				for _, sub := range v.subscribers {
					if sub != nil {
						sub(current, newValue)
					}
				}
				return newValue
			}
			// Retry if CAS failed
			continue
		} else {
			// Fallback to simple set if CAS is not available
			v.Set(newValue)
			return newValue
		}
	}
}

func (v *VariableWrapper[T]) Subscribe(callback func(oldValue, newValue T)) sync.UnsubscribeFunc {
	v.subscribers = append(v.subscribers, callback)
	index := len(v.subscribers) - 1

	// Return unsubscribe function
	return func() {
		if index < len(v.subscribers) {
			// Remove callback by setting to nil and compacting later
			v.subscribers[index] = nil
			// Compact the slice to remove nil entries
			compacted := make([]func(T, T), 0, len(v.subscribers))
			for _, cb := range v.subscribers {
				if cb != nil {
					compacted = append(compacted, cb)
				}
			}
			v.subscribers = compacted
		}
	}
}

func (v *VariableWrapper[T]) GetVersion() uint64 {
	return v.core.Version()
}

func (v *VariableWrapper[T]) Close() error {
	v.subscribers = nil
	return v.core.Close()
}
