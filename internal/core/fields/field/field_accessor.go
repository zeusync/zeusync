package field

import (
	"context"
	"encoding/json"
	"github.com/zeusync/zeusync/internal/core/fields"
	"sync"
	"sync/atomic"
	"time"
)

var _ fields.FA[any] = (*FA[any])(nil)

type FA[T any] struct {
	value       atomic.Value
	mx          sync.RWMutex
	version     atomic.Uint64
	subscribers map[uint64]func(T)
	subsID      uint64
}

func NewFA[T any](initial ...T) *FA[T] {
	fa := &FA[T]{}

	if initial != nil && len(initial) > 0 {
		fa.value.Store(initial[0])
	}

	return fa
}

func (f *FA[T]) Get() T {
	return f.value.Load().(T)
}

func (f *FA[T]) Set(value T) {
	f.value.Store(value)
	f.version.Add(1)
	notifyAll(f.subscribers, value, f.mx.RLocker())
}

func (f *FA[T]) Lock() {
	f.mx.Lock()
}

func (f *FA[T]) Unlock() {
	f.mx.Unlock()
}

func (f *FA[T]) RLock() {
	f.mx.RLock()
}

func (f *FA[T]) RUnlock() {
	f.mx.RUnlock()
}

func (f *FA[T]) TryLock(timeout time.Duration) bool {
	for {
		select {
		case <-time.After(timeout):
			return false
		default:
			if f.mx.TryLock() {
				return true
			}
		}
	}
}

func (f *FA[T]) Subscribe(onUpdate func(newValue T)) (unsubscribe func()) {
	f.mx.Lock()
	defer f.mx.Unlock()

	id := generateUniqueID(f.subsID)
	f.subscribers[id] = func(val T) { onUpdate(val) }

	return func() {
		f.mx.Lock()
		delete(f.subscribers, id)
		f.mx.Unlock()
	}
}

func (f *FA[T]) SubscribeIf(onUpdate func(newValue T), filter func(T) bool) (unsubscribe func()) {
	f.mx.Lock()
	defer f.mx.Unlock()

	id := generateUniqueID(f.subsID)
	f.subscribers[id] = func(val T) {
		if filter(val) {
			onUpdate(val)
		}
	}

	return func() {
		f.mx.Lock()
		delete(f.subscribers, id)
		f.mx.Unlock()
	}
}

func (f *FA[T]) SubscribeCh() (ch <-chan T, unsubscribe func()) {
	return f.SubscribeIfCh(nil)
}

func (f *FA[T]) SubscribeIfCh(filter func(T) bool) (ch <-chan T, unsubscribe func()) {
	outCh := make(chan T, 100) // Buffered broadcast channel

	cancelCh := make(chan struct{})

	go func() {
		defer close(outCh)

		// First value broadcasting
		if initial := f.Get(); initial != nil && filter == nil || filter(initial) {
			select {
			case outCh <- initial:
			case <-cancelCh:
				return
			}
		}

		// Default subscription
		unsub := f.SubscribeIf(func(val T) {
			select {
			case outCh <- val:
			case <-cancelCh:
			}
		}, filter)

		defer unsub()
		<-cancelCh
	}()

	return outCh, func() { close(cancelCh) }
}

func (f *FA[T]) CompareAndSwap(expected T, new T) bool {
	if f.value.CompareAndSwap(expected, new) {
		f.version.Add(1)
		notifyAll(f.subscribers, new, f.mx.RLocker())
	}
	return false
}

func (f *FA[T]) Swap(new T) T {
	old := f.value.Swap(new)
	f.version.Add(1)
	notifyAll(f.subscribers, new, f.mx.RLocker())
	return old
}

func (f *FA[T]) Transaction(update func(current T) T) (final T) {
	current := f.value.Load()
	updated := update(current)
	if f.CompareAndSwap(current, updated) {
		return updated
	}
	return current
}

func (f *FA[T]) Version() uint64 {
	return f.version.Load()
}

func (f *FA[T]) ChangedSince(version uint64) bool {
	return f.version.Load() == version
}

func (f *FA[T]) Merge(other fields.FieldAccessor[T], mergeFn func(current T, other T) T) {
	current := f.value.Load()
	merged := mergeFn(current, other.Get())
	f.CompareAndSwap(current, merged)
}

func (f *FA[T]) History(_ uint16) []T {
	//TODO implement me
	return []T{f.value.Load()}
}

func (f *FA[T]) Delta(fromVersion uint64) ([]T, error) {
	//TODO implement me
	return []T{f.value.Load()}, nil
}

func (f *FA[T]) GetWithContext(_ context.Context) (T, error) {
	//TODO implement me
	return f.Get(), nil
}

func (f *FA[T]) SetWithContext(_ context.Context, value T) error {
	//TODO implement me
	f.Set(value)
	return nil
}

func (f *FA[T]) Serialize() ([]byte, error) {
	return json.Marshal(f.value.Load())
}

func (f *FA[T]) Deserialize(bytes []byte) error {
	var newValue T
	return json.Unmarshal(bytes, &newValue)
}

func (f *FA[T]) Clone() fields.FieldAccessor[T] {
	return NewFA[T](f.value.Load())
}
