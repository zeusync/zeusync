package fields

import (
	"context"
	"sync"
	"time"
)

// Batch represents a batch of field accessor operations
type Batch[T any] struct {
	fa         FieldAccessor[T]
	operations []func()
	mu         sync.Mutex
}

// NewBatch creates a new batch for the given field accessor
func NewBatch[T any](fa FieldAccessor[T]) *Batch[T] {
	return &Batch[T]{
		fa:         fa,
		operations: make([]func(), 0),
	}
}

// Set adds a set operation to the batch
func (b *Batch[T]) Set(value T) *Batch[T] {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.operations = append(b.operations, func() {
		b.fa.Set(value)
	})
	return b
}

// Transaction adds a transaction operation to the batch
func (b *Batch[T]) Transaction(update func(current T) T) *Batch[T] {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.operations = append(b.operations, func() {
		b.fa.Transaction(update)
	})
	return b
}

// Execute executes all batched operations atomically
func (b *Batch[T]) Execute() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.fa.Lock()
	defer b.fa.Unlock()

	for _, op := range b.operations {
		op()
	}

	// Clear operations after execution
	b.operations = b.operations[:0]
}

// WaitGroup provides a way to wait for multiple field accessor operations
type WaitGroup struct {
	wg sync.WaitGroup
}

// NewWaitGroup creates a new WaitGroup
func NewWaitGroup() *WaitGroup {
	return &WaitGroup{}
}

// Add increments the WaitGroup counter
func (w *WaitGroup) Add(delta int) {
	w.wg.Add(delta)
}

// Done decrements the WaitGroup counter
func (w *WaitGroup) Done() {
	w.wg.Done()
}

// Wait blocks until the WaitGroup counter is zero
func (w *WaitGroup) Wait() {
	w.wg.Wait()
}

// WaitWithTimeout waits with a timeout
func (w *WaitGroup) WaitWithTimeout(timeout time.Duration) bool {
	done := make(chan struct{})
	go func() {
		defer close(done)
		w.wg.Wait()
	}()

	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}

// Aggregator aggregates values from multiple field accessors
type Aggregator[T any] struct {
	sources     []FieldAccessor[T]
	target      FieldAccessor[T]
	aggregateFn func([]T) T
	subscribers []func()
	mu          sync.RWMutex
}

// NewAggregator creates a new aggregator
func NewAggregator[T any](target FieldAccessor[T], aggregateFn func([]T) T) *Aggregator[T] {
	return &Aggregator[T]{
		sources:     make([]FieldAccessor[T], 0),
		target:      target,
		aggregateFn: aggregateFn,
		subscribers: make([]func(), 0),
	}
}

// AddSource adds a source field accessor to the aggregator
func (a *Aggregator[T]) AddSource(source FieldAccessor[T]) {
	a.mu.Lock()
	a.sources = append(a.sources, source)

	// Subscribe to changes in the source
	unsub := source.Subscribe(func(newValue T) {
		a.updateTarget()
	})
	a.subscribers = append(a.subscribers, unsub)
	a.mu.Unlock()

	// Update target immediately (outside of lock to avoid deadlock)
	a.updateTarget()
}

// updateTarget updates the target with aggregated values
func (a *Aggregator[T]) updateTarget() {
	a.mu.RLock()
	if len(a.sources) == 0 {
		a.mu.RUnlock()
		return
	}

	values := make([]T, len(a.sources))
	for i, source := range a.sources {
		values[i] = source.Get()
	}
	a.mu.RUnlock()

	aggregated := a.aggregateFn(values)
	a.target.Set(aggregated)
}

// Close cleans up the aggregator
func (a *Aggregator[T]) Close() {
	a.mu.Lock()
	defer a.mu.Unlock()

	for _, unsub := range a.subscribers {
		unsub()
	}
	a.subscribers = a.subscribers[:0]
	a.sources = a.sources[:0]
}

// Debouncer debounce field accessor updates
type Debouncer[T any] struct {
	fa     FieldAccessor[T]
	target FieldAccessor[T]
	delay  time.Duration
	timer  *time.Timer
	mu     sync.Mutex
	unsub  func()
}

// NewDebouncer creates a new debouncer
func NewDebouncer[T any](source, target FieldAccessor[T], delay time.Duration) *Debouncer[T] {
	d := &Debouncer[T]{
		fa:     source,
		target: target,
		delay:  delay,
	}

	d.unsub = source.Subscribe(func(newValue T) {
		d.debounce(newValue)
	})

	return d
}

// debounce handles the debouncing logic
func (d *Debouncer[T]) debounce(value T) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.timer != nil {
		d.timer.Stop()
	}

	d.timer = time.AfterFunc(d.delay, func() {
		d.target.Set(value)
	})
}

// Close cleans up the debouncer
func (d *Debouncer[T]) Close() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}

	if d.unsub != nil {
		d.unsub()
		d.unsub = nil
	}
}

// Throttler throttles field accessor updates
type Throttler[T any] struct {
	fa         FieldAccessor[T]
	target     FieldAccessor[T]
	interval   time.Duration
	lastUpdate time.Time
	mu         sync.Mutex
	unsub      func()
}

// NewThrottler creates a new throttler
func NewThrottler[T any](source, target FieldAccessor[T], interval time.Duration) *Throttler[T] {
	t := &Throttler[T]{
		fa:       source,
		target:   target,
		interval: interval,
	}

	t.unsub = source.Subscribe(func(newValue T) {
		t.throttle(newValue)
	})

	return t
}

// throttle handles the throttling logic
func (t *Throttler[T]) throttle(value T) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	if now.Sub(t.lastUpdate) >= t.interval {
		t.target.Set(value)
		t.lastUpdate = now
	}
}

// Close cleans up the throttler
func (t *Throttler[T]) Close() {
	if t.unsub != nil {
		t.unsub()
		t.unsub = nil
	}
}

// Validator validates field accessor values
type Validator[T any] struct {
	fa         FieldAccessor[T]
	validateFn func(T) error
	onError    func(error)
	unsub      func()
}

// NewValidator creates a new validator
func NewValidator[T any](fa FieldAccessor[T], validateFn func(T) error, onError func(error)) *Validator[T] {
	v := &Validator[T]{
		fa:         fa,
		validateFn: validateFn,
		onError:    onError,
	}

	v.unsub = fa.Subscribe(func(newValue T) {
		if err := v.validateFn(newValue); err != nil && v.onError != nil {
			v.onError(err)
		}
	})

	return v
}

// Close cleans up the validator
func (v *Validator[T]) Close() {
	if v.unsub != nil {
		v.unsub()
		v.unsub = nil
	}
}

// Transformer transforms values between field accessors
type Transformer[T, U any] struct {
	source      FieldAccessor[T]
	target      FieldAccessor[U]
	transformFn func(T) U
	unsub       func()
}

// NewTransformer creates a new transformer
func NewTransformer[T, U any](source FieldAccessor[T], target FieldAccessor[U], transformFn func(T) U) *Transformer[T, U] {
	t := &Transformer[T, U]{
		source:      source,
		target:      target,
		transformFn: transformFn,
	}

	// Set initial value
	t.target.Set(t.transformFn(source.Get()))

	// Subscribe to changes
	t.unsub = source.Subscribe(func(newValue T) {
		transformed := t.transformFn(newValue)
		t.target.Set(transformed)
	})

	return t
}

// Close cleans up the transformer
func (t *Transformer[T, U]) Close() {
	if t.unsub != nil {
		t.unsub()
		t.unsub = nil
	}
}

// Synchronizer synchronizes multiple field accessors
type Synchronizer[T any] struct {
	accessors []FieldAccessor[T]
	unsubs    []func()
	mu        sync.RWMutex
	syncing   bool
}

// NewSynchronizer creates a new synchronizer
func NewSynchronizer[T any](accessors ...FieldAccessor[T]) *Synchronizer[T] {
	s := &Synchronizer[T]{
		accessors: make([]FieldAccessor[T], len(accessors)),
		unsubs:    make([]func(), len(accessors)),
	}

	copy(s.accessors, accessors)

	// Subscribe to all accessors
	for i, fa := range s.accessors {
		unsub := fa.Subscribe(func(newValue T) {
			s.syncValue(newValue, i)
		})
		s.unsubs[i] = unsub
	}

	return s
}

// syncValue synchronizes a value across all accessors
func (s *Synchronizer[T]) syncValue(value T, sourceIndex int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.syncing {
		return // Prevent infinite loops
	}

	s.syncing = true
	defer func() { s.syncing = false }()

	for i, fa := range s.accessors {
		if i != sourceIndex {
			fa.Set(value)
		}
	}
}

// Close cleans up the synchronizer
func (s *Synchronizer[T]) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, unsub := range s.unsubs {
		if unsub != nil {
			unsub()
		}
	}
	s.unsubs = s.unsubs[:0]
	s.accessors = s.accessors[:0]
}

// WithTimeout wraps field accessor operations with timeout
func WithTimeout[T any](fa FieldAccessor[T], timeout time.Duration) *TimeoutWrapper[T] {
	return &TimeoutWrapper[T]{
		fa:      fa,
		timeout: timeout,
	}
}

// TimeoutWrapper wraps field accessor with timeout functionality
type TimeoutWrapper[T any] struct {
	fa      FieldAccessor[T]
	timeout time.Duration
}

// Get gets value with timeout
func (tw *TimeoutWrapper[T]) Get() (T, error) {
	ctx, cancel := context.WithTimeout(context.Background(), tw.timeout)
	defer cancel()
	return tw.fa.GetWithContext(ctx)
}

// Set sets value with timeout
func (tw *TimeoutWrapper[T]) Set(value T) error {
	ctx, cancel := context.WithTimeout(context.Background(), tw.timeout)
	defer cancel()
	return tw.fa.SetWithContext(ctx, value)
}

// Transaction performs transaction with timeout
func (tw *TimeoutWrapper[T]) Transaction(update func(current T) T) (T, error) {
	ctx, cancel := context.WithTimeout(context.Background(), tw.timeout)
	defer cancel()

	done := make(chan T, 1)
	go func() {
		result := tw.fa.Transaction(update)
		select {
		case done <- result:
		case <-ctx.Done():
		}
	}()

	select {
	case result := <-done:
		return result, nil
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	}
}
