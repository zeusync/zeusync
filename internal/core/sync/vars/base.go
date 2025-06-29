package vars

import (
	"encoding/gob"
	"errors"
	"reflect"
	"sort"
	sc "sync"
	"time"

	"github.com/zeusync/zeusync/internal/core/sync"
)

var _ sync.Variable = (*SyncVar)(nil)

type SyncVar struct {
	mu               sc.RWMutex
	value            any
	version          uint64
	dirty            bool
	permissionMask   sync.PermissionMask // (e.g ["read", "write"])
	history          []sync.Delta        // Circle buffer
	onChange         func(old, new any)
	onConflict       func(local, remote any) any
	conflictResolver sync.ConflictResolver
	maxHistory       uint8
}

func NewSyncVar(initialValue any, maxHistory uint8) *SyncVar {
	gob.Register(initialValue)

	var history []sync.Delta = nil
	if maxHistory > 0 {
		history = make([]sync.Delta, 0, maxHistory)
	}

	return &SyncVar{
		mu:         sc.RWMutex{},
		value:      initialValue,
		version:    1,
		maxHistory: maxHistory,
		history:    history,
	}
}

func (s *SyncVar) Get() (any, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.version, nil
}

func (s *SyncVar) Set(newValue any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !checkPermissions(sync.PermissionWrite, s.permissionMask) {
		return errors.New("permission denied")
	}

	value := s.value
	if reflect.DeepEqual(value, newValue) {
		if s.conflictResolver != nil {
			value = s.conflictResolver.Resolve(value, newValue, make(map[string]any))
		} else if s.onConflict != nil {
			value = s.onConflict(value, newValue)
		} else {
			return nil
		}
	}

	if len(s.history) >= int(s.maxHistory) {
		s.history = s.history[1:]
	} else if s.maxHistory != 0 {
		delta := sync.Delta{
			Version:  s.version + 1,
			OldValue: value,
			NewValue: newValue,
		}

		s.history = append(s.history, delta)
	}

	s.value = newValue
	s.version++
	s.dirty = true

	if s.onChange != nil {
		go s.onChange(value, newValue)
	}

	return nil
}

func (s *SyncVar) IsDirty() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dirty
}

func (s *SyncVar) MarkClean() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dirty = false
}

func (s *SyncVar) GetDelta(sinceVersion uint64) ([]sync.Delta, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.history == nil {
		return nil, errors.New("history is empty")
	}

	if sinceVersion >= s.version {
		return nil, nil
	}

	var deltas []sync.Delta
	for _, d := range s.history {
		if d.Version > sinceVersion {
			deltas = append(deltas, d)
		}
	}

	return deltas, nil
}

func (s *SyncVar) ApplyDelta(deltas ...sync.Delta) error {
	if deltas == nil {
		return errors.New("no deltas provided")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	sort.Slice(deltas, func(i, j int) bool {
		return deltas[i].Version < deltas[j].Version
	})

	if s.maxHistory > 0 {
		newHistory := make([]sync.Delta, 0, len(s.history)+len(deltas))

		for _, delta := range deltas {
			newHistory = append(newHistory, delta)
		}

		newHistory = append(newHistory, s.history...)

		if len(newHistory) >= int(s.maxHistory) {
			s.history = newHistory[1:]
		} else {
			s.history = newHistory
		}
	}

	return nil
}

func (s *SyncVar) SetConflictResolver(resolver sync.ConflictResolver) {
	s.mu.Lock()
	s.conflictResolver = resolver
	s.mu.Unlock()
}

func (s *SyncVar) GetVersion() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.version
}

func (s *SyncVar) OnChange(eventHandler func(oldValue any, newValue any)) {
	s.mu.Lock()
	s.onChange = eventHandler
	s.mu.Unlock()
}

func (s *SyncVar) OnConflict(eventHandler func(local any, remote any) any) {
	s.mu.Lock()
	s.onConflict = eventHandler
	s.mu.Unlock()
}

func (s *SyncVar) GetPermissions() sync.PermissionMask {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.permissionMask
}

func (s *SyncVar) SetPermissions(mask sync.PermissionMask) {
	s.mu.Lock()
	s.permissionMask = mask
	s.mu.Unlock()
}

func (s *SyncVar) GetHistory() []sync.HistoryEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return []sync.HistoryEntry{
		{
			Version:   s.version,
			Timestamp: time.Now().UnixNano(),
			Value:     s.value,
			ClientID:  "unknown",
		},
	}
}

var _ sync.TypedVariable[any] = (*TypedSyncVar[any])(nil)

// TypedSyncVar is a generic implementation of sync.TypedVariable[T]
// This implementation provides basic synchronization and versioning for any type T.
// It also provides methods for setting and getting the value, checking if the value is dirty,
type TypedSyncVar[T any] struct {
	mu           sc.Mutex
	variableType reflect.Type
	variable     sync.Variable
}

func NewBaseSyncVar[T any](initialValue T, maxHistory uint8) *TypedSyncVar[T] {
	gob.Register(initialValue)

	return &TypedSyncVar[T]{
		mu:           sc.Mutex{},
		variableType: reflect.TypeOf(initialValue),
		variable:     NewSyncVar(initialValue, maxHistory),
	}
}

func (v *TypedSyncVar[T]) SetRoot(variable sync.Variable) {
	v.mu.Lock()
	v.variable = variable
	v.mu.Unlock()
}

func (v *TypedSyncVar[T]) GetRoot() sync.Variable {
	v.mu.Lock()
	defer v.mu.Unlock()
	var c sync.Variable

	reflect.Copy(reflect.ValueOf(c), reflect.ValueOf(v.variable))
	return c
}

func (v *TypedSyncVar[T]) Get() (T, error) {
	res, err := v.variable.Get()
	if err != nil {
		return res, err
	}

	if val, ok := res.(T); ok {
		return val, nil
	} else {
		return val, errors.New("type mismatch")
	}
}

func (v *TypedSyncVar[T]) Set(newValue T) error {
	return v.variable.Set(newValue)
}

func (v *TypedSyncVar[T]) IsDirty() bool {
	return v.variable.IsDirty()
}

func (v *TypedSyncVar[T]) MarkClean() {
	v.variable.MarkClean()
}

func (v *TypedSyncVar[T]) GetDelta(sinceVersion uint64) ([]sync.Delta, error) {
	return v.variable.GetDelta(sinceVersion)
}

func (v *TypedSyncVar[T]) ApplyDelta(delta ...sync.Delta) error {
	return v.variable.ApplyDelta(delta...)
}

func (v *TypedSyncVar[T]) SetConflictResolver(resolver sync.ConflictResolver) {
	v.variable.SetConflictResolver(resolver)
}

func (v *TypedSyncVar[T]) GetVersion() uint64 {
	return v.variable.GetVersion()
}

func (v *TypedSyncVar[T]) OnChange(eventHandler func(oldValue T, newValue T)) {
	v.variable.OnChange(func(oldValue, newValue any) {
		eventHandler(oldValue, newValue)
	})
}

func (v *TypedSyncVar[T]) OnConflict(eventHandler func(local T, remote T) T) {
	v.variable.OnConflict(func(local, remote any) any {
		return eventHandler(local, remote)
	})
}

func (v *TypedSyncVar[T]) GetType() reflect.Type {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.variableType
}

func (v *TypedSyncVar[T]) SetType(r reflect.Type) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.variableType = r
}

func (v *TypedSyncVar[T]) GetPermissionMask() sync.PermissionMask {
	return v.GetPermissionMask()
}

func (v *TypedSyncVar[T]) SetPermissionMask(mask sync.PermissionMask) {
	v.SetPermissionMask(mask)
}

func (v *TypedSyncVar[T]) GetHistory() []sync.HistoryEntry {
	return v.GetHistory()
}

func checkPermissions(action sync.PermissionMask, mask sync.PermissionMask) bool {
	return action >= mask
}
