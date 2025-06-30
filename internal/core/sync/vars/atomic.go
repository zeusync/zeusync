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

type Atomic struct {
	valueMu   sc.RWMutex
	historyMu sc.Mutex

	value   atomic.Value
	version atomic.Uint64
	dirty   atomic.Bool

	permissionMask sync.PermissionMask
	history        []sync.Delta
	historyIndex   int
	maxHistory     uint8
	enabledHistory bool

	storageStrategy sync.StorageStrategy

	enabledMetrics bool
	tll            time.Duration

	onChange         atomic.Pointer[func(old, new any)]
	onConflict       atomic.Pointer[func(local, remote any) any]
	conflictResolver atomic.Pointer[sync.ConflictResolver]

	tags map[string]string
}

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

func (v *Atomic) Get() (any, error) {
	return v.value.Load(), nil
}

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

	// Кольцевой буфер
	v.history[v.historyIndex] = delta
	v.historyIndex = (v.historyIndex + 1) % len(v.history)
}

func (v *Atomic) IsDirty() bool {
	return v.dirty.Load()
}

func (v *Atomic) MarkClean() {
	v.dirty.Store(false)
}

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

func (v *Atomic) SetConflictResolver(resolver sync.ConflictResolver) {
	v.conflictResolver.Store(&resolver)
}

func (v *Atomic) GetVersion() uint64 {
	return v.version.Load()
}

func (v *Atomic) OnChange(eventHandler func(oldValue any, newValue any)) {
	v.onChange.Store(&eventHandler)
}

func (v *Atomic) OnConflict(eventHandler func(local any, remote any) any) {
	v.onConflict.Store(&eventHandler)
}

func (v *Atomic) GetPermissions() sync.PermissionMask {
	return v.getPermissionMask()
}

func (v *Atomic) SetPermissions(mask sync.PermissionMask) {
	v.valueMu.Lock()
	defer v.valueMu.Unlock()
	v.permissionMask = mask
}

func (v *Atomic) GetMetrics() sync.VariableMetrics {
	return sync.VariableMetrics{}
}

func (v *Atomic) GetStorageStrategy() sync.StorageStrategy {
	return v.storageStrategy
}

func (v *Atomic) CanMigrateTo(strategy sync.StorageStrategy) bool {
	return false
}

func (v *Atomic) GetType() reflect.Type {
	return reflect.TypeOf(v.value.Load())
}

func (v *Atomic) GetHistory() []sync.HistoryEntry {
	return []sync.HistoryEntry{
		{
			Version:   v.version.Load(),
			Timestamp: time.Now().UnixNano(),
			Value:     v.value.Load(),
			ClientID:  "unknown",
		},
	}
}

func (v *Atomic) Close() error {
	return nil
}

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

func NewAtomicTypedVariable[T any](cfg sync.VariableConfig) *AtomicTyped[T] {
	return &AtomicTyped[T]{
		root: NewAtomicVariable(cfg),
	}
}

func (v *AtomicTyped[T]) Get() (T, error) {
	val := v.root.value.Load()
	if typed, ok := val.(T); ok {
		return typed, nil
	}

	var zero T
	return zero, errors.New("type mismatch")
}

func (v *AtomicTyped[T]) Set(newValue T) error {
	return v.root.Set(newValue)
}

func (v *AtomicTyped[T]) IsDirty() bool {
	return v.root.IsDirty()
}

func (v *AtomicTyped[T]) MarkClean() {
	v.root.MarkClean()
}

func (v *AtomicTyped[T]) GetDelta(sinceVersion uint64) ([]sync.Delta, error) {
	return v.root.GetDelta(sinceVersion)
}

func (v *AtomicTyped[T]) ApplyDelta(delta ...sync.Delta) error {
	return v.root.ApplyDelta(delta...)
}

func (v *AtomicTyped[T]) SetConflictResolver(resolver sync.ConflictResolver) {
	v.root.SetConflictResolver(resolver)
}

func (v *AtomicTyped[T]) GetVersion() uint64 {
	return v.root.GetVersion()
}

func (v *AtomicTyped[T]) OnChange(eventHandler func(oldValue T, newValue T)) {
	v.root.OnChange(func(oldValue, newValue any) {
		if oldTyped, fOk := oldValue.(T); fOk {
			if newTyped, sOk := newValue.(T); sOk {
				eventHandler(oldTyped, newTyped)
			}
		}
	})
}

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

func (v *AtomicTyped[T]) GetPermissions() sync.PermissionMask {
	return v.root.GetPermissions()
}

func (v *AtomicTyped[T]) SetPermissions(mask sync.PermissionMask) {
	v.root.SetPermissions(mask)
}

func (v *AtomicTyped[T]) GetMetrics() sync.VariableMetrics {
	return sync.VariableMetrics{}
}

func (v *AtomicTyped[T]) GetStorageStrategy() sync.StorageStrategy {
	return sync.StrategyAtomic
}

func (v *AtomicTyped[T]) CanMigrateTo(strategy sync.StorageStrategy) bool {
	return false
}

func (v *AtomicTyped[T]) AsUntyped() sync.Variable {
	return v.root
}

func (v *AtomicTyped[T]) GetHistory() []sync.HistoryEntry {
	return v.root.GetHistory()
}

func (v *AtomicTyped[T]) Close() error {
	return nil
}

func (v *AtomicTyped[T]) Size() int64 {
	return int64(binary.Size(v.root.value.Load()))
}
