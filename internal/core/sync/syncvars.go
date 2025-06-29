package sync

import "reflect"

type Variable interface {
	Get() any
	Set(any) error

	IsDirty() bool
	MarkClean()
	GetDelta() ([]byte, error)
	ApplyDelta([]byte) error

	SetConflictResolver(ConflictResolver)
	GetVersion() uint64

	OnChange(func(oldValue, newValue any))
	OnConflict(func(local, remote any)) any
}

type TypedVariable[T any] interface {
	SetRoot(Variable)
	GetRoot() Variable

	Set(T)
	Get() T

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
}

type ConflictResolver interface {
	Resolve(local, remote any, metadata ConflictMetadata) any
}

type TypedConflictResolver[T any] interface {
	Resolve(local, remote T, metadata ConflictMetadata) T
}

type ConflictMetadata struct{}
