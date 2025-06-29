package vars

import (
	"errors"
	"reflect"
	"sort"
	sc "sync"
	"sync/atomic"
	"time"

	"github.com/zeusync/zeusync/internal/core/sync"
)

var _ sync.Variable = (*SyncVarV2)(nil)

type SyncVarV2 struct {
	valueMu   sc.RWMutex
	historyMu sc.Mutex

	value   atomic.Value
	version atomic.Uint64
	dirty   atomic.Bool

	permissionMask sync.PermissionMask
	history        []sync.Delta
	historyIndex   int

	onChange         atomic.Pointer[func(old, new any)]
	onConflict       atomic.Pointer[func(local, remote any) any]
	conflictResolver atomic.Pointer[sync.ConflictResolver]

	maxHistory uint8
}

func NewSyncVarV2(initialValue any, maxHistory uint8) *SyncVarV2 {
	s := &SyncVarV2{
		maxHistory: maxHistory,
	}

	s.value.Store(initialValue)
	s.version.Store(1)

	if maxHistory > 0 {
		s.history = make([]sync.Delta, maxHistory)
	}

	return s
}

func (s *SyncVarV2) Get() (any, error) {
	return s.value.Load(), nil
}

func (s *SyncVarV2) Set(newValue any) error {
	if !checkPermissions(sync.PermissionWrite, s.getPermissionMask()) {
		return errors.New("permission denied")
	}

	oldValue := s.value.Load()

	if fastEqual(oldValue, newValue) {
		resolver := s.conflictResolver.Load()
		onConflictFunc := s.onConflict.Load()

		if resolver != nil {
			newValue = (*resolver).Resolve(oldValue, newValue, make(map[string]any))
		} else if onConflictFunc != nil {
			newValue = (*onConflictFunc)(oldValue, newValue)
		} else {
			return nil
		}
	}

	s.value.Store(newValue)
	newVersion := s.version.Add(1)
	s.dirty.Store(true)

	if s.maxHistory > 0 {
		s.updateHistory(sync.Delta{
			Version:       newVersion,
			PreviousValue: oldValue,
			Value:         newValue,
		})
	}

	if onChangeFunc := s.onChange.Load(); onChangeFunc != nil {
		go (*onChangeFunc)(oldValue, newValue)
	}

	return nil
}

func (s *SyncVarV2) updateHistory(delta sync.Delta) {
	s.historyMu.Lock()
	defer s.historyMu.Unlock()

	if len(s.history) == 0 {
		return
	}

	// Кольцевой буфер
	s.history[s.historyIndex] = delta
	s.historyIndex = (s.historyIndex + 1) % len(s.history)
}

func (s *SyncVarV2) IsDirty() bool {
	return s.dirty.Load()
}

func (s *SyncVarV2) MarkClean() {
	s.dirty.Store(false)
}

func (s *SyncVarV2) GetDelta(sinceVersion uint64) ([]sync.Delta, error) {
	s.historyMu.Lock()
	defer s.historyMu.Unlock()

	if s.history == nil {
		return nil, errors.New("history is empty")
	}

	currentVersion := s.version.Load()
	if sinceVersion >= currentVersion {
		return nil, nil
	}

	var deltas []sync.Delta
	for _, d := range s.history {
		if d.Version > sinceVersion && d.Version != 0 {
			deltas = append(deltas, d)
		}
	}

	return deltas, nil
}

func (s *SyncVarV2) ApplyDelta(deltas ...sync.Delta) error {
	if len(deltas) == 0 {
		return errors.New("no deltas provided")
	}

	sort.Slice(deltas, func(i, j int) bool {
		return deltas[i].Version < deltas[j].Version
	})

	s.historyMu.Lock()
	defer s.historyMu.Unlock()

	if s.maxHistory > 0 {
		for _, delta := range deltas {
			s.history[s.historyIndex] = delta
			s.historyIndex = (s.historyIndex + 1) % len(s.history)
		}
	}

	return nil
}

func (s *SyncVarV2) SetConflictResolver(resolver sync.ConflictResolver) {
	s.conflictResolver.Store(&resolver)
}

func (s *SyncVarV2) GetVersion() uint64 {
	return s.version.Load()
}

func (s *SyncVarV2) OnChange(eventHandler func(oldValue any, newValue any)) {
	s.onChange.Store(&eventHandler)
}

func (s *SyncVarV2) OnConflict(eventHandler func(local any, remote any) any) {
	s.onConflict.Store(&eventHandler)
}

func (s *SyncVarV2) GetPermissions() sync.PermissionMask {
	return s.getPermissionMask()
}

func (s *SyncVarV2) SetPermissions(mask sync.PermissionMask) {
	s.valueMu.Lock()
	defer s.valueMu.Unlock()
	s.permissionMask = mask
}

func (s *SyncVarV2) getPermissionMask() sync.PermissionMask {
	s.valueMu.RLock()
	defer s.valueMu.RUnlock()
	return s.permissionMask
}

func (s *SyncVarV2) GetType() reflect.Type {
	return reflect.TypeOf(s.value.Load())
}

func (s *SyncVarV2) GetHistory() []sync.HistoryEntry {
	return []sync.HistoryEntry{
		{
			Version:   s.version.Load(),
			Timestamp: time.Now().UnixNano(),
			Value:     s.value.Load(),
			ClientID:  "unknown",
		},
	}
}

var _ sync.TypedVariable[any] = (*TypedSyncVarV2[any])(nil)

// TypedSyncVarV2 is a generic implementation of sync.TypedVariable[T]
// This implementation provides basic synchronization and versioning for any type T.
// It also provides methods for setting and getting the value, checking if the value is dirty,
type TypedSyncVarV2[T any] struct {
	root *SyncVarV2
}

func NewTypedSyncVarV2[T any](initialValue T, maxHistory uint8) *TypedSyncVarV2[T] {
	return &TypedSyncVarV2[T]{
		root: NewSyncVarV2(initialValue, maxHistory),
	}
}

func (v *TypedSyncVarV2[T]) Get() (T, error) {
	val := v.root.value.Load()
	if typed, ok := val.(T); ok {
		return typed, nil
	}

	var zero T
	return zero, errors.New("type mismatch")
}

func (v *TypedSyncVarV2[T]) Set(newValue T) error {
	return v.root.Set(newValue)
}

func (v *TypedSyncVarV2[T]) IsDirty() bool {
	return v.root.IsDirty()
}

func (v *TypedSyncVarV2[T]) MarkClean() {
	v.root.MarkClean()
}

func (v *TypedSyncVarV2[T]) GetDelta(sinceVersion uint64) ([]sync.Delta, error) {
	return v.root.GetDelta(sinceVersion)
}

func (v *TypedSyncVarV2[T]) ApplyDelta(delta ...sync.Delta) error {
	return v.root.ApplyDelta(delta...)
}

func (v *TypedSyncVarV2[T]) SetConflictResolver(resolver sync.ConflictResolver) {
	v.root.SetConflictResolver(resolver)
}

func (v *TypedSyncVarV2[T]) GetVersion() uint64 {
	return v.root.GetVersion()
}

func (v *TypedSyncVarV2[T]) OnChange(eventHandler func(oldValue T, newValue T)) {
	v.root.OnChange(func(oldValue, newValue any) {
		if oldTyped, fOk := oldValue.(T); fOk {
			if newTyped, sOk := newValue.(T); sOk {
				eventHandler(oldTyped, newTyped)
			}
		}
	})
}

func (v *TypedSyncVarV2[T]) OnConflict(eventHandler func(local T, remote T) T) {
	v.root.OnConflict(func(local, remote any) any {
		if localTyped, fOk := local.(T); fOk {
			if remoteTyped, sOk := remote.(T); sOk {
				return eventHandler(localTyped, remoteTyped)
			}
		}
		return local
	})
}

func (v *TypedSyncVarV2[T]) GetPermissionMask() sync.PermissionMask {
	return v.root.GetPermissions()
}

func (v *TypedSyncVarV2[T]) SetPermissionMask(mask sync.PermissionMask) {
	v.root.SetPermissions(mask)
}

func (v *TypedSyncVarV2[T]) GetHistory() []sync.HistoryEntry {
	return v.root.GetHistory()
}
