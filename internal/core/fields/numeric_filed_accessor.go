package fields

import (
	"context"
	"time"
)

var _ NumericFieldAccessor[int] = (*NFA[int])(nil)

// NFA is a numeric generic field accessor implementation that provides thread-safe access
type NFA[T Numeric] struct {
	root *FA[T]
}

// NewNFA creates a new NFA with the given initial value.
func NewNFA[T Numeric](initial ...T) *NFA[T] {
	return &NFA[T]{
		root: NewFA[T](initial...),
	}
}

// Get returns the current value stored in the field. This operation is atomic and thread-safe.
func (a *NFA[T]) Get() T {
	return a.root.Get()
}

// Set updates the field value and notifies all subscribers. This operation is atomic and thread-safe.
func (a *NFA[T]) Set(newValue T) {
	a.root.Set(newValue)
}

// Lock method for manual synchronization
func (a *NFA[T]) Lock() {
	a.root.Lock()
}

// Unlock method for manual synchronization
func (a *NFA[T]) Unlock() {
	a.root.Unlock()
}

// RLock method for manual synchronization
func (a *NFA[T]) RLock() {
	a.root.RLock()
}

// RUnlock method for manual synchronization
func (a *NFA[T]) RUnlock() {
	a.root.RUnlock()
}

// TryLock method for manual synchronization
func (a *NFA[T]) TryLock(timeout time.Duration) bool {
	return a.root.TryLock(timeout)
}

func (a *NFA[T]) Subscribe(onUpdate func(newValue T)) (unsubscribe func()) {
	return a.root.Subscribe(onUpdate)
}

func (a *NFA[T]) SubscribeIf(onUpdate func(newValue T), filter func(T) bool) (unsubscribe func()) {
	return a.root.SubscribeIf(onUpdate, filter)
}

func (a *NFA[T]) SubscribeCh() (ch <-chan T, unsubscribe func()) {
	return a.root.SubscribeCh()
}

func (a *NFA[T]) SubscribeIfCh(filter func(T) bool) (ch <-chan T, unsubscribe func()) {
	return a.root.SubscribeIfCh(filter)
}

func (a *NFA[T]) CompareAndSwap(expected T, new T) bool {
	return a.root.CompareAndSwap(expected, new)
}

func (a *NFA[T]) Swap(new T) T {
	return a.root.Swap(new)
}

func (a *NFA[T]) Transaction(update func(current T) T) (final T) {
	return a.root.Transaction(update)
}

func (a *NFA[T]) Version() uint64 {
	return a.root.Version()
}

func (a *NFA[T]) ChangedSince(version uint64) bool {
	return a.root.ChangedSince(version)
}

func (a *NFA[T]) Merge(other FieldAccessor[T], mergeFn func(current T, other T) T) {
	a.root.Merge(other, mergeFn)
}

func (a *NFA[T]) History(limit uint16) []T {
	return a.root.History(limit)
}

func (a *NFA[T]) Delta(fromVersion uint64) ([]T, error) {
	return a.root.Delta(fromVersion)
}

func (a *NFA[T]) GetWithContext(ctx context.Context) (T, error) {
	return a.root.GetWithContext(ctx)
}

func (a *NFA[T]) SetWithContext(ctx context.Context, value T) error {
	return a.root.SetWithContext(ctx, value)
}

func (a *NFA[T]) Serialize() ([]byte, error) {
	return a.root.Serialize()
}

func (a *NFA[T]) Deserialize(bytes []byte) error {
	return a.root.Deserialize(bytes)
}

func (a *NFA[T]) Clone() FieldAccessor[T] {
	return a.root.Clone()
}

func (a *NFA[T]) Increment() {
	a.root.Transaction(func(current T) T {
		return current + 1
	})
}

func (a *NFA[T]) Decrement() {
	a.root.Transaction(func(current T) T {
		return current - 1
	})
}

func (a *NFA[T]) Add(value T) {
	a.root.Transaction(func(current T) T {
		return current + value
	})
}

func (a *NFA[T]) Sub(value T) {
	a.root.Transaction(func(current T) T {
		return current - value
	})
}

func (a *NFA[T]) Mul(value T) {
	a.root.Transaction(func(current T) T {
		return current * value
	})
}

func (a *NFA[T]) Div(value T) {
	a.root.Transaction(func(current T) T {
		return current / value
	})
}
