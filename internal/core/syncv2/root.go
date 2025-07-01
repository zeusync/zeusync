package sync

import "unsafe"

// AtomicRoot variables are optimized for simple types that can be handled atomically.
type AtomicRoot[T any] interface {
	Root[T]

	// Get performs an atomic load operation
	Get() T

	// Set performs an atomic store operation
	Set(value T)

	// Swap atomically swaps the value and returns the old value
	Swap(new T) T

	// ApplyDeltas atomically adds delta to the value (for numeric types)
	// Returns the new value after addition
	ApplyDeltas(deltas ...Delta[T]) T
}

// MutexRoot defines the interface for mutex-based variable implementations.
// MutexRoot variables are suitable for complex data structures that require exclusive access.
type MutexRoot[T any] interface {
	Root[T]

	// RLock acquires a read lock
	RLock()

	// RUnlock releases a read lock
	RUnlock()

	// Lock acquires a write lock
	Lock()

	// Unlock releases a write lock
	Unlock()

	// GetWithLock returns the value while holding a read lock
	GetWithLock() T

	// SetWithLock sets the value while holding a write lock
	SetWithLock(value T)
}

// ShardedRoot defines the interface for sharded variable implementations.
// ShardedRoot variables distribute data across multiple shards to reduce contention.
type ShardedRoot[T any] interface {
	Root[T]

	// GetShard returns the value from a specific shard
	GetShard(shardKey string) T

	// SetShard sets the value in a specific shard
	SetShard(shardKey string, value T)

	// GetShardCount returns the number of shards
	GetShardCount() int

	// ForEachShard iterates over all shards
	ForEachShard(action func(shardIndex int, value T) bool)

	// MergeShard merges values from all shards using the provided function
	MergeShard(merge func(values []T) T) T
}

// ChannelRoot defines the interface for channel-based variable implementations.
// Channel variables use Go channels for synchronization.
type ChannelRoot[T any] interface {
	Root[T]

	// Send sends a value through the channel (non-blocking)
	Send(value T) bool

	// Receive receives a value from the channel (non-blocking)
	Receive() (T, bool)

	// SendBlocking sends a value through the channel (blocking)
	SendBlocking(value T)

	// ReceiveBlocking receives a value from the channel (blocking)
	ReceiveBlocking() T

	// BufferSize returns the current buffer size
	BufferSize() int

	// SetBufferSize sets the buffer size (if supported)
	SetBufferSize(size int) error

	// Close closes the channel and releases resources
	Close() error
}

// LockFreeRoot defines the interface for lock-free variable implementations.
// Lock-free variables use advanced algorithms to avoid locks entirely.
type LockFreeRoot[T any] interface {
	Root[T]

	// TrySet attempts to set the value without blocking
	TrySet(value T) bool

	// TryGet attempts to get the value without blocking
	TryGet() (T, bool)

	// CompareAndSwapWeak performs a weak compare-and-swap operation
	CompareAndSwapWeak(old, new T) bool

	// LoadPointer loads a pointer value atomically (for pointer types)
	LoadPointer() unsafe.Pointer

	// StorePointer stores a pointer value atomically (for pointer types)
	StorePointer(ptr unsafe.Pointer)
}

// PersistedRoot defines the interface for persistent variable implementations.
// Persisted variables automatically save their state to disk.
type PersistedRoot[T any] interface {
	Root[T]

	// Flush forces write to persistent storage
	Flush() error

	// Load loads the value from persistent storage
	Load() error

	// SetInterval sets how often to persist (in operations or time)
	SetInterval(interval int) error

	// GetLastPersisted returns the timestamp of the last persistence operation
	GetLastPersisted() int64
}

// ReplicatedRoot defines the interface for replicated variable implementations.
// Replicated variables maintain copies across multiple nodes.
type ReplicatedRoot[T any] interface {
	Root[T]

	// AddReplica adds a new replica node
	AddReplica(nodeID string) error

	// RemoveReplica removes a replica node
	RemoveReplica(nodeID string) error

	// GetReplicaCount returns the number of active replicas
	GetReplicaCount() int

	// SyncReplicas forces synchronization with all replicas
	SyncReplicas() error

	// SetConsistencyLevel sets the required consistency level
	SetConsistencyLevel(level ConsistencyLevel) error
}

// WriteOptimizedRoot defines the interface for write-optimized variable implementations.
// Write-optimized variables are tuned for high write throughput.
type WriteOptimizedRoot[T any] interface {
	Root[T]

	// BatchSet sets multiple values in a single operation
	BatchSet(values ...T) error

	// AsyncSet sets the value asynchronously
	AsyncSet(value T) <-chan error

	// SetWriteBuffer sets the size of the write buffer
	SetWriteBuffer(size int) error

	// FlushWrites forces all pending writes to complete
	FlushWrites() error

	// GetWriteQueueSize returns the current write queue size
	GetWriteQueueSize() int
}

// ReadOptimizedRoot defines the interface for read-optimized variable implementations.
// Read-optimized variables are tuned for high read throughput.
type ReadOptimizedRoot[T any] interface {
	Root[T]

	// BatchGet gets multiple cached values
	BatchGet(count int) []T

	// GetCached returns a cached value (maybe slightly stale)
	GetCached() T

	// SetCacheSize sets the size of the read cache
	SetCacheSize(size int) error

	// InvalidateCache invalidates the read cache
	InvalidateCache()

	// GetCacheHitRate returns the cache hit rate
	GetCacheHitRate() float32

	// PreloadCache preloads the cache with values
	PreloadCache() error
}

// MemoryOptimizedRoot defines the interface for memory-optimized variable implementations.
// Memory-optimized variables minimize memory usage.
type MemoryOptimizedRoot[T any] interface {
	Root[T]

	// Compress compresses the stored value
	Compress() error

	// Decompress decompresses the stored value
	Decompress() error

	// SetCompressionLevel sets the compression level (0-9)
	SetCompressionLevel(level uint8) error

	// GetMemoryUsage returns current memory usage in bytes
	GetMemoryUsage() int64

	// Compact performs memory compaction
	Compact() error

	// SetMemoryLimit sets the maximum memory usage
	SetMemoryLimit(limit int64) error
}

// ConsistencyLevel defines the consistency requirements for replicated variables
type ConsistencyLevel uint8

const (
	ConsistencyEventual ConsistencyLevel = iota
	ConsistencyStrong
	ConsistencyLinearizable
)

// ReplicaStatus represents the status of a replica node
type ReplicaStatus struct {
	NodeID    string
	IsHealthy bool
	LastSync  int64
	Lag       int64
	Version   uint64
}
