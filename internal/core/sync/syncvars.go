package sync

import "reflect"

import "context"

type Variable interface {
	Get(ctx context.Context) (any, error)
	Set(ctx context.Context, value any) error

	IsDirty() bool
	MarkClean()
	GetDelta() ([]byte, error)
	ApplyDelta([]byte) error

	SetConflictResolver(ConflictResolver)
	GetVersion() uint64

	OnChange(func(oldValue, newValue any))
	OnConflict(func(local, remote any)) any

	GetPermissions() Permissions
	SetPermissions(Permissions)

	GetHistory() []HistoryEntry
}

type TypedVariable[T any] interface {
	SetRoot(Variable)
	GetRoot() Variable

	Get(ctx context.Context) (T, error)
	Set(ctx context.Context, value T) error

	IsDirty() bool
	MarkClean()
	GetDelta() ([]byte, error)
	ApplyDelta([]byte) error

	SetConflictResolver(ConflictResolver)
	GetVersion() uint64

	OnChange(func(oldValue, newValue T))
	OnConflict(func(local, remote T)) T

	GetType() reflect.Type
	SetType(reflect.Type)

	GetPermissions() Permissions
	SetPermissions(Permissions)

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

type ConflictMetadata struct{}
