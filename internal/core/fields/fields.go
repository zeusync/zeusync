package fields

import (
	"context"
	"time"

	"github.com/zeusync/zeusync/pkg/encoding"
	"github.com/zeusync/zeusync/pkg/sequence"
)

// Numeric is an interface that represents any numeric type. It is used to define
type Numeric interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~float32 | ~float64
}

// FieldAccessor provides thread-safe access to a field value with comprehensive functionality
// including atomic operations, versioning, event subscriptions, and serialization support.
// It is designed for high-concurrency scenarios where multiple goroutines need to safely
// access and modify shared state.
type FieldAccessor[T any] interface {
	// Basic access methods

	// Get returns the current value stored in the field.
	// This operation is atomic and thread-safe.
	Get() T

	// Set updates the field value and notifies all subscribers.
	// This operation is atomic and thread-safe.
	Set(T)

	// Locking methods for manual synchronization

	// Lock acquires an exclusive lock on the field accessor.
	// Use this when you need to perform multiple operations atomically.
	Lock()

	// Unlock releases the exclusive lock on the field accessor.
	Unlock()

	// RLock acquires a shared (read) lock on the field accessor.
	// Multiple readers can hold the lock simultaneously.
	RLock()

	// RUnlock releases the shared (read) lock on the field accessor.
	RUnlock()

	// TryLock attempts to acquire an exclusive lock within the specified timeout.
	// Returns true if the lock was acquired, false if the timeout was reached.
	TryLock(timeout time.Duration) bool

	// Event subscription methods

	// Subscribe registers a callback function that will be called whenever the value changes.
	// Returns an unsubscribe function that can be called to remove the subscription.
	Subscribe(onUpdate func(newValue T)) (unsubscribe func())

	// SubscribeIf registers a conditional callback function that will be called only when
	// the value changes and the filter function returns true.
	// Returns an unsubscribe function that can be called to remove the subscription.
	SubscribeIf(onUpdate func(newValue T), filter func(T) bool) (unsubscribe func())

	// SubscribeCh returns a channel that receives value updates and an unsubscribe function.
	// The channel is buffered to prevent blocking on slow consumers.
	SubscribeCh() (ch <-chan T, unsubscribe func())

	// SubscribeIfCh returns a channel that receives filtered value updates and an unsubscribe function.
	// Only values that pass the filter function (if provided) are sent to the channel.
	SubscribeIfCh(filter func(T) bool) (ch <-chan T, unsubscribe func())

	// Atomic operation methods

	// CompareAndSwap atomically compares the current value with expected and,
	// if they are equal, swaps the current value with new.
	// Returns true if the swap was performed.
	CompareAndSwap(expected T, new T) bool

	// Swap atomically stores new as the new value and returns the previous value.
	Swap(new T) T

	// Transaction performs an atomic update using the provided update function.
	// The update function receives the current value and should return the new value.
	// The operation is retried until it succeeds or a maximum number of attempts is reached.
	Transaction(update func(current T) T) (final T)

	// Version control methods

	// Version returns the current version number of the field.
	// The version is incremented each time the value changes.
	Version() uint64

	// ChangedSince returns true if the field has been modified since the specified version.
	ChangedSince(version uint64) bool

	// Merging methods

	// Merge combines the current value with another field accessor's value using the provided merge function.
	// The operation is performed atomically using compare-and-swap.
	Merge(other FieldAccessor[T], mergeFn func(current, other T) T)

	// History tracking methods

	// History returns the last 'limit' values stored in the field accessor.
	// The values are returned in chronological order (oldest first).
	History(limit uint16) []T

	// Delta returns all values that have been set since the specified version.
	// Returns an error if the requested version is too old and no longer in history.
	Delta(fromVersion uint64) ([]T, error)

	// Context support methods

	// GetWithContext returns the current value with context support.
	// The context can be used for cancellation or timeout.
	GetWithContext(ctx context.Context) (T, error)

	// SetWithContext sets a new value with context support.
	// The context can be used for cancellation or timeout.
	SetWithContext(ctx context.Context, value T) error

	// Serialization methods

	// Serializable interface provides JSON serialization capabilities
	encoding.Serializable[T]

	// Clone creates a new field accessor with the same current value.
	// The clone does not share subscribers or history with the original.
	Clone() FieldAccessor[T]
}

// NumericFieldAccessor extends FieldAccessor with mathematical operations
// for numeric types. It provides thread-safe arithmetic operations that
// automatically handle value updates and subscriber notifications.
type NumericFieldAccessor[T Numeric] interface {
	FieldAccessor[T]

	// Mathematical operation methods

	// Increment atomically increments the current field value by one.
	Increment()

	// Decrement atomically decrements the current field value by one.
	Decrement()

	// Add atomically adds the given value to the current field value.
	// The operation is thread-safe and notifies subscribers of the change.
	Add(value T)

	// Sub atomically subtracts the given value from the current field value.
	// The operation is thread-safe and notifies subscribers of the change.
	Sub(value T)

	// Mul atomically multiplies the current field value by the given value.
	// The operation is thread-safe and notifies subscribers of the change.
	Mul(value T)

	// Div atomically divides the current field value by the given value.
	// The operation is thread-safe and notifies subscribers of the change.
	// Note: Division by zero behavior depends on the underlying numeric type.
	Div(value T)
}

// SliceFieldAccessor extends FieldAccessor with slice-specific operations.
// It provides thread-safe array manipulation methods that automatically
// handle value updates and subscriber notifications.
type SliceFieldAccessor[T any, S ~[]T] interface {
	FieldAccessor[S]

	// Array manipulation methods

	// Index returns the element at the specified index.
	// Panics if the index is out of bounds.
	Index(index int) (T, bool)

	// Append adds one or more elements to the end of the array.
	// The operation is thread-safe and notifies subscribers of the change.
	Append(values ...T)

	// Remove removes the element at the specified index.
	// The operation is thread-safe and notifies subscribers of the change.
	// Panics if the index is out of bounds.
	Remove(index int) bool

	// Insert inserts a value at the specified index, shifting existing elements.
	// The operation is thread-safe and notifies subscribers of the change.
	// Panics if the index is out of bounds.
	Insert(index int, value T)

	// Len returns the current length of the array.
	// This operation is thread-safe.
	Len() int

	// Clear removes all elements from the array.
	// The operation is thread-safe and notifies subscribers of the change.
	Clear()

	// Resize changes the array size to the specified length.
	// If the new size is larger, zero values are appended.
	// If smaller, elements are truncated.
	// The operation is thread-safe and notifies subscribers of the change.
	Resize(newLen int) bool

	// CopyTo copies array elements to the provided destination slice.
	// Returns the number of elements copied.
	// This operation is thread-safe.
	CopyTo(dest S) int

	// Reverse returns a new array with elements in reverse order.
	// The original array is not modified.
	// This operation is thread-safe.
	Reverse() S

	// Iter returns an iterator for traversing array elements.
	// The iterator provides thread-safe access to array elements.
	Iter() *sequence.Iterator[T]
}

// ComparableFieldAccessor extends FieldAccessor with comparison operations
// for types that support ordering. It provides thread-safe comparison methods
// that can be used for conditional operations and sorting.
type ComparableFieldAccessor[T any] interface {
	FieldAccessor[T]

	// Comparison methods

	// Equal returns true if this field's value equals the other field's value.
	// The comparison is performed atomically and is thread-safe.
	Equal(other ComparableFieldAccessor[T]) bool

	// Less returns true if this field's value is less than the other field's value.
	// The comparison is performed atomically and is thread-safe.
	Less(other ComparableFieldAccessor[T]) bool

	// Greater returns true if this field's value is greater than the other field's value.
	// The comparison is performed atomically and is thread-safe.
	Greater(other ComparableFieldAccessor[T]) bool

	// LessOrEqual returns true if this field's value is less than or equal to the other field's value.
	// The comparison is performed atomically and is thread-safe.
	LessOrEqual(other ComparableFieldAccessor[T]) bool

	// GreaterOrEqual returns true if this field's value is greater than or equal to the other field's value.
	// The comparison is performed atomically and is thread-safe.
	GreaterOrEqual(other ComparableFieldAccessor[T]) bool
}

// CovFA is a type alias for ConvertibleFieldAccessor[T] providing a shorter name
// for convertible field accessor types.
type CovFA[T any] ConvertibleFieldAccessor[T]

// ConvertibleFieldAccessor extends FieldAccessor with type conversion capabilities.
// It provides thread-safe methods for converting field values to different types
// using Go's type system information.
type ConvertibleFieldAccessor[T any] interface {
	FieldAccessor[T]

	// Type conversion methods

	// ToType converts the current field value to the specified Go type.
	// Returns the converted value and an error if the conversion is not possible.
	// The conversion is performed atomically and is thread-safe.
	ToType(t Type) (any, error)
}
