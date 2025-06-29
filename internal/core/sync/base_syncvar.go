package sync

import (
	"context"
	"reflect"
	"sync"
)

type BaseSyncVar[T any] struct {
	mu          sync.RWMutex
	value       T
	version     uint64
	dirty       bool
	permissions Permissions
	history     []HistoryEntry
	onChange    func(oldValue, newValue T)
}

func NewBaseSyncVar[T any](initialValue T) *BaseSyncVar[T] {
	return &BaseSyncVar[T]{
		value: initialValue,
	}
}

func (v *BaseSyncVar[T]) Get(ctx context.Context) (T, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.value, nil
}

func (v *BaseSyncVar[T]) Set(ctx context.Context, value T) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	oldValue := v.value
	v.value = value
	v.version++
	v.dirty = true

	if v.onChange != nil {
		v.onChange(oldValue, v.value)
	}

	return nil
}

func (v *BaseSyncVar[T]) IsDirty() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.dirty
}

func (v *BaseSyncVar[T]) MarkClean() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.dirty = false
}

func (v *BaseSyncVar[T]) GetVersion() uint64 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.version
}

func (v *BaseSyncVar[T]) GetPermissions() Permissions {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.permissions
}

func (v *BaseSyncVar[T]) SetPermissions(p Permissions) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.permissions = p
}

func (v *BaseSyncVar[T]) GetHistory() []HistoryEntry {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.history
}

func (v *BaseSyncVar[T]) OnChange(callback func(oldValue, newValue T)) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.onChange = callback
}

// The following methods are not implemented in the base class
// and should be implemented by specific SyncVar types.

func (v *BaseSyncVar[T]) GetDelta() ([]byte, error) {
	return nil, nil
}

func (v *BaseSyncVar[T]) ApplyDelta([]byte) error {
	return nil
}

func (v *BaseSyncVar[T]) SetConflictResolver(ConflictResolver) {}

func (v *BaseSyncVar[T]) OnConflict(func(local, remote T)) {}

func (v *BaseSyncVar[T]) GetType() reflect.Type {
	return nil
}

func (v *BaseSyncVar[T]) SetType(reflect.Type) {}

func (v *BaseSyncVar[T]) SetRoot(Variable) {}

func (v *BaseSyncVar[T]) GetRoot() Variable {
	return nil
}
