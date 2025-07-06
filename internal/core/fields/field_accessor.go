package fields

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// maxRetries is the maximum number of retries for transactional operations.
const maxRetries = 10

var _ FieldAccessor[any] = (*FA[any])(nil)

// FA is a generic field accessor implementation that provides thread-safe access
// to a value of type T with support for versioning, subscriptions, and atomic operations.
type FA[T any] struct {
	value       atomic.Value       // Stores the current value atomically
	mx          sync.RWMutex       // Protects general operations (not subscribers)
	version     atomic.Uint64      // Tracks version changes for optimistic concurrency
	subscribers map[uint64]func(T) // Map of subscriber callbacks
	subsID      atomic.Uint64      // Atomic counter for generating unique subscriber IDs
	subsMx      sync.RWMutex       // Separate mutex for subscribers to prevent deadlocks
	history     []historyEntry[T]  // Stores value history for History() method
	historyMx   sync.RWMutex       // Protects history slice
	maxHistory  uint16             // Maximum number of history entries to keep
}

// historyEntry represents a single entry in the value history
type historyEntry[T any] struct {
	value   T
	version uint64
	time    time.Time
}

// NewFA creates a new field accessor with an optional initial value.
// If no initial value is provided, the zero value of type T is used.
func NewFA[T any](initial ...T) *FA[T] {
	fa := &FA[T]{
		value:       atomic.Value{},
		mx:          sync.RWMutex{},
		version:     atomic.Uint64{},
		subscribers: make(map[uint64]func(T)),
		subsID:      atomic.Uint64{},
		history:     make([]historyEntry[T], 0, 100),
		historyMx:   sync.RWMutex{},
		maxHistory:  100,
	}

	var initialValue T
	if len(initial) > 0 {
		initialValue = initial[0]
	}

	fa.value.Store(initialValue)
	fa.version.Store(1)
	fa.addToHistory(initialValue, 0)

	return fa
}

// Get returns the current value stored in the field accessor.
// This operation is atomic and thread-safe.
func (a *FA[T]) Get() T {
	val := a.value.Load()
	if val == nil {
		var zero T
		return zero
	}
	return val.(T)
}

// Set updates the value stored in the field accessor and notifies all subscribers.
// This operation is atomic and thread-safe.
func (a *FA[T]) Set(value T) {
	a.value.Store(value)
	version := a.version.Add(1)
	a.addToHistory(value, version)
	a.notifySubscribers(value)
}

// Lock acquires an exclusive lock on the field accessor.
// Use this when you need to perform multiple operations atomically.
func (a *FA[T]) Lock() {
	a.mx.Lock()
}

// Unlock releases the exclusive lock on the field accessor.
func (a *FA[T]) Unlock() {
	a.mx.Unlock()
}

// RLock acquires a shared (read) lock on the field accessor.
// Multiple readers can hold the lock simultaneously.
func (a *FA[T]) RLock() {
	a.mx.RLock()
}

// RUnlock releases the shared (read) lock on the field accessor.
func (a *FA[T]) RUnlock() {
	a.mx.RUnlock()
}

// TryLock attempts to acquire an exclusive lock within the specified timeout.
// Returns true if the lock was acquired, false if the timeout was reached.
func (a *FA[T]) TryLock(timeout time.Duration) bool {
	done := make(chan bool, 1)

	go func() {
		a.mx.Lock()
		select {
		case done <- true:
		default:
			a.mx.Unlock()
		}
	}()

	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}

// Subscribe registers a callback function that will be called whenever the value changes.
// Returns an unsubscribe function that can be called to remove the subscription.
func (a *FA[T]) Subscribe(onUpdate func(newValue T)) (unsubscribe func()) {
	a.subsMx.Lock()
	defer a.subsMx.Unlock()

	id := generateUniqueID(&a.subsID)
	a.subscribers[id] = onUpdate

	return func() {
		a.subsMx.Lock()
		defer a.subsMx.Unlock()
		delete(a.subscribers, id)
	}
}

// SubscribeIf registers a conditional callback function that will be called only when
// the value changes and the filter function returns true.
// Returns an unsubscribe function that can be called to remove the subscription.
func (a *FA[T]) SubscribeIf(onUpdate func(newValue T), filter func(T) bool) (unsubscribe func()) {
	a.subsMx.Lock()
	defer a.subsMx.Unlock()

	id := generateUniqueID(&a.subsID)
	a.subscribers[id] = func(val T) {
		if filter == nil || filter(val) {
			onUpdate(val)
		}
	}

	return func() {
		a.subsMx.Lock()
		defer a.subsMx.Unlock()
		delete(a.subscribers, id)
	}
}

// SubscribeCh returns a channel that receives value updates and an unsubscribe function.
// The channel is buffered to prevent blocking on slow consumers.
func (a *FA[T]) SubscribeCh() (ch <-chan T, unsubscribe func()) {
	return a.SubscribeIfCh(nil)
}

// SubscribeIfCh returns a channel that receives filtered value updates and an unsubscribe function.
// Only values that pass the filter function (if provided) are sent to the channel.
func (a *FA[T]) SubscribeIfCh(filter func(T) bool) (ch <-chan T, unsubscribe func()) {
	outCh := make(chan T, 10)
	cancelCh := make(chan struct{})

	go func() {
		defer close(outCh)

		// Send current value if it passes the filter
		if current := a.Get(); filter == nil || filter(current) {
			select {
			case outCh <- current:
			case <-cancelCh:
				return
			}
		}

		// Subscribe to future updates
		unsub := a.SubscribeIf(func(val T) {
			select {
			case outCh <- val:
			case <-cancelCh:
			}
		}, filter)

		defer unsub()
		<-cancelCh
	}()

	return outCh, func() {
		close(cancelCh)
	}
}

// CompareAndSwap atomically compares the current value with expected and,
// if they are equal, swaps the current value with new.
// Returns true if the swap was performed.
func (a *FA[T]) CompareAndSwap(expected T, new T) bool {
	if a.value.CompareAndSwap(expected, new) {
		version := a.version.Add(1)
		a.addToHistory(new, version)
		a.notifySubscribers(new)
		return true
	}
	return false
}

// Swap atomically stores new as the new value and returns the previous value.
func (a *FA[T]) Swap(new T) T {
	old := a.value.Swap(new)
	version := a.version.Add(1)
	a.addToHistory(new, version)
	a.notifySubscribers(new)

	if old == nil {
		var zero T
		return zero
	}
	return old.(T)
}

// Transaction performs an atomic update using the provided update function.
// The update function receives the current value and should return the new value.
// The operation is retried until it succeeds or a maximum number of attempts is reached.
// Default retries are set to 25.
func (a *FA[T]) Transaction(update func(current T) T) (final T) {
	for i := 0; i < maxRetries; i++ {
		current := a.Get()
		updated := update(current)
		if a.CompareAndSwap(current, updated) {
			return updated
		}
		// Small backoff to reduce contention
		if i > 5 {
			time.Sleep(time.Microsecond * time.Duration(i))
		}
	}

	// Fallback: return current value if transaction fails
	return a.Get()
}

// Version returns the current version number of the field.
// The version is incremented each time the value changes.
func (a *FA[T]) Version() uint64 {
	return a.version.Load()
}

// ChangedSince returns true if the field has been modified since the specified version.
func (a *FA[T]) ChangedSince(version uint64) bool {
	return a.version.Load() > version
}

// Merge combines the current value with another field accessor's value using the provided merge function.
// The operation is performed atomically using compare-and-swap.
func (a *FA[T]) Merge(other FieldAccessor[T], mergeFn func(current T, other T) T) {
	const maxRetries = 100

	for i := 0; i < maxRetries; i++ {
		current := a.Get()
		otherValue := other.Get()
		merged := mergeFn(current, otherValue)

		if a.CompareAndSwap(current, merged) {
			return
		}

		// Small backoff to reduce contention
		if i > 10 {
			time.Sleep(time.Microsecond * time.Duration(i))
		}
	}
}

// History returns the last 'limit' values stored in the field accessor.
// The values are returned in chronological order (oldest first).
func (a *FA[T]) History(limit uint16) []T {
	a.historyMx.RLock()
	defer a.historyMx.RUnlock()

	if limit == 0 || len(a.history) == 0 {
		return []T{}
	}

	start := 0
	if int(limit) < len(a.history) {
		start = len(a.history) - int(limit)
	}

	result := make([]T, len(a.history)-start)
	for i, entry := range a.history[start:] {
		result[i] = entry.value
	}

	return result
}

// Delta returns all values that have been set since the specified version.
// Returns an error if the requested version is too old and no longer in history.
func (a *FA[T]) Delta(fromVersion uint64) ([]T, error) {
	a.historyMx.RLock()
	defer a.historyMx.RUnlock()

	var result []T
	found := false

	for _, entry := range a.history {
		if entry.version > fromVersion {
			if !found {
				found = true
			}
			result = append(result, entry.value)
		}
	}

	if !found && fromVersion < a.version.Load() {
		return nil, errors.New("version too old, not available in history")
	}

	return result, nil
}

// GetWithContext returns the current value with context support.
// The context can be used for cancellation or timeout.
func (a *FA[T]) GetWithContext(ctx context.Context) (T, error) {
	select {
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	default:
		return a.Get(), nil
	}
}

// SetWithContext sets a new value with context support.
// The context can be used for cancellation or timeout.
func (a *FA[T]) SetWithContext(ctx context.Context, value T) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		a.Set(value)
		return nil
	}
}

// Serialize converts the current value to JSON bytes.
func (a *FA[T]) Serialize() ([]byte, error) {
	return json.Marshal(a.Get())
}

// Deserialize updates the field value from JSON bytes.
func (a *FA[T]) Deserialize(bytes []byte) error {
	var newValue T
	if err := json.Unmarshal(bytes, &newValue); err != nil {
		return err
	}
	a.Set(newValue)
	return nil
}

// Clone creates a new field accessor with the same current value.
// The clone does not share subscribers or history with the original.
func (a *FA[T]) Clone() FieldAccessor[T] {
	return NewFA[T](a.Get())
}

// addToHistory adds a new entry to the value history.
// This method should be called while holding appropriate locks.
func (a *FA[T]) addToHistory(value T, version uint64) {
	a.historyMx.Lock()
	defer a.historyMx.Unlock()

	entry := historyEntry[T]{
		value:   value,
		version: version,
		time:    time.Now(),
	}

	a.history = append(a.history, entry)

	// Trim history if it exceeds maximum size
	if len(a.history) > int(a.maxHistory) {
		copy(a.history, a.history[1:])
		a.history = a.history[:a.maxHistory]
	}
}

// notifySubscribers notifies all registered subscribers about a value change.
// Notifications are sent asynchronously to prevent blocking the setter.
func (a *FA[T]) notifySubscribers(value T) {
	a.subsMx.RLock()
	if len(a.subscribers) == 0 {
		a.subsMx.RUnlock()
		return
	}

	// Create a snapshot of subscribers to avoid holding the lock during notifications
	subscribers := make([]func(T), 0, len(a.subscribers))
	for _, sub := range a.subscribers {
		subscribers = append(subscribers, sub)
	}
	a.subsMx.RUnlock()

	// Notify subscribers asynchronously
	go func() {
		var wg sync.WaitGroup
		wg.Add(len(subscribers))

		for _, sub := range subscribers {
			go func(callback func(T)) {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						// Log panic but don't crash the application
						// Note: In a real application, you'd want to use a proper logger here
					}
				}()
				callback(value)
			}(sub)
		}

		wg.Wait()
	}()
}
