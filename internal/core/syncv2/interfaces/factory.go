package interfaces

import (
	"fmt"
	"github.com/zeusync/zeusync/internal/core/syncv2"
	"github.com/zeusync/zeusync/internal/core/syncv2/types"
	"github.com/zeusync/zeusync/internal/core/syncv2/vars"
	"github.com/zeusync/zeusync/internal/core/syncv2/wrappers"
	"reflect"
	"unsafe"
)

// RootFactory defines the interface for creating different root implementations
// This allows easy swapping of internal implementations without affecting the upper layer
type RootFactory[T any] interface {
	// CreateAtomicRoot creates a new atomic root implementation
	CreateAtomicRoot(initialValue T) sync.AtomicRoot[T]

	// CreateMutexRoot creates a new mutex root implementation
	CreateMutexRoot(initialValue T) sync.MutexRoot[T]

	// CreateShardedRoot creates a new sharded root implementation
	CreateShardedRoot(initialValue T, shardCount int) sync.ShardedRoot[T]

	// CreateChannelRoot creates a new channel root implementation
	CreateChannelRoot(initialValue T, bufferSize int) sync.ChannelRoot[T]

	// CreateLockFreeRoot creates a new lock-free root implementation
	CreateLockFreeRoot(initialValue T) sync.LockFreeRoot[T]

	// CreatePersistedRoot creates a new persisted root implementation
	CreatePersistedRoot(initialValue T, storagePath string) sync.PersistedRoot[T]

	// CreateReplicatedRoot creates a new replicated root implementation
	CreateReplicatedRoot(initialValue T, nodeID string) sync.ReplicatedRoot[T]

	// CreateWriteOptimizedRoot creates a new write-optimized root implementation
	CreateWriteOptimizedRoot(initialValue T) sync.WriteOptimizedRoot[T]

	// CreateReadOptimizedRoot creates a new read-optimized root implementation
	CreateReadOptimizedRoot(initialValue T) sync.ReadOptimizedRoot[T]

	// CreateMemoryOptimizedRoot creates a new memory-optimized root implementation
	CreateMemoryOptimizedRoot(initialValue T) sync.MemoryOptimizedRoot[T]
}

// AtomicSpecificFactory allows creating type-specific optimized implementations
type AtomicSpecificFactory interface {
	// CreateInt64Atomic creates an optimized atomic implementation for int64
	CreateInt64Atomic(initialValue int64) sync.AtomicRoot[int64]

	// CreateInt32Atomic creates an optimized atomic implementation for int32
	CreateInt32Atomic(initialValue int32) sync.AtomicRoot[int32]

	// CreateUint64Atomic creates an optimized atomic implementation for uint64
	CreateUint64Atomic(initialValue uint64) sync.AtomicRoot[uint64]

	// CreateUint32Atomic creates an optimized atomic implementation for uint32
	CreateUint32Atomic(initialValue uint32) sync.AtomicRoot[uint32]

	// CreateBoolAtomic creates an optimized atomic implementation for bool
	CreateBoolAtomic(initialValue bool) sync.AtomicRoot[bool]

	// CreatePointerAtomic creates an optimized atomic implementation for pointers
	CreatePointerAtomic(initialValue unsafe.Pointer) sync.AtomicRoot[unsafe.Pointer]

	// CreateStringAtomic creates an optimized atomic implementation for strings
	CreateStringAtomic(initialValue string) sync.AtomicRoot[string]

	// CreateFloatAtomic creates an optimized atomic implementation for float64
	CreateFloatAtomic(initialValue float64) sync.AtomicRoot[float64]
}

// DefaultFactory provides a default implementation of RootFactory and AtomicSpecificFactory
type DefaultFactory[T any] struct {
	config types.Configuration
}

// NewDefaultFactory creates a new DefaultFactory with the given configuration
func NewDefaultFactory[T any](config types.Configuration) *DefaultFactory[T] {
	return &DefaultFactory[T]{
		config: config,
	}
}

// CreateAtomicRoot creates an optimized atomic root based on the type T
func (f *DefaultFactory[T]) CreateAtomicRoot(initialValue T) sync.AtomicRoot[T] {
	// Use reflection to determine the best atomic implementation for type T
	valueType := reflect.TypeOf(initialValue)

	switch valueType.Kind() {
	case reflect.Int64:
		if root, ok := any(vars.NewAtomicInt64(any(initialValue).(int64))).(sync.AtomicRoot[T]); ok {
			return root
		}
	case reflect.Int32:
		if root, ok := any(vars.NewAtomicInt32(any(initialValue).(int32))).(sync.AtomicRoot[T]); ok {
			return root
		}
	case reflect.Uint64:
		if root, ok := any(vars.NewAtomicUint64(any(initialValue).(uint64))).(sync.AtomicRoot[T]); ok {
			return root
		}
	case reflect.Bool:
		if root, ok := any(vars.NewAtomicBool(any(initialValue).(bool))).(sync.AtomicRoot[T]); ok {
			return root
		}
	default:
	}

	// Fallback to generic AtomicValue for other types
	return vars.NewAtomicValue[T](initialValue)
}

// CreateMutexRoot creates a mutex-based root implementation
func (f *DefaultFactory[T]) CreateMutexRoot(initialValue T) sync.MutexRoot[T] {
	// TODO: Implement mutex-based root
	panic("MutexRoot not implemented yet")
}

// CreateShardedRoot creates a sharded root implementation
func (f *DefaultFactory[T]) CreateShardedRoot(initialValue T, shardCount int) sync.ShardedRoot[T] {
	// TODO: Implement sharded root
	panic("ShardedRoot not implemented yet")
}

// CreateChannelRoot creates a channel-based root implementation
func (f *DefaultFactory[T]) CreateChannelRoot(initialValue T, bufferSize int) sync.ChannelRoot[T] {
	// TODO: Implement channel-based root
	panic("ChannelRoot not implemented yet")
}

// CreateLockFreeRoot creates a lock-free root implementation
func (f *DefaultFactory[T]) CreateLockFreeRoot(initialValue T) sync.LockFreeRoot[T] {
	// TODO: Implement lock-free root
	panic("LockFreeRoot not implemented yet")
}

// CreatePersistedRoot creates a persisted root implementation
func (f *DefaultFactory[T]) CreatePersistedRoot(initialValue T, storagePath string) sync.PersistedRoot[T] {
	// TODO: Implement persisted root
	panic("PersistedRoot not implemented yet")
}

// CreateReplicatedRoot creates a replicated root implementation
func (f *DefaultFactory[T]) CreateReplicatedRoot(initialValue T, nodeID string) sync.ReplicatedRoot[T] {
	// TODO: Implement replicated root
	panic("ReplicatedRoot not implemented yet")
}

// CreateWriteOptimizedRoot creates a write-optimized root implementation
func (f *DefaultFactory[T]) CreateWriteOptimizedRoot(initialValue T) sync.WriteOptimizedRoot[T] {
	// TODO: Implement write-optimized root
	panic("WriteOptimizedRoot not implemented yet")
}

// CreateReadOptimizedRoot creates a read-optimized root implementation
func (f *DefaultFactory[T]) CreateReadOptimizedRoot(initialValue T) sync.ReadOptimizedRoot[T] {
	// TODO: Implement read-optimized root
	panic("ReadOptimizedRoot not implemented yet")
}

// CreateMemoryOptimizedRoot creates a memory-optimized root implementation
func (f *DefaultFactory[T]) CreateMemoryOptimizedRoot(initialValue T) sync.MemoryOptimizedRoot[T] {
	// TODO: Implement memory-optimized root
	panic("MemoryOptimizedRoot not implemented yet")
}

// AtomicSpecificFactory implementation =>

func (f *DefaultFactory[T]) CreateInt64Atomic(initialValue int64) sync.AtomicRoot[int64] {
	return vars.NewAtomicInt64(initialValue)
}

func (f *DefaultFactory[T]) CreateInt32Atomic(initialValue int32) sync.AtomicRoot[int32] {
	return vars.NewAtomicInt32(initialValue)
}

func (f *DefaultFactory[T]) CreateUint64Atomic(initialValue uint64) sync.AtomicRoot[uint64] {
	return vars.NewAtomicUint64(initialValue)
}

func (f *DefaultFactory[T]) CreateUint32Atomic(initialValue uint32) sync.AtomicRoot[uint32] {
	// TODO: Implement AtomicUint32
	panic("AtomicUint32 not implemented yet")
}

func (f *DefaultFactory[T]) CreateBoolAtomic(initialValue bool) sync.AtomicRoot[bool] {
	return vars.NewAtomicBool(initialValue)
}

func (f *DefaultFactory[T]) CreatePointerAtomic(initialValue unsafe.Pointer) sync.AtomicRoot[unsafe.Pointer] {
	// TODO: Implement AtomicPointer
	panic("AtomicPointer not implemented yet")
}

func (f *DefaultFactory[T]) CreateStringAtomic(initialValue string) sync.AtomicRoot[string] {
	return vars.NewAtomicValue[string](initialValue)
}

func (f *DefaultFactory[T]) CreateFloatAtomic(initialValue float64) sync.AtomicRoot[float64] {
	return vars.NewAtomicValue[float64](initialValue)
}

// VariableFactory creates Variable[T] instances with the appropriate strategy
type VariableFactory[T any] struct {
	rootFactory RootFactory[T]
}

// NewVariableFactory creates a new VariableFactory
func NewVariableFactory[T any](rootFactory RootFactory[T]) *VariableFactory[T] {
	return &VariableFactory[T]{
		rootFactory: rootFactory,
	}
}

// CreateVariable creates a new Variable[T] with the specified strategy
func (f *VariableFactory[T]) CreateVariable(strategy types.StorageStrategy, initialValue T, options ...sync.Option) (sync.Variable[T], error) {
	var core sync.Core[T]

	switch strategy {
	case types.StrategyAtomic:
		atomicRoot := f.rootFactory.CreateAtomicRoot(initialValue)
		core = wrappers.NewAtomicWrapper(atomicRoot)

	case types.StrategyMutex:
		/*mutexRoot := f.rootFactory.CreateMutexRoot(initialValue)
		core = wrappers.NewMutexWrapper(mutexRoot) */ // TODO: Implement MutexWrapper

	case types.StrategySharded:
		/*shardCount := 16 // default
		if len(options) > 0 && options[0].Config.ShardCount > 0 {
			shardCount = options[0].Config.ShardCount
		}
		shardedRoot := f.rootFactory.CreateShardedRoot(initialValue, shardCount)
		core = wrappers.NewShardedWrapper(shardedRoot)*/ // TODO: Implement ShardedWrapper

	default:
		return nil, fmt.Errorf("unsupported storage strategy: %v", strategy)
	}

	return wrappers.NewVariableWrapper(core), nil
}

// GlobalFactory provides a global factory instance for convenience
var GlobalFactory = NewDefaultFactory[any](types.Configuration{
	AtomicType:  types.AtomicValue,
	MutexImpl:   types.StandardMutex,
	ShardedImpl: types.HashSharded,
	ShardCount:  16,
})

// CreateVariable is a convenience function that uses the global factory
func CreateVariable[T any](strategy types.StorageStrategy, initialValue T, options ...sync.Option) (sync.Variable[T], error) {
	factory := NewVariableFactory[T](NewDefaultFactory[T](types.Configuration{}))
	return factory.CreateVariable(strategy, initialValue, options...)
}
