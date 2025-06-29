package sync

import "reflect"

type Variable interface {
	Get() (any, error)
	Set(any) error

	IsDirty() bool
	MarkClean()
	GetDelta(sinceVersion uint64) ([]Delta, error)
	ApplyDelta(...Delta) error

	SetConflictResolver(ConflictResolver)
	GetVersion() uint64

	OnChange(func(oldValue, newValue any))
	OnConflict(func(local, remote any) any)

	GetPermissions() PermissionMask
	SetPermissions(PermissionMask)

	GetType() reflect.Type

	GetHistory() []HistoryEntry
}

type TypedVariable[T any] interface {
	Get() (T, error)
	Set(T) error

	IsDirty() bool
	MarkClean()
	GetDelta(sinceVersion uint64) ([]Delta, error)
	ApplyDelta(...Delta) error

	SetConflictResolver(ConflictResolver)
	GetVersion() uint64

	OnChange(func(oldValue, newValue T))
	OnConflict(func(local, remote T) T)

	GetPermissionMask() PermissionMask
	SetPermissionMask(PermissionMask)

	GetHistory() []HistoryEntry
}

type Permissions struct {
	Read  []string
	Write []string
}

type HistoryEntry struct {
	Version   uint64
	Timestamp int64
	Value     any
	ClientID  string
}

type ConflictResolver interface {
	Resolve(local, remote any, metadata ConflictMetadata) any
}

type TypedConflictResolver[T any] interface {
	Resolve(local, remote T, metadata ConflictMetadata) T
}

type ConflictMetadata map[string]any

type Delta struct {
	Version       uint64
	Path          string
	PreviousValue any
	Value         any
}

type TypedDelta[T any] struct {
	Version  uint64
	Path     string
	OldValue T
	NewValue any
}
