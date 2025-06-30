package sync

import "unsafe"

// Factory defines the interface for creating different root implementations
// This allows easy swapping of internal implementations without affecting the upper layer
type Factory[T any] interface {
	// CreateAtomicRoot creates a new atomic root implementation
	CreateAtomicRoot(initialValue T) AtomicRoot[T]

	// CreateMutexRoot creates a new mutex root implementation
	CreateMutexRoot(initialValue T) MutexRoot[T]

	// CreateShardedRoot creates a new sharded root implementation
	CreateShardedRoot(initialValue T, shardCount int) ShardedRoot[T]

	// CreateChannelRoot creates a new channel root implementation
	CreateChannelRoot(initialValue T, bufferSize int) ChannelRoot[T]

	// CreateLockFreeRoot creates a new lock-free root implementation
	CreateLockFreeRoot(initialValue T) LockFreeRoot[T]

	// CreatePersistedRoot creates a new persisted root implementation
	CreatePersistedRoot(initialValue T, storagePath string) PersistedRoot[T]

	// CreateReplicatedRoot creates a new replicated root implementation
	CreateReplicatedRoot(initialValue T, nodeID string) ReplicatedRoot[T]

	// CreateWriteOptimizedRoot creates a new write-optimized root implementation
	CreateWriteOptimizedRoot(initialValue T) WriteOptimizedRoot[T]

	// CreateReadOptimizedRoot creates a new read-optimized root implementation
	CreateReadOptimizedRoot(initialValue T) ReadOptimizedRoot[T]

	// CreateMemoryOptimizedRoot creates a new memory-optimized root implementation
	CreateMemoryOptimizedRoot(initialValue T) MemoryOptimizedRoot[T]
}

// AtomicSpecificFactory allows creating type-specific optimized implementations
type AtomicSpecificFactory interface {
	// CreateInt64Atomic creates an optimized atomic implementation for int64
	CreateInt64Atomic(initialValue int64) AtomicRoot[int64]

	// CreateInt32Atomic creates an optimized atomic implementation for int32
	CreateInt32Atomic(initialValue int32) AtomicRoot[int32]

	// CreateUint64Atomic creates an optimized atomic implementation for uint64
	CreateUint64Atomic(initialValue uint64) AtomicRoot[uint64]

	// CreateUint32Atomic creates an optimized atomic implementation for uint32
	CreateUint32Atomic(initialValue uint32) AtomicRoot[uint32]

	// CreateBoolAtomic creates an optimized atomic implementation for bool
	CreateBoolAtomic(initialValue bool) AtomicRoot[bool]

	// CreatePointerAtomic creates an optimized atomic implementation for pointers
	CreatePointerAtomic(initialValue unsafe.Pointer) AtomicRoot[unsafe.Pointer]

	// CreateStringAtomic creates an optimized atomic implementation for strings
	CreateStringAtomic(initialValue string) AtomicRoot[string]

	// CreateFloatAtomic creates an optimized atomic implementation for float64
	CreateFloatAtomic(initialValue float64) AtomicRoot[float64]
}
