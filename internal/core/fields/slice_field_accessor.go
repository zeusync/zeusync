package fields

import (
	"context"
	"github.com/zeusync/zeusync/pkg/sequence"
	"time"
)

var _ SliceFieldAccessor[any, []any] = (*SFA[any])(nil)

type SFA[T any] struct {
	root *FA[[]T]
}

func NewSFA[T any](initial ...[]T) *SFA[T] {
	return &SFA[T]{
		root: NewFA[[]T](initial...),
	}
}

func (a *SFA[T]) Get() []T {
	return a.root.Get()
}

func (a *SFA[T]) Set(newSlice []T) {
	a.root.Set(newSlice)
}

func (a *SFA[T]) Lock() {
	a.root.Lock()
}

func (a *SFA[T]) Unlock() {
	a.root.Unlock()
}

func (a *SFA[T]) RLock() {
	a.root.RLock()
}

func (a *SFA[T]) RUnlock() {
	a.root.RUnlock()
}

func (a *SFA[T]) TryLock(timeout time.Duration) bool {
	return a.root.TryLock(timeout)
}

func (a *SFA[T]) Subscribe(onUpdate func(newSlice []T)) (unsubscribe func()) {
	return a.root.Subscribe(onUpdate)
}

func (a *SFA[T]) SubscribeIf(onUpdate func(newSlice []T), filter func([]T) bool) (unsubscribe func()) {
	return a.root.SubscribeIf(onUpdate, filter)
}

func (a *SFA[T]) SubscribeCh() (ch <-chan []T, unsubscribe func()) {
	return a.SubscribeCh()
}

func (a *SFA[T]) SubscribeIfCh(filter func([]T) bool) (ch <-chan []T, unsubscribe func()) {
	return a.root.SubscribeIfCh(filter)
}

func (a *SFA[T]) CompareAndSwap(expected []T, new []T) bool {
	return a.root.CompareAndSwap(expected, new)
}

func (a *SFA[T]) Swap(new []T) []T {
	return a.root.Swap(new)
}

func (a *SFA[T]) Transaction(update func(current []T) []T) (final []T) {
	return a.root.Transaction(update)
}

func (a *SFA[T]) Version() uint64 {
	return a.root.Version()
}

func (a *SFA[T]) ChangedSince(version uint64) bool {
	return a.root.ChangedSince(version)
}

func (a *SFA[T]) Merge(other FieldAccessor[[]T], mergeFn func(current []T, other []T) []T) {
	a.root.Merge(other, mergeFn)
}

func (a *SFA[T]) History(limit uint16) [][]T {
	return a.root.History(limit)
}

func (a *SFA[T]) Delta(fromVersion uint64) ([][]T, error) {
	return a.root.Delta(fromVersion)
}

func (a *SFA[T]) GetWithContext(ctx context.Context) ([]T, error) {
	return a.root.GetWithContext(ctx)
}

func (a *SFA[T]) SetWithContext(ctx context.Context, value []T) error {
	return a.root.SetWithContext(ctx, value)
}

func (a *SFA[T]) Serialize() ([]byte, error) {
	return a.root.Serialize()
}

func (a *SFA[T]) Deserialize(bytes []byte) error {
	return a.root.Deserialize(bytes)
}

func (a *SFA[T]) Clone() FieldAccessor[[]T] {
	return a.root.Clone()
}

func (a *SFA[T]) Index(index int) (T, bool) {
	slice := a.root.Get()
	if len(slice)-1 < index || index < 0 {
		var zero T
		return zero, false
	}
	return slice[index], true
}

func (a *SFA[T]) Assign(index int, value T) bool {
	if len(a.root.Get())-1 < index || index < 0 {
		return false
	}

	a.root.Transaction(func(current []T) []T {
		current[index] = value
		return current
	})

	return true
}

func (a *SFA[T]) Append(values ...T) {
	a.root.Transaction(func(current []T) []T {
		return append(current, values...)
	})
}

func (a *SFA[T]) Remove(index int) bool {
	if len(a.root.Get())-1 < index || index < 0 {
		return false
	}

	a.root.Transaction(func(current []T) []T {
		return append(current[:index], current[index+1:]...)
	})

	return true
}

func (a *SFA[T]) Insert(index int, value T) {
	a.root.Transaction(func(current []T) []T {
		return append(current[:index], append([]T{value}, current[index:]...)...)
	})
}

func (a *SFA[T]) Len() int {
	return len(a.root.Get())
}

func (a *SFA[T]) Clear() {
	a.root.Transaction(func(_ []T) []T {
		return []T{}
	})
}

func (a *SFA[T]) Resize(newLen int) bool {
	if newLen < 0 {
		return false
	}
	a.root.Transaction(func(current []T) []T {
		newSlice := make([]T, newLen)
		for i := 0; i < newLen; i++ {
			newSlice[i] = current[i]
		}
		return newSlice
	})
	return true
}

func (a *SFA[T]) CopyTo(dest []T) int {
	copy(dest, a.root.Get())
	return len(dest)
}

func (a *SFA[T]) Reverse() []T {
	origin := a.root.Get()
	reversed := make([]T, len(origin))
	for i, j := 0, len(origin)-1; i < j; i, j = i+1, j-1 {
		reversed[i], reversed[j] = origin[j], origin[i]
	}
	return reversed
}

func (a *SFA[T]) Iter() *sequence.Iterator[T] {
	slice := a.root.Get()
	return sequence.From(slice)
}
