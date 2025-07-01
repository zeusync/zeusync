package vars

import (
	"fmt"
	"github.com/zeusync/zeusync/internal/core/syncv2"
	"github.com/zeusync/zeusync/pkg/concurrent"
	"github.com/zeusync/zeusync/pkg/sequence"
	sc "sync"
	"sync/atomic"
)

// BufferedChannel implements ChannelRoot using buffered Go channels
type BufferedChannel[T any] struct {
	ch      chan T
	current atomic.Value // stores T
	version atomic.Uint64
	dirty   atomic.Bool
	mx      sc.RWMutex
	closed  atomic.Bool
}

var _ sync.ChannelRoot[string] = (*BufferedChannel[string])(nil)

// NewBufferedChannel creates a new BufferedChannel with the given buffer size
func NewBufferedChannel[T any](bufferSize int, initialValue T) *BufferedChannel[T] {
	c := &BufferedChannel[T]{
		ch: make(chan T, bufferSize),
	}
	c.current.Store(initialValue)
	c.version.Store(1)
	c.dirty.Store(false)
	c.closed.Store(false)
	return c
}

// Get returns the current value
func (c *BufferedChannel[T]) Get() T {
	return c.current.Load().(T)
}

// Set updates the current value and sends it to the channel if possible
func (c *BufferedChannel[T]) Set(value T) {
	c.current.Store(value)
	c.version.Add(1)
	c.dirty.Store(true)

	// Try to send to channel (non-blocking)
	select {
	case c.ch <- value:
	default:
		// Channel is full, value is still stored in current
	}
}

// Send sends a value through the channel (non-blocking)
func (c *BufferedChannel[T]) Send(value T) bool {
	if c.closed.Load() {
		return false
	}

	select {
	case c.ch <- value:
		c.current.Store(value)
		c.version.Add(1)
		c.dirty.Store(true)
		return true
	default:
		return false
	}
}

// Receive receives a value from the channel (non-blocking)
func (c *BufferedChannel[T]) Receive() (T, bool) {
	select {
	case value := <-c.ch:
		c.current.Store(value)
		return value, true
	default:
		var zero T
		return zero, false
	}
}

// SendBlocking sends a value through the channel (blocking)
func (c *BufferedChannel[T]) SendBlocking(value T) {
	if c.closed.Load() {
		return
	}

	c.ch <- value
	c.current.Store(value)
	c.version.Add(1)
	c.dirty.Store(true)
}

// ReceiveBlocking receives a value from the channel (blocking)
func (c *BufferedChannel[T]) ReceiveBlocking() T {
	value := <-c.ch
	c.current.Store(value)
	return value
}

// BufferSize returns the current buffer size
func (c *BufferedChannel[T]) BufferSize() int {
	return cap(c.ch)
}

// SetBufferSize sets the buffer size (creates a new channel)
func (c *BufferedChannel[T]) SetBufferSize(size int) error {
	c.mx.Lock()
	defer c.mx.Unlock()

	if c.closed.Load() {
		return fmt.Errorf("channel is closed")
	}

	// Create new channel with new size
	newCh := make(chan T, size)

	// Drain old channel and copy to new one
	close(c.ch)
	for value := range c.ch {
		select {
		case newCh <- value:
		default:
			// New channel is smaller and full, drop remaining values
			break
		}
	}

	c.ch = newCh
	return nil
}

// Version returns the current version number
func (c *BufferedChannel[T]) Version() uint64 {
	return c.version.Load()
}

// IsDirty returns true if the value has been modified since last clean
func (c *BufferedChannel[T]) IsDirty() bool {
	return c.dirty.Load()
}

// MarkClean marks the value as clean
func (c *BufferedChannel[T]) MarkClean() {
	c.dirty.Store(false)
}

// Close closes the channel
func (c *BufferedChannel[T]) Close() error {
	if c.closed.CompareAndSwap(false, true) {
		close(c.ch)
	}
	return nil
}

// UnbufferedChannel implements ChannelRoot using unbuffered Go channels
type UnbufferedChannel[T any] struct {
	ch      chan T
	current atomic.Value // stores T
	version atomic.Uint64
	dirty   atomic.Bool
	mx      sc.RWMutex
	closed  atomic.Bool
}

var _ sync.ChannelRoot[string] = (*UnbufferedChannel[string])(nil)

// NewUnbufferedChannel creates a new UnbufferedChannel
func NewUnbufferedChannel[T any](initialValue T) *UnbufferedChannel[T] {
	c := &UnbufferedChannel[T]{
		ch: make(chan T),
	}
	c.current.Store(initialValue)
	c.version.Store(1)
	c.dirty.Store(false)
	c.closed.Store(false)
	return c
}

// Get returns the current value
func (c *UnbufferedChannel[T]) Get() T {
	return c.current.Load().(T)
}

// Set updates the current value
func (c *UnbufferedChannel[T]) Set(value T) {
	c.current.Store(value)
	c.version.Add(1)
	c.dirty.Store(true)
}

// Send sends a value through the channel (non-blocking)
func (c *UnbufferedChannel[T]) Send(value T) bool {
	if c.closed.Load() {
		return false
	}

	select {
	case c.ch <- value:
		c.current.Store(value)
		c.version.Add(1)
		c.dirty.Store(true)
		return true
	default:
		return false
	}
}

// Receive receives a value from the channel (non-blocking)
func (c *UnbufferedChannel[T]) Receive() (T, bool) {
	select {
	case value := <-c.ch:
		c.current.Store(value)
		return value, true
	default:
		var zero T
		return zero, false
	}
}

// SendBlocking sends a value through the channel (blocking)
func (c *UnbufferedChannel[T]) SendBlocking(value T) {
	if c.closed.Load() {
		return
	}

	c.ch <- value
	c.current.Store(value)
	c.version.Add(1)
	c.dirty.Store(true)
}

// ReceiveBlocking receives a value from the channel (blocking)
func (c *UnbufferedChannel[T]) ReceiveBlocking() T {
	value := <-c.ch
	c.current.Store(value)
	return value
}

// BufferSize returns 0 for unbuffered channels
func (c *UnbufferedChannel[T]) BufferSize() int {
	return 0
}

// SetBufferSize is not supported for unbuffered channels
func (c *UnbufferedChannel[T]) SetBufferSize(size int) error {
	return fmt.Errorf("cannot set buffer size on unbuffered channel")
}

// Version returns the current version number
func (c *UnbufferedChannel[T]) Version() uint64 {
	return c.version.Load()
}

// IsDirty returns true if the value has been modified since last clean
func (c *UnbufferedChannel[T]) IsDirty() bool {
	return c.dirty.Load()
}

// MarkClean marks the value as clean
func (c *UnbufferedChannel[T]) MarkClean() {
	c.dirty.Store(false)
}

// Close closes the channel
func (c *UnbufferedChannel[T]) Close() error {
	if c.closed.CompareAndSwap(false, true) {
		close(c.ch)
	}
	return nil
}

// PriorityChannel implements ChannelRoot using priority-based channels
type PriorityChannel[T any] struct {
	queue   *sequence.PriorityQueue[T]
	current atomic.Value
	version atomic.Uint64
	dirty   atomic.Bool
	mx      sc.RWMutex
	cond    *sc.Cond
	closed  atomic.Bool
	maxSize int
}

var _ sync.ChannelRoot[string] = (*PriorityChannel[string])(nil)

// NewPriorityChannel creates a new PriorityChannel
func NewPriorityChannel[T any](maxSize int, initialValue T) *PriorityChannel[T] {
	c := &PriorityChannel[T]{
		queue:   sequence.NewPriorityQueue[T](),
		maxSize: maxSize,
	}
	c.current.Store(initialValue)
	c.version.Store(1)
	c.dirty.Store(false)
	c.closed.Store(false)
	c.cond = sc.NewCond(&c.mx)
	return c
}

// Get returns the current value
func (c *PriorityChannel[T]) Get() T {
	return c.current.Load().(T)
}

// Set updates the current value
func (c *PriorityChannel[T]) Set(value T) {
	c.current.Store(value)
	c.version.Add(1)
	c.dirty.Store(true)
}

// SendWithPriority sends a value with priority through the channel
func (c *PriorityChannel[T]) SendWithPriority(value T, priority int) bool {
	if c.closed.Load() {
		return false
	}

	c.mx.Lock()
	defer c.mx.Unlock()

	if c.queue.Len() >= c.maxSize {
		return false // Queue is full
	}

	c.queue.Enqueue(value, priority)
	c.current.Store(value)
	c.version.Add(1)
	c.dirty.Store(true)
	c.cond.Signal()

	return true
}

// Send sends a value through the channel with default priority (0)
func (c *PriorityChannel[T]) Send(value T) bool {
	return c.SendWithPriority(value, 0)
}

// Receive receives the highest priority value from the channel (non-blocking)
func (c *PriorityChannel[T]) Receive() (T, bool) {
	c.mx.Lock()
	defer c.mx.Unlock()

	if value, is := c.queue.Dequeue(); is {
		c.current.Store(value)
		return value, true
	}

	var zero = new(T)
	return *zero, false
}

// SendBlocking sends a value through the channel (blocking)
func (c *PriorityChannel[T]) SendBlocking(value T) {
	c.SendBlockingWithPriority(value, 0)
}

// SendBlockingWithPriority sends a value with priority through the channel (blocking)
func (c *PriorityChannel[T]) SendBlockingWithPriority(value T, priority int) {
	if c.closed.Load() {
		return
	}

	c.mx.Lock()
	defer c.mx.Unlock()

	for c.queue.Len() >= c.maxSize && !c.closed.Load() {
		c.cond.Wait()
	}

	if c.closed.Load() {
		return
	}

	c.queue.Enqueue(value, priority)
	c.current.Store(value)
	c.version.Add(1)
	c.dirty.Store(true)
	c.cond.Signal()
}

// ReceiveBlocking receives the highest priority value from the channel (blocking)
func (c *PriorityChannel[T]) ReceiveBlocking() T {
	c.mx.Lock()
	defer c.mx.Unlock()

	for c.queue.IsEmpty() && !c.closed.Load() {
		c.cond.Wait()
	}

	if c.queue.IsEmpty() {
		// Channel was closed
		var zero = new(T)
		return *zero
	}

	defer c.cond.Signal()
	if value, is := c.queue.Dequeue(); is {
		c.current.Store(value)
		return value
	}

	var zero = new(T)
	return *zero
}

// BufferSize returns the current buffer size
func (c *PriorityChannel[T]) BufferSize() int {
	return c.maxSize
}

// SetBufferSize sets the buffer size
func (c *PriorityChannel[T]) SetBufferSize(size int) error {
	c.mx.Lock()
	defer c.mx.Unlock()

	if c.closed.Load() {
		return fmt.Errorf("channel is closed")
	}

	c.maxSize = size
	c.cond.Broadcast() // Wake up any waiting goroutines
	return nil
}

// Version returns the current version number
func (c *PriorityChannel[T]) Version() uint64 {
	return c.version.Load()
}

// IsDirty returns true if the value has been modified since last clean
func (c *PriorityChannel[T]) IsDirty() bool {
	return c.dirty.Load()
}

// MarkClean marks the value as clean
func (c *PriorityChannel[T]) MarkClean() {
	c.dirty.Store(false)
}

// Close closes the channel
func (c *PriorityChannel[T]) Close() error {
	if c.closed.CompareAndSwap(false, true) {
		c.mx.Lock()
		c.cond.Broadcast()
		c.mx.Unlock()
	}
	return nil
}

var _ sync.ChannelRoot[string] = (*BroadcastChannel[string])(nil)

// BroadcastChannel implements ChannelRoot using broadcast channels
type BroadcastChannel[T any] struct {
	subscribers []chan T
	current     atomic.Value // stores T
	version     atomic.Uint64
	dirty       atomic.Bool
	mx          sc.RWMutex
	closed      atomic.Bool
	bufferSize  int
}

// NewBroadcastChannel creates a new BroadcastChannel
func NewBroadcastChannel[T any](bufferSize int, initialValue T) *BroadcastChannel[T] {
	c := &BroadcastChannel[T]{
		subscribers: make([]chan T, 0),
		bufferSize:  bufferSize,
	}
	c.current.Store(initialValue)
	c.version.Store(1)
	c.dirty.Store(false)
	c.closed.Store(false)
	return c
}

// Subscribe adds a new subscriber channel
func (c *BroadcastChannel[T]) Subscribe() <-chan T {
	c.mx.Lock()
	defer c.mx.Unlock()

	if c.closed.Load() {
		templCh := make(chan T)
		close(templCh)
		return templCh
	}

	ch := make(chan T, c.bufferSize)
	c.subscribers = append(c.subscribers, ch)
	return ch
}

// Unsubscribe removes a subscriber channel
func (c *BroadcastChannel[T]) Unsubscribe(ch <-chan T) {
	c.mx.Lock()
	defer c.mx.Unlock()

	for i, sub := range c.subscribers {
		if sub == ch {
			c.subscribers = append(c.subscribers[:i], c.subscribers[i+1:]...)
			close(sub)
			break
		}
	}
}

// Get returns the current value
func (c *BroadcastChannel[T]) Get() T {
	return c.current.Load().(T)
}

// Set updates the current value
func (c *BroadcastChannel[T]) Set(value T) {
	c.current.Store(value)
	c.version.Add(1)
	c.dirty.Store(true)
}

// Send broadcasts a value to all subscribers (non-blocking)
func (c *BroadcastChannel[T]) Send(value T) bool {
	if c.closed.Load() {
		return false
	}

	c.mx.RLock()
	defer c.mx.RUnlock()

	c.current.Store(value)
	c.version.Add(1)
	c.dirty.Store(true)

	// Broadcast to all subscribers
	if len(c.subscribers) < 1_000 {
		for _, sub := range c.subscribers {
			select {
			case sub <- value:
			default:
			}
		}
	} else {
		concurrent.Batch(sequence.From(c.subscribers), 4, func(batch []chan T) {
			for _, sub := range c.subscribers {
				select {
				case sub <- value:
				default:
				}
			}
		})
	}

	return true
}

// Receive is not supported for broadcast channels
func (c *BroadcastChannel[T]) Receive() (T, bool) {
	var zero = new(T)
	return *zero, false
}

// SendBlocking broadcasts a value to all subscribers (blocking)
func (c *BroadcastChannel[T]) SendBlocking(value T) {
	if c.closed.Load() {
		return
	}

	c.mx.RLock()
	defer c.mx.RUnlock()

	c.current.Store(value)
	c.version.Add(1)
	c.dirty.Store(true)

	// Broadcast to all subscribers (blocking)
	for _, sub := range c.subscribers {
		sub <- value
	}
}

// ReceiveBlocking is not supported for broadcast channels
func (c *BroadcastChannel[T]) ReceiveBlocking() T {
	var zero = new(T)
	return *zero
}

// BufferSize returns the current buffer size
func (c *BroadcastChannel[T]) BufferSize() int {
	return c.bufferSize
}

// SetBufferSize sets the buffer size for new subscribers
func (c *BroadcastChannel[T]) SetBufferSize(size int) error {
	c.mx.Lock()
	defer c.mx.Unlock()

	if c.closed.Load() {
		return fmt.Errorf("channel is closed")
	}

	c.bufferSize = size
	return nil
}

// Version returns the current version number
func (c *BroadcastChannel[T]) Version() uint64 {
	return c.version.Load()
}

// IsDirty returns true if the value has been modified since last clean
func (c *BroadcastChannel[T]) IsDirty() bool {
	return c.dirty.Load()
}

// MarkClean marks the value as clean
func (c *BroadcastChannel[T]) MarkClean() {
	c.dirty.Store(false)
}

// Close closes all subscriber channels
func (c *BroadcastChannel[T]) Close() error {
	if c.closed.CompareAndSwap(false, true) {
		c.mx.Lock()
		for _, sub := range c.subscribers {
			close(sub)
		}
		c.subscribers = nil
		c.mx.Unlock()
	}
	return nil
}
