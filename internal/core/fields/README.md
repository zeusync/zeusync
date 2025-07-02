# Field Accessor Package

The `fields` package provides thread-safe, generic field accessors with comprehensive functionality including atomic operations, versioning, event subscriptions, and serialization support. It's designed for high-concurrency scenarios where multiple goroutines need to safely access and modify shared state.

## Features

- **Thread-Safe Operations**: All operations are atomic and safe for concurrent use
- **Generic Type Support**: Works with any Go type using generics
- **Event Subscriptions**: Subscribe to value changes with callback functions or channels
- **Version Control**: Track changes with automatic versioning
- **History Tracking**: Maintain a history of value changes
- **Atomic Operations**: CompareAndSwap, Swap, and Transaction support
- **Context Support**: Context-aware operations for cancellation and timeouts
- **Serialization**: JSON serialization and deserialization
- **Locking**: Manual locking support for complex operations
- **High Performance**: Optimized for high-throughput scenarios

## Quick Start

```go
package main

import (
    "fmt"
    "github.com/zeusync/zeusync/internal/core/fields"
)

func main() {
    // Create a new field accessor
    fa := fields.NewFA[int](42)
    
    // Get and set values
    fmt.Println("Initial:", fa.Get()) // 42
    fa.Set(100)
    fmt.Println("After set:", fa.Get()) // 100
    
    // Subscribe to changes
    unsub := fa.Subscribe(func(newValue int) {
        fmt.Println("Value changed to:", newValue)
    })
    defer unsub()
    
    fa.Set(200) // Triggers subscription
}
```

## Core Interface

### FieldAccessor[T]

The main interface providing comprehensive field access functionality:

```go
type FieldAccessor[T any] interface {
    // Basic operations
    Get() T
    Set(T)
    
    // Locking
    Lock()
    Unlock()
    RLock()
    RUnlock()
    TryLock(timeout time.Duration) bool
    
    // Subscriptions
    Subscribe(onUpdate func(newValue T)) (unsubscribe func())
    SubscribeIf(onUpdate func(newValue T), filter func(T) bool) (unsubscribe func())
    SubscribeCh() (ch <-chan T, unsubscribe func())
    SubscribeIfCh(filter func(T) bool) (ch <-chan T, unsubscribe func())
    
    // Atomic operations
    CompareAndSwap(expected T, new T) bool
    Swap(new T) T
    Transaction(update func(current T) T) (final T)
    
    // Version control
    Version() uint64
    ChangedSince(version uint64) bool
    
    // Advanced operations
    Merge(other FieldAccessor[T], mergeFn func(current, other T) T)
    History(limit uint16) []T
    Delta(fromVersion uint64) ([]T, error)
    Clone() FieldAccessor[T]
    
    // Context support
    GetWithContext(ctx context.Context) (T, error)
    SetWithContext(ctx context.Context, value T) error
    
    // Serialization
    Serialize() ([]byte, error)
    Deserialize([]byte) error
}
```

## Usage Examples

### Basic Operations

```go
fa := fields.NewFA[string]("hello")

// Get current value
value := fa.Get()

// Set new value
fa.Set("world")

// Check version
version := fa.Version()
```

### Subscriptions

```go
fa := fields.NewFA[int](0)

// Simple subscription
unsub := fa.Subscribe(func(newValue int) {
    fmt.Println("New value:", newValue)
})
defer unsub()

// Conditional subscription
unsub2 := fa.SubscribeIf(func(newValue int) {
    fmt.Println("Large value:", newValue)
}, func(value int) bool {
    return value > 100
})
defer unsub2()

// Channel subscription
ch, unsub3 := fa.SubscribeCh()
defer unsub3()

go func() {
    for value := range ch {
        fmt.Println("Received:", value)
    }
}()
```

### Atomic Operations

```go
fa := fields.NewFA[int](10)

// Compare and swap
if fa.CompareAndSwap(10, 20) {
    fmt.Println("Swap successful")
}

// Atomic swap
oldValue := fa.Swap(30)

// Transaction
result := fa.Transaction(func(current int) int {
    return current * 2
})
```

### Concurrent Access

```go
fa := fields.NewFA[int64](0)

// Safe concurrent increments
var wg sync.WaitGroup
for i := 0; i < 100; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        fa.Transaction(func(current int64) int64 {
            return current + 1
        })
    }()
}
wg.Wait()

fmt.Println("Final value:", fa.Get()) // 100
```

### History and Versioning

```go
fa := fields.NewFA[string]("start")

fa.Set("first")
fa.Set("second")
fa.Set("third")

// Get recent history
history := fa.History(3)
fmt.Println("History:", history) // [first, second, third]

// Get changes since version
delta, err := fa.Delta(1)
if err == nil {
    fmt.Println("Delta:", delta)
}
```

### Context Operations

```go
fa := fields.NewFA[int](42)
ctx := context.Background()

// Context-aware operations
value, err := fa.GetWithContext(ctx)
err = fa.SetWithContext(ctx, 100)

// With timeout
timeoutCtx, cancel := context.WithTimeout(ctx, time.Second)
defer cancel()

value, err = fa.GetWithContext(timeoutCtx)
```

### Serialization

```go
fa := fields.NewFA[map[string]int](map[string]int{
    "apple": 5,
    "banana": 3,
})

// Serialize
data, err := fa.Serialize()

// Deserialize
newFA := fields.NewFA[map[string]int]()
err = newFA.Deserialize(data)
```

## Extended Interfaces

### NumberFieldAccessor[T]

For numeric types with mathematical operations:

```go
type NumberFieldAccessor[T any] interface {
    FieldAccessor[T]
    
    Add(value T)
    Sub(value T)
    Mul(value T)
    Div(value T)
    Mod(value T)
    Pow(value T)
}
```

### ArrayFieldAccessor[T]

For array/slice operations:

```go
type ArrayFieldAccessor[T any] interface {
    FieldAccessor[T]
    
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
```

### ComparableFieldAccessor[T]

For types supporting comparison:

```go
type ComparableFieldAccessor[T any] interface {
    FieldAccessor[T]
    
    Equal(other ComparableFieldAccessor[T]) bool
    Less(other ComparableFieldAccessor[T]) bool
    Greater(other ComparableFieldAccessor[T]) bool
    LessOrEqual(other ComparableFieldAccessor[T]) bool
    GreaterOrEqual(other ComparableFieldAccessor[T]) bool
}
```

## Performance Characteristics

- **Get Operations**: O(1) atomic load
- **Set Operations**: O(1) atomic store + O(n) subscriber notification
- **Subscriptions**: O(1) add/remove, O(n) notification where n = number of subscribers
- **History**: O(1) append, O(k) retrieval where k = requested history size
- **Transactions**: O(1) per attempt, with automatic retry on contention

## Thread Safety

All operations are thread-safe and can be called concurrently from multiple goroutines:

- **Atomic Operations**: Use Go's `sync/atomic` package for lock-free operations
- **Subscriptions**: Protected by RWMutex with optimized notification patterns
- **History**: Separate RWMutex for history operations to minimize contention
- **Notifications**: Asynchronous to prevent blocking setters

## Best Practices

1. **Use Transactions for Complex Updates**: When you need to update a value based on its current state
2. **Unsubscribe Properly**: Always call the unsubscribe function to prevent memory leaks
3. **Consider Buffer Sizes**: Channel subscriptions use buffered channels; adjust size based on your needs
4. **Limit History Size**: Configure appropriate history limits to control memory usage
5. **Use Context Operations**: For operations that might need cancellation or timeouts

## Testing

The package includes comprehensive tests and benchmarks:

```bash
# Run tests
go test ./internal/core/fields -v

# Run benchmarks
go test ./internal/core/fields -bench=. -benchmem

# Run specific benchmark
go test ./internal/core/fields -bench=BenchmarkFA_Get -benchmem
```

## Examples

See `examples_test.go` for comprehensive usage examples covering all major features.

## Performance Benchmarks

The package is optimized for high-performance scenarios:

- **Get**: ~1ns per operation
- **Set**: ~10ns per operation (without subscribers)
- **CompareAndSwap**: ~5ns per operation
- **Transaction**: ~20ns per operation (low contention)
- **Subscribe/Unsubscribe**: ~100ns per operation

Actual performance may vary based on hardware, Go version, and usage patterns.