package vars

import (
	"encoding/binary"
	"errors"
	"reflect"
	"sort"
	sc "sync"
	"sync/atomic"
	"time"

	"github.com/zeusync/zeusync/internal/core/sync"
)

var _ sync.Variable = (*Atomic)(nil)

// Atomic implements the sync.Variable interface using atomic operations.
// This provides a basic level of synchronization and is suitable for simple data types.
type Atomic struct {
	valueMu   sc.RWMutex // Mutex for protecting access to the value
	historyMu sc.Mutex   // Mutex for protecting access to the history

	value   atomic.Value  // The value of the variable
	version atomic.Uint64 // The version of the variable
	dirty   atomic.Bool   // Flag indicating if the variable has been modified

	permissionMask sync.PermissionMask // Access permissions for the variable
	history        []sync.Delta        // History of changes
	historyIndex   int                 // Index for the circular history buffer
	maxHistory     uint8               // Maximum number of history entries
	enabledHistory bool                // Flag to enable/disable history tracking

	storageStrategy sync.StorageStrategy // The storage strategy

	enabledMetrics bool          // Flag to enable/disable metrics collection
	tll            time.Duration // Time-to-live for the variable

	onChange         atomic.Pointer[func(old, new any)]          // Callback for value changes
	onConflict       atomic.Pointer[func(local, remote any) any] // Callback for conflicts
	conflictResolver atomic.Pointer[sync.ConflictResolver]       // Conflict resolver

	tags map[string]string // Tags for categorizing and filtering
}

// NewAtomicVariable creates a new Atomic variable.
func NewAtomicVariable(cfg sync.VariableConfig) *Atomic {
	v := &Atomic{
		maxHistory:      cfg.MaxHistory,
		permissionMask:  cfg.Permissions,
		storageStrategy: cfg.StorageStrategy,
		enabledMetrics:  cfg.EnableMetrics,
		enabledHistory:  cfg.EnableHistory,
		tll:             cfg.TTL,
		tags:            cfg.Tags,
	}

	if cfg.ConflictResolver != nil {
		v.conflictResolver.Store(&cfg.ConflictResolver)
	}

	v.version.Store(1)

	if v.enabledHistory {
		v.history = make([]sync.Delta, v.maxHistory)
	}

	return v
}

// Get returns the current value of the variable.
func (v *Atomic) Get() (any, error) {
	return v.value.Load(), nil
}

// Set updates the value of the variable.
func (v *Atomic) Set(newValue any) error {
	if !checkPermissions(sync.PermissionWrite, v.getPermissionMask()) {
		return errors.New("permission denied")
	}

	oldValue := v.value.Load()

	if fastEqual(oldValue, newValue) {
		resolver := v.conflictResolver.Load()
		onConflictFunc := v.onConflict.Load()

		if resolver != nil {
			newValue = (*resolver).Resolve(oldValue, newValue, make(map[string]any))
		} else if onConflictFunc != nil {
			newValue = (*onConflictFunc)(oldValue, newValue)
		} else {
			return nil
		}
	}

	v.value.Store(newValue)
	newVersion := v.version.Add(1)
	v.dirty.Store(true)

	if v.maxHistory > 0 {
		v.updateHistory(sync.Delta{
			Version:       newVersion,
			PreviousValue: oldValue,
			Value:         newValue,
		})
	}

	if onChangeFunc := v.onChange.Load(); onChangeFunc != nil {
		go (*onChangeFunc)(oldValue, newValue)
	}

	return nil
}

func (v *Atomic) updateHistory(delta sync.Delta) {
	v.historyMu.Lock()
	defer v.historyMu.Unlock()

	if len(v.history) == 0 {
		return
	}

	v.history[v.historyIndex] = delta
	v.historyIndex = (v.historyIndex + 1) % len(v.history)
}

// IsDirty returns true if the variable has been modified since the last sync.
func (v *Atomic) IsDirty() bool {
	return v.dirty.Load()
}

// MarkClean marks the variable as clean (not modified).
func (v *Atomic) MarkClean() {
	v.dirty.Store(false)
}

// GetDelta returns a list of changes since a specific version.
func (v *Atomic) GetDelta(sinceVersion uint64) ([]sync.Delta, error) {
	v.historyMu.Lock()
	defer v.historyMu.Unlock()

	if v.history == nil {
		return nil, errors.New("history is empty")
	}

	currentVersion := v.version.Load()
	if sinceVersion >= currentVersion {
		return nil, nil
	}

	var deltas []sync.Delta
	for _, d := range v.history {
		if d.Version > sinceVersion && d.Version != 0 {
			deltas = append(deltas, d)
		}
	}

	return deltas, nil
}

// ApplyDelta applies a list of changes to the variable.
func (v *Atomic) ApplyDelta(deltas ...sync.Delta) error {
	if len(deltas) == 0 {
		return errors.New("no deltas provided")
	}

	sort.Slice(deltas, func(i, j int) bool {
		return deltas[i].Version < deltas[j].Version
	})

	v.historyMu.Lock()
	defer v.historyMu.Unlock()

	if v.maxHistory > 0 {
		for _, delta := range deltas {
			v.history[v.historyIndex] = delta
			v.historyIndex = (v.historyIndex + 1) % len(v.history)
		}
	}

	return nil
}

// SetConflictResolver sets the conflict resolver for the variable.
func (v *Atomic) SetConflictResolver(resolver sync.ConflictResolver) {
	v.conflictResolver.Store(&resolver)
}

// GetVersion returns the current version of the variable.
func (v *Atomic) GetVersion() uint64 {
	return v.version.Load()
}

// OnChange registers a callback function to be called when the variable's value changes.
func (v *Atomic) OnChange(eventHandler func(oldValue any, newValue any)) {
	v.onChange.Store(&eventHandler)
}

// OnConflict registers a callback function to be called when a conflict occurs.
func (v *Atomic) OnConflict(eventHandler func(local any, remote any) any) {
	v.onConflict.Store(&eventHandler)
}

// GetPermissions returns the access permissions for the variable.
func (v *Atomic) GetPermissions() sync.PermissionMask {
	return v.getPermissionMask()
}

// SetPermissions sets the access permissions for the variable.
func (v *Atomic) SetPermissions(mask sync.PermissionMask) {
	v.valueMu.Lock()
	defer v.valueMu.Unlock()
	v.permissionMask = mask
}

// GetMetrics returns the performance metrics for the variable.
func (v *Atomic) GetMetrics() sync.VariableMetrics {
	return sync.VariableMetrics{}
}

// GetStorageStrategy returns the current storage strategy of the variable.
func (v *Atomic) GetStorageStrategy() sync.StorageStrategy {
	return v.storageStrategy
}

// CanMigrateTo checks if the variable can be migrated to a new storage strategy.
func (v *Atomic) CanMigrateTo(strategy sync.StorageStrategy) bool {
	return strategy != v.storageStrategy
}

// GetType returns the type of the variable's value.
func (v *Atomic) GetType() reflect.Type {
	return reflect.TypeOf(v.value.Load())
}

func (v *Atomic) GetHistory() []sync.HistoryEntry {
	v.historyMu.Lock()
	defer v.historyMu.Unlock()

	if v.history == nil {
		return nil
	}

	historyEntries := make([]sync.HistoryEntry, 0, len(v.history))
	for _, d := range v.history {
		if d.Version != 0 {
			historyEntries = append(historyEntries, sync.HistoryEntry{
				Version:   d.Version,
				Timestamp: d.Timestamp.UnixNano(),
				Value:     d.Value,
				ClientID:  d.ClientID,
			})
		}
	}

	return historyEntries
}

// Close releases any resources used by the variable.
func (v *Atomic) Close() error {
	return nil
}

// Size returns the memory footprint of the variable in bytes.
func (v *Atomic) Size() int64 {
	return int64(binary.Size(v.value.Load()))
}

func (v *Atomic) getPermissionMask() sync.PermissionMask {
	v.valueMu.RLock()
	defer v.valueMu.RUnlock()
	return v.permissionMask
}

var _ sync.TypedVariable[any] = (*AtomicTyped[any])(nil)

// AtomicTyped is a generic implementation of sync.TypedVariable[T]
// This implementation provides basic synchronization and versioning for any type T.
// It also provides methods for setting and getting the value, checking if the value is dirty,
type AtomicTyped[T any] struct {
	root *Atomic
}

// NewAtomicTypedVariable creates a new type-safe Atomic variable.
func NewAtomicTypedVariable[T any](cfg sync.VariableConfig) *AtomicTyped[T] {
	return &AtomicTyped[T]{
		root: NewAtomicVariable(cfg),
	}
}

// Get returns the current value of the variable.
func (v *AtomicTyped[T]) Get() (T, error) {
	val := v.root.value.Load()
	if typed, ok := val.(T); ok {
		return typed, nil
	}

	var zero T
	return zero, errors.New("type mismatch")
}

// Set updates the value of the variable.
func (v *AtomicTyped[T]) Set(newValue T) error {
	return v.root.Set(newValue)
}

// IsDirty returns true if the variable has been modified since the last sync.
func (v *AtomicTyped[T]) IsDirty() bool {
	return v.root.IsDirty()
}

// MarkClean marks the variable as clean (not modified).
func (v *AtomicTyped[T]) MarkClean() {
	v.root.MarkClean()
}

// GetDelta returns a list of changes since a specific version.
func (v *AtomicTyped[T]) GetDelta(sinceVersion uint64) ([]sync.Delta, error) {
	return v.root.GetDelta(sinceVersion)
}

// ApplyDelta applies a list of changes to the variable.
func (v *AtomicTyped[T]) ApplyDelta(delta ...sync.Delta) error {
	return v.root.ApplyDelta(delta...)
}

// SetConflictResolver sets the conflict resolver for the variable.
func (v *AtomicTyped[T]) SetConflictResolver(resolver sync.ConflictResolver) {
	v.root.SetConflictResolver(resolver)
}

// GetVersion returns the current version of the variable.
func (v *AtomicTyped[T]) GetVersion() uint64 {
	return v.root.GetVersion()
}

// OnChange registers a callback function to be called when the variable's value changes.
func (v *AtomicTyped[T]) OnChange(eventHandler func(oldValue T, newValue T)) {
	v.root.OnChange(func(oldValue, newValue any) {
		if oldTyped, fOk := oldValue.(T); fOk {
			if newTyped, sOk := newValue.(T); sOk {
				eventHandler(oldTyped, newTyped)
			}
		}
	})
}

// OnConflict registers a callback function to be called when a conflict occurs.
func (v *AtomicTyped[T]) OnConflict(eventHandler func(local T, remote T) T) {
	v.root.OnConflict(func(local, remote any) any {
		if localTyped, fOk := local.(T); fOk {
			if remoteTyped, sOk := remote.(T); sOk {
				return eventHandler(localTyped, remoteTyped)
			}
		}
		return local
	})
}

// GetPermissions returns the access permissions for the variable.
func (v *AtomicTyped[T]) GetPermissions() sync.PermissionMask {
	return v.root.GetPermissions()
}

// SetPermissions sets the access permissions for the variable.
func (v *AtomicTyped[T]) SetPermissions(mask sync.PermissionMask) {
	v.root.SetPermissions(mask)
}

// GetMetrics returns the performance metrics for the variable.
func (v *AtomicTyped[T]) GetMetrics() sync.VariableMetrics {
	return sync.VariableMetrics{}
}

// GetStorageStrategy returns the current storage strategy of the variable.
func (v *AtomicTyped[T]) GetStorageStrategy() sync.StorageStrategy {
	return sync.StrategyAtomic
}

// CanMigrateTo checks if the variable can be migrated to a new storage strategy.
func (v *AtomicTyped[T]) CanMigrateTo(strategy sync.StorageStrategy) bool {
	return false
}

// AsUntyped returns the underlying non-type-safe variable.
func (v *AtomicTyped[T]) AsUntyped() sync.Variable {
	return v.root
}

// GetHistory returns the history of changes for the variable.
func (v *AtomicTyped[T]) GetHistory() []sync.HistoryEntry {
	return v.root.GetHistory()
}

// Close releases any resources used by the variable.
func (v *AtomicTyped[T]) Close() error {
	return nil
}

// Size returns the memory footprint of the variable in bytes.
func (v *AtomicTyped[T]) Size() int64 {
	return int64(binary.Size(v.root.value.Load()))
}
