package vars

import (
	"encoding/gob"
	"errors"
	"reflect"
	"sort"
	sc "sync"
	"sync/atomic"
	"time"

	"github.com/zeusync/zeusync/internal/core/sync"
)

var _ sync.Variable = (*Mutex)(nil)

type Mutex struct {
	valueMu   sc.RWMutex
	historyMu sc.Mutex

	value   any
	version atomic.Uint64
	dirty   bool

	permissionMask sync.PermissionMask
	history        []sync.Delta
	historyIndex   int

	onChange         atomic.Pointer[func(old, new any)]
	onConflict       atomic.Pointer[func(local, remote any) any]
	conflictResolver atomic.Pointer[sync.ConflictResolver]

	maxHistory uint8
}

func NewMutex(initialValue any, maxHistory uint8) *Mutex {
	s := &Mutex{
		value:      initialValue,
		maxHistory: maxHistory,
	}

	s.version.Store(1)

	if maxHistory > 0 {
		s.history = make([]sync.Delta, maxHistory)
	}

	return s
}

func (s *Mutex) Get() (any, error) {
	s.valueMu.RLock()
	defer s.valueMu.RUnlock()
	return s.value, nil
}

func (s *Mutex) Set(newValue any) error {
	if !checkPermissions(sync.PermissionWrite, s.getPermissionMask()) {
		return errors.New("permission denied")
	}

	s.valueMu.Lock()
	defer s.valueMu.Unlock()

	oldValue := s.value
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

	s.value = newValue
	newVersion := s.version.Add(1)
	s.dirty = true

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

func (s *Mutex) updateHistory(delta sync.Delta) {
	s.historyMu.Lock()
	defer s.historyMu.Unlock()

	if len(s.history) == 0 {
		return
	}

	s.history[s.historyIndex] = delta
	s.historyIndex = (s.historyIndex + 1) % len(s.history)
}

func (s *Mutex) getPermissionMask() sync.PermissionMask {
	s.valueMu.RLock()
	defer s.valueMu.RUnlock()
	return s.permissionMask
}

func (s *Mutex) IsDirty() bool {
	s.valueMu.RLock()
	defer s.valueMu.RUnlock()
	return s.dirty
}

func (s *Mutex) MarkClean() {
	s.valueMu.Lock()
	defer s.valueMu.Unlock()
	s.dirty = false
}

func (s *Mutex) GetDelta(sinceVersion uint64) ([]sync.Delta, error) {
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

func (s *Mutex) ApplyDelta(deltas ...sync.Delta) error {
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

	s.valueMu.Lock()
	s.valueMu.Unlock()
	lastDelta := deltas[len(deltas)-1]
	if lastDelta.Version > s.version.Load() {
		s.value = lastDelta.Value
	}

	return nil
}

func (s *Mutex) SetConflictResolver(resolver sync.ConflictResolver) {
	s.conflictResolver.Store(&resolver)
}

func (s *Mutex) GetVersion() uint64 {
	return s.version.Load()
}

func (s *Mutex) OnChange(eventHandler func(oldValue any, newValue any)) {
	s.onChange.Store(&eventHandler)
}

func (s *Mutex) OnConflict(eventHandler func(local any, remote any) any) {
	s.onConflict.Store(&eventHandler)
}

func (s *Mutex) GetPermissions() sync.PermissionMask {
	return s.getPermissionMask()
}

func (s *Mutex) SetPermissions(mask sync.PermissionMask) {
	s.valueMu.Lock()
	s.permissionMask = mask
	s.valueMu.Unlock()
}

func (s *Mutex) GetType() reflect.Type {
	return reflect.TypeOf(s.value)
}

func (s *Mutex) GetHistory() []sync.HistoryEntry {
	s.valueMu.RLock()
	defer s.valueMu.RUnlock()
	return []sync.HistoryEntry{
		{
			Version:   s.version.Load(),
			Timestamp: time.Now().UnixNano(),
			Value:     s.value,
			ClientID:  "unknown",
		},
	}
}

var _ sync.TypedVariable[any] = (*MutexTyped[any])(nil)

// MutexTyped is a generic implementation of sync.TypedVariable[T]
// This implementation provides basic synchronization and versioning for any type T.
// It also provides methods for setting and getting the value, checking if the value is dirty,
type MutexTyped[T any] struct {
	mu   sc.Mutex
	root sync.Variable
}

func NewMutexTyped[T any](initialValue T, maxHistory uint8) *MutexTyped[T] {
	gob.Register(initialValue)

	return &MutexTyped[T]{
		mu:   sc.Mutex{},
		root: NewMutex(initialValue, maxHistory),
	}
}

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

func (v *MutexTyped[T]) getDefaultValue(val any) T {
	return reflect.Zero(reflect.TypeOf(val)).Interface().(T)
}

func (v *MutexTyped[T]) Set(newValue T) error {
	return v.root.Set(newValue)
}

func (v *MutexTyped[T]) IsDirty() bool {
	return v.root.IsDirty()
}

func (v *MutexTyped[T]) MarkClean() {
	v.root.MarkClean()
}

func (v *MutexTyped[T]) GetDelta(sinceVersion uint64) ([]sync.Delta, error) {
	return v.root.GetDelta(sinceVersion)
}

func (v *MutexTyped[T]) ApplyDelta(delta ...sync.Delta) error {
	return v.root.ApplyDelta(delta...)
}

func (v *MutexTyped[T]) SetConflictResolver(resolver sync.ConflictResolver) {
	v.root.SetConflictResolver(resolver)
}

func (v *MutexTyped[T]) GetVersion() uint64 {
	return v.root.GetVersion()
}

func (v *MutexTyped[T]) OnChange(eventHandler func(oldValue T, newValue T)) {
	v.root.OnChange(func(oldValue, newValue any) {
		eventHandler(oldValue.(T), newValue.(T))
	})
}

func (v *MutexTyped[T]) OnConflict(eventHandler func(local T, remote T) T) {
	v.root.OnConflict(func(local, remote any) any {
		return eventHandler(local.(T), remote.(T))
	})
}

func (v *MutexTyped[T]) GetPermissionMask() sync.PermissionMask {
	return v.root.GetPermissions()
}

func (v *MutexTyped[T]) SetPermissionMask(mask sync.PermissionMask) {
	v.root.SetPermissions(mask)
}

func (v *MutexTyped[T]) GetHistory() []sync.HistoryEntry {
	return v.root.GetHistory()
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
