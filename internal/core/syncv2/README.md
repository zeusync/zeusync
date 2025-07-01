# SyncV2 - High-Performance Synchronized Variables

SyncV2 provides a flexible, high-performance system for synchronized variables with automatic optimization and hot-swapping capabilities.

## Key Features

- **Simple Interface**: Clean, easy-to-use API that hides internal complexity
- **Type-Specific Optimizations**: Automatic selection of optimal implementations based on data type
- **Hot-Swapping**: Change synchronization strategies at runtime without downtime
- **Multiple Strategies**: Atomic, Mutex, Sharded, Channel-based, Lock-free, and more
- **Metrics & Analytics**: Built-in performance monitoring and optimization suggestions
- **Network Synchronization**: Built-in support for distributed variables

## Quick Start

```go
import sync "github.com/zeusync/zeusync/internal/core/syncv2"

// Create a simple int64 variable (automatically uses AtomicInt64 for performance)
intVar, err := sync.CreateVariable(sync.StrategyAtomic, int64(42))
if err != nil {
    log.Fatal(err)
}
defer intVar.Close()

// Basic operations
fmt.Printf("Value: %d\n", intVar.Get())
intVar.Set(100)

// Atomic updates
newValue := intVar.Update(func(current int64) int64 {
    return current * 2
})

// Subscribe to changes
unsubscribe := intVar.Subscribe(func(old, new int64) {
    fmt.Printf("Changed from %d to %d\n", old, new)
})
defer unsubscribe()
```

## Architecture

### Two-Level Design

**Upper Level (Simple Interface)**:
- `Variable[T]` - Simple interface with Get(), Set(), Update(), Subscribe()
- `CreateVariable()` - Easy variable creation
- Hides all internal complexity

**Lower Level (Flexible Roots)**:
- `AtomicRoot[T]`, `MutexRoot[T]`, `ShardedRoot[T]` - Specialized interfaces
- Type-specific implementations (AtomicInt64, AtomicBool, etc.)
- Hot-swappable internal implementations

### Type-Specific Optimizations

The system automatically chooses the best implementation based on your data type:

- `int64` → `AtomicInt64` (uses `sync/atomic.Int64`)
- `int32` → `AtomicInt32` (uses `sync/atomic.Int32`)
- `bool` → `AtomicBool` (uses `sync/atomic.Bool`)
- `uint64` → `AtomicUint64` (uses `sync/atomic.Uint64`)
- Other types → `AtomicValue[T]` (uses `sync/atomic.Value`)

## Storage Strategies

- **StrategyAtomic**: Lock-free atomic operations (best for simple types)
- **StrategyMutex**: Read-write mutex (best for complex types)
- **StrategySharded**: Distributed across multiple shards (best for high contention)
- **StrategyChannels**: Channel-based synchronization
- **StrategyLockFree**: Advanced lock-free algorithms
- **StrategyPersisted**: Automatic persistence to disk
- **StrategyReplicated**: Multi-node replication
- **StrategyWriteOptimized**: Optimized for write-heavy workloads
- **StrategyReadOptimized**: Optimized for read-heavy workloads
- **StrategyMemoryOptimized**: Minimized memory usage

## Advanced Usage

### Custom Factory Configuration

```go
config := sync.Configuration{
    AtomicType:  sync.AtomicInt64,
    ShardCount:  32,
    BufferSize:  1024,
    CacheSize:   512,
    MemoryLimit: 1024 * 1024,
}

factory := sync.NewDefaultFactory[int64](config)
variableFactory := sync.NewVariableFactory(factory)

intVar, err := variableFactory.CreateVariable(sync.StrategyAtomic, int64(42))
```

### Type-Specific Creation

```go
factory := sync.NewDefaultFactory[int64](sync.Configuration{})

// Create optimized type-specific variables
int64Root := factory.CreateInt64Atomic(42)
boolRoot := factory.CreateBoolAtomic(true)
stringRoot := factory.CreateStringAtomic("hello")
```

### Complex Data Types

```go
type GameState struct {
    Score  int
    Level  int
    Health int
}

gameVar, err := sync.CreateVariable(sync.StrategyAtomic, GameState{
    Score:  0,
    Level:  1,
    Health: 100,
})

// Atomic updates
gameVar.Update(func(current GameState) GameState {
    current.Score += 100
    current.Level++
    return current
})
```

## Performance Benefits

1. **Type-Specific Optimizations**: `int64` variables use `atomic.Int64` instead of `atomic.Value`
2. **Zero-Copy Operations**: Direct atomic operations without boxing/unboxing
3. **Hot-Swapping**: Change strategies without recreating variables
4. **Automatic Optimization**: Built-in analytics suggest optimal strategies
5. **Minimal Overhead**: Simple interface with maximum performance

## Migration & Hot-Swapping

The system supports automatic migration between strategies based on usage patterns:

```go
// Variable starts as Atomic
intVar, _ := sync.CreateVariable(sync.StrategyAtomic, int64(42))

// System can automatically migrate to Sharded if high contention is detected
// Or manually migrate:
// migrator.Migrate(intVar, sync.StrategySharded)
```

## Thread Safety

All operations are thread-safe. The system is designed for high-concurrency environments like game servers with thousands of concurrent players.

## Future Extensions

- Additional storage strategies
- Custom conflict resolution algorithms
- Advanced analytics and ML-based optimization
- Integration with distributed systems
- Custom serialization protocols