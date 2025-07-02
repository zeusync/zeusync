package fields

import (
	"context"
	"github.com/zeusync/zeusync/pkg/encoding"
	"github.com/zeusync/zeusync/pkg/sequence"
	"go/types"
	"time"
)

// FA is alias for FieldAccessor[T] where T is a specific type.
type FA[T any] FieldAccessor[T]

// FieldAccessor provides basic methods for accessing and modifying a field.
// It also includes locking and event subscription methods, atomic and version control methods, serialization methods.
type FieldAccessor[T any] interface {
	// Basic methods

	Get() T
	Set(T)

	// Locking methods

	Lock()
	Unlock()
	RLock()
	RUnlock()
	TryLock(timeout time.Duration) bool

	// Event methods

	Subscribe(onUpdate func(newValue T)) (unsubscribe func())
	SubscribeIf(onUpdate func(newValue T), filter func(T) bool) (unsubscribe func())
	SubscribeCh() (ch <-chan T, unsubscribe func())
	SubscribeIfCh(filter func(T) bool) (ch <-chan T, unsubscribe func())

	// Atomic methods

	CompareAndSwap(expected T, new T) bool
	Swap(new T) T
	Transaction(update func(current T) T) (final T)

	// Version control methods

	Version() uint64
	ChangedSince(version uint64) bool

	// Merging methods

	Merge(other FieldAccessor[T], mergeFn func(current, other T) T)

	// History methods

	History(limit uint16) []T
	Delta(fromVersion uint64) ([]T, error)

	// Context support methods

	GetWithContext(ctx context.Context) (T, error)
	SetWithContext(ctx context.Context, value T) error

	// Serialization methods

	encoding.Serializable[T]

	Clone() FieldAccessor[T]
}

// NFA is alias for NumberFieldAccessor[T] where T is a specific type.
type NFA[T any] NumberFieldAccessor[T]

// NumberFieldAccessor provides basic methods for accessing and modifying a numeric field.
type NumberFieldAccessor[T any] interface {
	FieldAccessor[T]

	// Mathematical methods

	Add(value T)
	Sub(value T)

	Mul(value T)
	Div(value T)

	Mod(value T)
	Pow(value T)
}

// ArrayFA is alias for ArrayFieldAccessor[T] where T is a specific type.
type ArrayFA[T any] ArrayFieldAccessor[T]

// ArrayFieldAccessor provides basic methods for accessing and modifying an array field.
type ArrayFieldAccessor[T any] interface {
	FieldAccessor[T]

	// Array methods

	Index(index int) T
	Append(values ...T)
	Remove(index int)
	Insert(index int, value T)
	Len() int
	Clear()
	Resize(newLen int)
	CopyTo(dest []T) int
	Reverse() []T
	Iter() sequence.Iterator[T]
}

// ComFA is alias for ComparableFieldAccessor[T] where T is a specific type.
type ComFA[T any] ComparableFieldAccessor[T]

// ComparableFieldAccessor provides basic methods for comparing two field values the same type.
type ComparableFieldAccessor[T any] interface {
	FieldAccessor[T]

	// Comparison methods

	Equal(other ComparableFieldAccessor[T]) bool
	Less(other ComparableFieldAccessor[T]) bool
	Greater(other ComparableFieldAccessor[T]) bool
	LessOrEqual(other ComparableFieldAccessor[T]) bool
	GreaterOrEqual(other ComparableFieldAccessor[T]) bool
}

// CovFA is alias for ConvertibleFieldAccessor[T] where T is a specific type.
type CovFA[T any] ConvertibleFieldAccessor[T]

// ConvertibleFieldAccessor provides basic methods for converting a field value to a different type.
type ConvertibleFieldAccessor[T any] interface {
	FieldAccessor[T]

	// Conversion methods

	ToType(t types.Type) (any, error)
}
