package vars

import (
	"fmt"
	sc "sync"
	"sync/atomic"

	"github.com/zeusync/zeusync/internal/core/syncv2"
	"github.com/zeusync/zeusync/pkg/concurrent"
	"github.com/zeusync/zeusync/pkg/sequence"
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

	// Try to send to ch (non-blocking)
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
		zero := new(T)
		return *zero, false
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
		return fmt.Errorf("ch is closed")
	}

	// Create new ch with new size
	newCh := make(chan T, size)

	// Drain old ch and copy to new one
	close(c.ch)
	for value := range c.ch {
		select {
		case newCh <- value:
		default:
			// New ch is smaller and full, drop remaining values
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

// Close closes the ch
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
// Returns false when the channel is closed or nobody receiving cross it
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
// Returns false when the channel is closed or nobody sending cross it
func (c *UnbufferedChannel[T]) Receive() (T, bool) {
	select {
	case value := <-c.ch:
		c.current.Store(value)
		return value, true
	default:
		zero := new(T)
		return *zero, false
	}
}

// SendBlocking sends a value through the channel (blocking).
// Invoker will be blocked until a value will be received (pipe)
func (c *UnbufferedChannel[T]) SendBlocking(value T) {
	if c.closed.Load() {
		return
	}

	c.ch <- value
	c.current.Store(value)
	c.version.Add(1)
	c.dirty.Store(true)
}

// ReceiveBlocking receives a value from the channel (blocking).
// Invoker will be blocked until a value will be sent (pipe)
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
	return fmt.Errorf("cannot set buffer size on unbuffered ch")
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

// Close closes the ch
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

	zero := new(T)
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
		zero := new(T)
		return *zero
	}

	defer c.cond.Signal()
	if value, is := c.queue.Dequeue(); is {
		c.current.Store(value)
		return value
	}

	zero := new(T)
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
		return fmt.Errorf("ch is closed")
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
	subscribers map[*Subscription[T]]chan T
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
		subscribers: make(map[*Subscription[T]]chan T, 10),
		bufferSize:  bufferSize,
	}
	c.current.Store(initialValue)
	c.version.Store(1)
	c.dirty.Store(false)
	c.closed.Store(false)
	return c
}

// Subscription represents a subscription to a broadcast channel
type Subscription[T any] struct {
	ch    <-chan T
	close func()
}

// Channel returns the channel for the subscription
func (s *Subscription[T]) Channel() <-chan T {
	return s.ch
}

// Close closes the subscription. Save to multi calls
func (s *Subscription[T]) Close() {
	s.close()
}

// Subscribe adds a new subscriber ch
func (c *BroadcastChannel[T]) Subscribe() *Subscription[T] {
	c.mx.Lock()
	defer c.mx.Unlock()

	if c.closed.Load() {
		ch := make(chan T, 1)
		close(ch)
		return &Subscription[T]{ch: ch, close: func() {}}
	}

	ch := make(chan T, c.bufferSize)
	sub := &Subscription[T]{
		ch: ch,
	}
	sub.close = func() { c.Unsubscribe(sub) }
	c.subscribers[sub] = ch
	return sub
}

// Unsubscribe removes a subscriber ch
func (c *BroadcastChannel[T]) Unsubscribe(sub *Subscription[T]) {
	c.mx.Lock()
	defer c.mx.Unlock()

	if ch, ok := c.subscribers[sub]; ok {
		delete(c.subscribers, sub)
		close(ch)
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
		concurrent.Batch(sequence.FromMap(c.subscribers), 4, func(batch []chan T) {
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
	zero := new(T)
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
	zero := new(T)
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
		return fmt.Errorf("ch is closed")
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

var _ sync.ChannelRoot[string] = (*CycleChannel[string])(nil)

// CycleChannel implements a circular buffer channel with non-blocking operations
type CycleChannel[T any] struct {
	buffer   []T           // Circular buffer
	size     uint32        // Fixed size of the buffer
	head     atomic.Uint32 // Position for next write
	tail     atomic.Uint32 // Position for next read
	count    atomic.Int32  // Number of items in buffer
	version  atomic.Uint64 // Version counter for changes
	dirty    atomic.Bool   // Flag indicating if value was modified
	closed   atomic.Bool   // Flag indicating if channel is closed
	current  atomic.Value  // Current value (interface{} type)
	notEmpty *sc.Cond      // Condition variable for waiting receivers
	mx       sc.RWMutex    // Main mutex for synchronization
}

// NewCycleChannel creates a new CycleChannel with given buffer size
// Minimum buffer size is 1 to ensure proper operation
func NewCycleChannel[T any](bufferSize int, initialValue T) *CycleChannel[T] {
	if bufferSize <= 0 {
		bufferSize = 1 // Ensure minimum buffer size
	}

	c := &CycleChannel[T]{
		buffer: make([]T, bufferSize),
		size:   uint32(bufferSize),
	}
	c.current.Store(initialValue)
	c.notEmpty = sc.NewCond(&c.mx)
	return c
}

// Get returns the current value atomically
func (c *CycleChannel[T]) Get() T {
	return c.current.Load().(T) // Type assertion is safe here
}

// Set updates the current value atomically
func (c *CycleChannel[T]) Set(value T) {
	c.current.Store(value)
	c.version.Add(1)
	c.dirty.Store(true)
}

// Send adds a value to the buffer (non-blocking, always succeeds unless closed)
func (c *CycleChannel[T]) Send(value T) bool {
	if c.closed.Load() {
		return false
	}

	c.mx.Lock()
	defer c.mx.Unlock()

	headPos := c.head.Load() % c.size
	c.buffer[headPos] = value // Store the new value

	// Update positions and counters
	c.head.Add(1)
	currentCount := c.count.Add(1)

	// Maintain circular behavior when buffer is full
	if currentCount > int32(c.size) {
		c.tail.Add(1)
		c.count.Store(int32(c.size))
	}

	// Update metadata
	c.current.Store(value)
	c.version.Add(1)
	c.dirty.Store(true)
	c.notEmpty.Signal() // Notify any waiting receivers

	return true
}

// Receive retrieves a value from the buffer (non-blocking)
func (c *CycleChannel[T]) Receive() (T, bool) {
	c.mx.Lock()
	defer c.mx.Unlock()

	if c.count.Load() == 0 {
		var zero T
		return zero, false
	}

	tailPos := c.tail.Load() % c.size
	value := c.buffer[tailPos]

	// Update positions and counters
	c.tail.Add(1)
	c.count.Add(-1)

	// Update current value
	c.current.Store(value)

	return value, true
}

// ReceiveBlocking retrieves a value, blocking if buffer is empty
func (c *CycleChannel[T]) ReceiveBlocking() T {
	c.mx.Lock()
	defer c.mx.Unlock()

	// Wait for data or closure
	for c.count.Load() == 0 && !c.closed.Load() {
		c.notEmpty.Wait()
	}

	if c.count.Load() == 0 {
		var zero T
		return zero
	}

	tailPos := c.tail.Load() % c.size
	value := c.buffer[tailPos]

	// Update positions and counters
	c.tail.Add(1)
	c.count.Add(-1)

	// Update current value
	c.current.Store(value)

	return value
}

// SendBlocking sends a value to the circular buffer (always succeeds immediately)
func (c *CycleChannel[T]) SendBlocking(value T) {
	c.Send(value) // CycleChannel never blocks on send
}

// BufferSize returns the size of the circular buffer
func (c *CycleChannel[T]) BufferSize() int { return int(c.size) }

// Version returns the current version number
func (c *CycleChannel[T]) Version() uint64 {
	return c.version.Load()
}

// IsDirty returns true if the value has been modified since last clean
func (c *CycleChannel[T]) IsDirty() bool {
	return c.dirty.Load()
}

// MarkClean marks the value as clean
func (c *CycleChannel[T]) MarkClean() {
	c.dirty.Store(false)
}

// Close safely shuts down the channel
func (c *CycleChannel[T]) Close() error {
	if c.closed.CompareAndSwap(false, true) {
		c.mx.Lock()
		c.notEmpty.Broadcast() // Wake all waiting receivers
		c.mx.Unlock()
	}
	return nil
}

// SetBufferSize resizes the buffer while preserving existing data
func (c *CycleChannel[T]) SetBufferSize(size int) error {
	if size <= 0 {
		return fmt.Errorf("buffer size must be positive")
	}

	c.mx.Lock()
	defer c.mx.Unlock()

	if c.closed.Load() {
		return fmt.Errorf("channel is closed")
	}

	newBuffer := make([]T, size)
	currentCount := c.count.Load()

	// Calculate how many items we can preserve
	preserveCount := currentCount
	if preserveCount > int32(size) {
		preserveCount = int32(size)
		// If shrinking, keep most recent items
		c.tail.Add(uint32(currentCount - preserveCount))
	}

	// Copy existing data
	for i := int32(0); i < preserveCount; i++ {
		pos := (c.tail.Load() + uint32(i)) % c.size
		newBuffer[i] = c.buffer[pos]
	}

	// Update state
	c.buffer = newBuffer
	c.size = uint32(size)
	c.head.Store(uint32(preserveCount))
	c.tail.Store(0)
	c.count.Store(preserveCount)
	c.version.Add(1)
	c.dirty.Store(true)

	return nil
}

// Peek returns the next value without removing it
func (c *CycleChannel[T]) Peek() (T, bool) {
	c.mx.RLock()
	defer c.mx.RUnlock()

	if c.count.Load() == 0 {
		var zero T
		return zero, false
	}

	tailPos := c.tail.Load() % c.size
	return c.buffer[tailPos], true
}

// DrainAll removes and returns all current items
func (c *CycleChannel[T]) DrainAll() []T {
	c.mx.Lock()
	defer c.mx.Unlock()

	count := int(c.count.Load())
	if count == 0 {
		return nil
	}

	result := make([]T, count)
	for i := 0; i < count; i++ {
		pos := (c.tail.Load() + uint32(i)) % c.size
		result[i] = c.buffer[pos]
	}

	// Reset buffer state
	c.head.Store(0)
	c.tail.Store(0)
	c.count.Store(0)
	c.version.Add(1)
	c.dirty.Store(true)

	return result
}
