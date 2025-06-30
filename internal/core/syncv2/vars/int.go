package vars

import (
	"encoding/binary"
	"errors"
	"sort"
	sc "sync"
	"sync/atomic"
	"time"

	"github.com/zeusync/zeusync/internal/core/sync"
	"github.com/zeusync/zeusync/internal/core/sync/resolver"
)

var _ sync.TypedVariable[int64] = (*AtomicInt)(nil)

type AtomicInt struct {
	valueMu   sc.RWMutex // Mutex for protecting access to the value
	historyMu sc.Mutex   // Mutex for protecting access to the history

	value   atomic.Int64  // The value of the variable
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

	onChange         atomic.Pointer[func(old, new int64)]              // Callback for value changes
	onConflict       atomic.Pointer[func(local, remote int64) int64]   // Callback for conflicts
	conflictResolver atomic.Pointer[sync.TypedConflictResolver[int64]] // Conflict resolver

	tags map[string]string // Tags for categorizing and filtering
}

func NewInt(cfg sync.VariableConfig) *AtomicInt {
	v := &AtomicInt{
		maxHistory:      cfg.MaxHistory,
		permissionMask:  cfg.Permissions,
		storageStrategy: cfg.StorageStrategy,
		enabledMetrics:  cfg.EnableMetrics,
		enabledHistory:  cfg.EnableHistory,
		tll:             cfg.TTL,
		tags:            cfg.Tags,
	}

	v.value.Store(1)
	v.version.Store(1)

	if v.enabledHistory {
		v.history = make([]sync.Delta, v.maxHistory)
	}

	return v
}

func (v *AtomicInt) Get() (int64, error) {
	return v.value.Load(), nil
}

func (v *AtomicInt) Set(newValue int64) error {
	if !checkPermissions(sync.PermissionWrite, v.getPermissionMask()) {
		return errors.New("permission denied")
	}

	oldValue := v.value.Load()

	if oldValue == newValue {
		res := v.conflictResolver.Load()
		onConflictFunc := v.onConflict.Load()

		if res != nil {
			newValue = (*res).Resolve(oldValue, newValue, make(map[string]any))
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

func (v *AtomicInt) updateHistory(delta sync.Delta) {
	v.historyMu.Lock()
	defer v.historyMu.Unlock()

	if len(v.history) == 0 {
		return
	}

	v.history[v.historyIndex] = delta
	v.historyIndex = (v.historyIndex + 1) % len(v.history)
}

func (v *AtomicInt) IsDirty() bool {
	return v.dirty.Load()
}

func (v *AtomicInt) MarkClean() {
	v.dirty.Store(false)
}

func (v *AtomicInt) GetVersion() uint64 {
	return v.version.Load()
}

func (v *AtomicInt) GetDelta(sinceVersion uint64) ([]sync.Delta, error) {
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

func (v *AtomicInt) ApplyDelta(deltas ...sync.Delta) error {
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

func (v *AtomicInt) SetConflictResolver(res sync.ConflictResolver) {
	r := resolver.NewTypedResolver[int64](res)
	var ir sync.TypedConflictResolver[int64] = r
	v.conflictResolver.Store(&ir)
}

func (v *AtomicInt) OnChange(eventHandler func(oldValue int64, newValue int64)) {
	v.onChange.Store(&eventHandler)
}

func (v *AtomicInt) OnConflict(eventHandler func(local int64, remote int64) int64) {
	v.onConflict.Store(&eventHandler)
}

func (v *AtomicInt) GetPermissions() sync.PermissionMask {
	return v.getPermissionMask()
}

func (v *AtomicInt) SetPermissions(mask sync.PermissionMask) {
	v.valueMu.Lock()
	v.permissionMask = mask
	v.valueMu.Unlock()
}

func (v *AtomicInt) getPermissionMask() sync.PermissionMask {
	v.valueMu.RLock()
	defer v.valueMu.RUnlock()
	return v.permissionMask
}

func (v *AtomicInt) GetHistory() []sync.HistoryEntry {
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

func (v *AtomicInt) GetMetrics() sync.VariableMetrics {
	return sync.VariableMetrics{}
}

func (v *AtomicInt) GetStorageStrategy() sync.StorageStrategy {
	return v.storageStrategy
}

func (v *AtomicInt) CanMigrateTo(strategy sync.StorageStrategy) bool {
	return strategy != v.storageStrategy
}

func (v *AtomicInt) Close() error {
	return nil
}

func (v *AtomicInt) Size() int64 {
	return int64(binary.Size(v.value.Load()))
}

func (v *AtomicInt) AsUntyped() sync.Variable {
	return nil
}
