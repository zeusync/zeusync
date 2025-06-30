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

var _ sync.Variable = (*Mutex)(nil)

// Mutex implements the sync.Variable interface using a read-write mutex.
// This provides a good balance of performance and safety for general use cases.
type Mutex struct {
	valueMu   sc.RWMutex // Mutex for protecting access to the value
	historyMu sc.Mutex   // Mutex for protecting access to the history

	value   any           // The value of the variable
	version atomic.Uint64 // The version of the variable
	dirty   bool          // Flag indicating if the variable has been modified

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

// NewMutexVariable creates a new Mutex variable.
func NewMutexVariable(cfg sync.VariableConfig) *Mutex {
	v := &Mutex{
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
func (v *Mutex) Get() (any, error) {
	v.valueMu.RLock()
	defer v.valueMu.RUnlock()
	return v.value, nil
}

// Set updates the value of the variable.
func (v *Mutex) Set(newValue any) error {
	if !checkPermissions(sync.PermissionWrite, v.getPermissionMask()) {
		return errors.New("permission denied")
	}

	v.valueMu.Lock()
	defer v.valueMu.Unlock()

	oldValue := v.value
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

	v.value = newValue
	newVersion := v.version.Add(1)
	v.dirty = true

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

func (v *Mutex) updateHistory(delta sync.Delta) {
	v.historyMu.Lock()
	defer v.historyMu.Unlock()

	if len(v.history) == 0 {
		return
	}

	v.history[v.historyIndex] = delta
	v.historyIndex = (v.historyIndex + 1) % len(v.history)
}

func (v *Mutex) getPermissionMask() sync.PermissionMask {
	v.valueMu.RLock()
	defer v.valueMu.RUnlock()
	return v.permissionMask
}

// IsDirty returns true if the variable has been modified since the last sync.
func (v *Mutex) IsDirty() bool {
	v.valueMu.RLock()
	defer v.valueMu.RUnlock()
	return v.dirty
}

// MarkClean marks the variable as clean (not modified).
func (v *Mutex) MarkClean() {
	v.valueMu.Lock()
	defer v.valueMu.Unlock()
	v.dirty = false
}

// GetDelta returns a list of changes since a specific version.
func (v *Mutex) GetDelta(sinceVersion uint64) ([]sync.Delta, error) {
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
func (v *Mutex) ApplyDelta(deltas ...sync.Delta) error {
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

	v.valueMu.Lock()
	v.valueMu.Unlock()
	lastDelta := deltas[len(deltas)-1]
	if lastDelta.Version > v.version.Load() {
		v.value = lastDelta.Value
	}

	return nil
}

// SetConflictResolver sets the conflict resolver for the variable.
func (v *Mutex) SetConflictResolver(resolver sync.ConflictResolver) {
	v.conflictResolver.Store(&resolver)
}

// GetVersion returns the current version of the variable.
func (v *Mutex) GetVersion() uint64 {
	return v.version.Load()
}

// OnChange registers a callback function to be called when the variable's value changes.
func (v *Mutex) OnChange(eventHandler func(oldValue any, newValue any)) {
	v.onChange.Store(&eventHandler)
}

// OnConflict registers a callback function to be called when a conflict occurs.
func (v *Mutex) OnConflict(eventHandler func(local any, remote any) any) {
	v.onConflict.Store(&eventHandler)
}

// GetPermissions returns the access permissions for the variable.
func (v *Mutex) GetPermissions() sync.PermissionMask {
	return v.getPermissionMask()
}

// SetPermissions sets the access permissions for the variable.
func (v *Mutex) SetPermissions(mask sync.PermissionMask) {
	v.valueMu.Lock()
	v.permissionMask = mask
	v.valueMu.Unlock()
}

// GetMetrics returns the performance metrics for the variable.
func (v *Mutex) GetMetrics() sync.VariableMetrics {
	v.valueMu.RLock()
	defer v.valueMu.RUnlock()
	// This is a simplified implementation. In a real-world scenario, you would
	// collect and return actual metrics.
	return sync.VariableMetrics{
		ReadCount:  v.version.Load(), // Just an example
		WriteCount: v.version.Load(), // Just an example
	}
}

// GetStorageStrategy returns the current storage strategy of the variable.
func (v *Mutex) GetStorageStrategy() sync.StorageStrategy {
	return v.storageStrategy
}

// CanMigrateTo checks if the variable can be migrated to a new storage strategy.
func (v *Mutex) CanMigrateTo(strategy sync.StorageStrategy) bool {
	return strategy != v.storageStrategy
}

// GetType returns the type of the variable's value.
func (v *Mutex) GetType() reflect.Type {
	return reflect.TypeOf(v.value)
}

// GetHistory returns the history of changes for the variable.
func (v *Mutex) GetHistory() []sync.HistoryEntry {
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
func (v *Mutex) Close() error {
	return nil
}

// Size returns the memory footprint of the variable in bytes.
func (v *Mutex) Size() int64 {
	return int64(binary.Size(v.value))
}

var _ sync.TypedVariable[any] = (*MutexTyped[any])(nil)

// MutexTyped is a generic implementation of sync.TypedVariable[T]
// This implementation provides basic synchronization and versioning for any type T.
// It also provides methods for setting and getting the value, checking if the value is dirty,
type MutexTyped[T any] struct {
	mu   sc.Mutex
	root *Mutex
}

// NewMutexTypedVariable creates a new type-safe Mutex variable.
func NewMutexTypedVariable[T any](cfg sync.VariableConfig) *MutexTyped[T] {
	return &MutexTyped[T]{
		root: NewMutexVariable(cfg),
	}
}

// Get returns the current value of the variable.
func (v *MutexTyped[T]) Get() (T, error) {
	res, err := v.root.Get()
	if err != nil {
		return v.getDefaultValue(res), err
	}

	if val, ok := res.(T); ok {
		return val, nil
	} else {
		return v.getDefaultValue(res), errors.New("type mismatch")
	}
}

// Set updates the value of the variable.
func (v *MutexTyped[T]) Set(newValue T) error {
	return v.root.Set(newValue)
}

// IsDirty returns true if the variable has been modified since the last sync.
func (v *MutexTyped[T]) IsDirty() bool {
	return v.root.IsDirty()
}

// MarkClean marks the variable as clean (not modified).
func (v *MutexTyped[T]) MarkClean() {
	v.root.MarkClean()
}

// GetDelta returns a list of changes since a specific version.
func (v *MutexTyped[T]) GetDelta(sinceVersion uint64) ([]sync.Delta, error) {
	return v.root.GetDelta(sinceVersion)
}

// ApplyDelta applies a list of changes to the variable.
func (v *MutexTyped[T]) ApplyDelta(delta ...sync.Delta) error {
	return v.root.ApplyDelta(delta...)
}

// SetConflictResolver sets the conflict resolver for the variable.
func (v *MutexTyped[T]) SetConflictResolver(resolver sync.ConflictResolver) {
	v.root.SetConflictResolver(resolver)
}

// GetVersion returns the current version of the variable.
func (v *MutexTyped[T]) GetVersion() uint64 {
	return v.root.GetVersion()
}

// OnChange registers a callback function to be called when the variable's value changes.
func (v *MutexTyped[T]) OnChange(eventHandler func(oldValue T, newValue T)) {
	v.root.OnChange(func(oldValue, newValue any) {
		eventHandler(oldValue.(T), newValue.(T))
	})
}

// OnConflict registers a callback function to be called when a conflict occurs.
func (v *MutexTyped[T]) OnConflict(eventHandler func(local T, remote T) T) {
	v.root.OnConflict(func(local, remote any) any {
		return eventHandler(local.(T), remote.(T))
	})
}

// GetPermissions returns the access permissions for the variable.
func (v *MutexTyped[T]) GetPermissions() sync.PermissionMask {
	return v.root.GetPermissions()
}

// SetPermissions sets the access permissions for the variable.
func (v *MutexTyped[T]) SetPermissions(mask sync.PermissionMask) {
	v.root.SetPermissions(mask)
}

// GetMetrics returns the performance metrics for the variable.
func (v *MutexTyped[T]) GetMetrics() sync.VariableMetrics {
	return sync.VariableMetrics{}
}

// GetStorageStrategy returns the current storage strategy of the variable.
func (v *MutexTyped[T]) GetStorageStrategy() sync.StorageStrategy {
	return v.root.storageStrategy
}

// CanMigrateTo checks if the variable can be migrated to a new storage strategy.
func (v *MutexTyped[T]) CanMigrateTo(strategy sync.StorageStrategy) bool {
	return false
}

// Close releases any resources used by the variable.
func (v *MutexTyped[T]) Close() error {
	return nil
}

// Size returns the memory footprint of the variable in bytes.
func (v *MutexTyped[T]) Size() int64 {
	return v.root.Size()
}

// AsUntyped returns the underlying non-type-safe variable.
func (v *MutexTyped[T]) AsUntyped() sync.Variable {
	return v.root
}

// GetHistory returns the history of changes for the variable.
func (v *MutexTyped[T]) GetHistory() []sync.HistoryEntry {
	return v.root.GetHistory()
}

func (v *MutexTyped[T]) getDefaultValue(val any) T {
	return reflect.Zero(reflect.TypeOf(val)).Interface().(T)
}

func fastEqual(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	typeA := reflect.TypeOf(a)
	typeB := reflect.TypeOf(b)
	if typeA != typeB {
		return false
	}

	switch typeA.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return reflect.ValueOf(a).Int() == reflect.ValueOf(b).Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return reflect.ValueOf(a).Uint() == reflect.ValueOf(b).Uint()
	case reflect.Float32, reflect.Float64:
		return reflect.ValueOf(a).Float() == reflect.ValueOf(b).Float()
	case reflect.String:
		return reflect.ValueOf(a).String() == reflect.ValueOf(b).String()
	case reflect.Bool:
		return reflect.ValueOf(a).Bool() == reflect.ValueOf(b).Bool()
	default:
		return reflect.DeepEqual(a, b)
	}
}

func checkPermissions(action sync.PermissionMask, mask sync.PermissionMask) bool {
	return action&mask != 0
}
