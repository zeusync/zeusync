package types

// StorageStrategy defines the different storage strategies for a synchronized variable.
type StorageStrategy uint8

const (
	// StrategyAtomic uses atomic operations for synchronization.
	StrategyAtomic StorageStrategy = iota
	// StrategyMutex uses a read-write mutex for synchronization.
	StrategyMutex
	// StrategyChannels uses channels for synchronization.
	StrategyChannels
	// StrategySharded uses sharding for high concurrency.
	StrategySharded
	// StrategyLockFree uses lock-free data structures.
	StrategyLockFree
	// StrategyPersisted persists the variable to disk.
	StrategyPersisted
	// StrategyReplicated replicates the variable across multiple nodes.
	StrategyReplicated
	// StrategyWriteOptimized is optimized for write-heavy workloads.
	StrategyWriteOptimized
	// StrategyReadOptimized is optimized for read-heavy workloads.
	StrategyReadOptimized
	// StrategyMemoryOptimized is optimized for low memory usage.
	StrategyMemoryOptimized
)

// AccessPattern defines the different access patterns for a variable.
type AccessPattern uint8

const (
	// PatternRandom indicates a random access pattern.
	PatternRandom AccessPattern = iota
	// PatternBurst indicates a bursty access pattern.
	PatternBurst
	// PatternSteady indicates a steady access pattern.
	PatternSteady
	// PatternReadHeavy indicates a read-heavy access pattern.
	PatternReadHeavy
	// PatternWriteHeavy indicates a write-heavy access pattern.
	PatternWriteHeavy
	// PatternMixed indicates a mixed read/write access pattern.
	PatternMixed
	// PatternRareAccess indicates a rare access pattern.
	PatternRareAccess
)

// AtomicType defines different implementations for atomic roots
type AtomicType uint8

const (
	// AtomicValue uses sync/atomic.Value for any type
	AtomicValue AtomicType = iota
	// AtomicInt64 uses sync/atomic int64 operations (for int64 types)
	AtomicInt64
	// AtomicInt32 uses sync/atomic int32 operations (for int32 types)
	AtomicInt32
	// AtomicUint64 uses sync/atomic uint64 operations (for uint64 types)
	AtomicUint64
	// AtomicUint32 uses sync/atomic uint32 operations (for uint32 types)
	AtomicUint32
	// AtomicPointer uses sync/atomic pointer operations (for pointer types)
	AtomicPointer
	// AtomicBool uses sync/atomic bool operations (for bool types)
	AtomicBool
	// AtomicString uses sync/atomic string operations (for string types)
	AtomicString
)

// MutexType defines different implementations for mutex roots
type MutexType uint8

const (
	// StandardMutex uses sync.RWMutex
	StandardMutex MutexType = iota
	// SpinlockMutex uses a spinlock implementation
	SpinlockMutex
	// AdaptiveMutex uses an adaptive mutex that switches between spinning and blocking
	AdaptiveMutex
	// PriorityMutex uses a priority-aware mutex
	PriorityMutex
)

// ShardedType defines different implementations for sharded roots
type ShardedType uint8

const (
	// HashSharded uses hash-based sharding
	HashSharded ShardedType = iota
	// ConsistentHashSharded uses consistent hashing
	ConsistentHashSharded
	// RangeSharded uses range-based sharding
	RangeSharded
	// AdaptiveSharded uses adaptive sharding that can resize
	AdaptiveSharded
)

// ChannelType defines different implementations for channel roots
type ChannelType uint8

const (
	// BufferedChannel uses buffered Go channels
	BufferedChannel ChannelType = iota
	// UnbufferedChannel uses unbuffered Go channels
	UnbufferedChannel
	// PriorityChannel uses priority-based channels
	PriorityChannel
	// BroadcastChannel uses broadcast channels
	BroadcastChannel
	// CycleChannel uses cycle-based channels
	CycleChannel
)

// LockFreeType defines different implementations for lock-free roots
type LockFreeType uint8

const (
	// CASLockFree uses compare-and-swap operations
	CASLockFree LockFreeType = iota
	// RCULockFree uses Read-Copy-Update
	RCULockFree
	// HazardPointerLockFree uses hazard pointers
	HazardPointerLockFree
	// EpochBasedLockFree uses epoch-based memory management
	EpochBasedLockFree
)

// PersistedType defines different implementations for persisted roots
type PersistedType uint8

const (
	// FileSystemPersisted uses file system for persistence
	FileSystemPersisted PersistedType = iota
	// DatabasePersisted uses a database for persistence
	DatabasePersisted
	// MemoryMappedPersisted uses memory-mapped files
	MemoryMappedPersisted
	// CompressedPersisted uses compressed storage
	CompressedPersisted
	// RemotePersisted uses remote client for persistence
	RemotePersisted
)

// ReplicatedType defines different implementations for replicated roots
type ReplicatedType uint8

const (
	// RaftReplicated uses Raft consensus algorithm
	RaftReplicated ReplicatedType = iota
	// GossipReplicated uses gossip protocol
	GossipReplicated
	// MasterSlaveReplicated uses master-slave replication
	MasterSlaveReplicated
	// MultiMasterReplicated uses multi-master replication
	MultiMasterReplicated
)

// WriteOptimizedType defines different implementations for write-optimized roots
type WriteOptimizedType uint8

const (
	// BatchedWrites uses write batching
	BatchedWrites WriteOptimizedType = iota
	// AsyncWrites uses asynchronous writes
	AsyncWrites
	// WriteCoalescing uses write coalescing
	WriteCoalescing
	// WriteBuffering uses write buffering
	WriteBuffering
)

// ReadOptimizedTypes defines different implementations for read-optimized roots
type ReadOptimizedTypes uint8

const (
	// CachedReads uses read caching
	CachedReads ReadOptimizedTypes = iota
	// PreloadedReads uses read preloading
	PreloadedReads
	// CompressedReads uses compressed read cache
	CompressedReads
	// TieredReads uses tiered read cache
	TieredReads
)

// MemoryOptimizedType defines different implementations for memory-optimized roots
type MemoryOptimizedType uint8

const (
	// CompressedMemory uses compression to reduce memory usage
	CompressedMemory MemoryOptimizedType = iota
	// PooledMemory uses memory pooling
	PooledMemory
	// CompactMemory uses compact data structures
	CompactMemory
	// LazyMemory uses lazy loading
	LazyMemory
)

// Configuration holds configuration for creating root implementations
type Configuration struct {
	// AtomicType specifies which atomic implementation to use
	AtomicType AtomicType

	// MutexType specifies which mutex implementation to use
	MutexType MutexType

	// ShardedType specifies which sharded implementation to use
	ShardedType ShardedType

	// ChannelType specifies which channel implementation to use
	ChannelType ChannelType

	// LockFreeType specifies which lock-free implementation to use
	LockFreeType LockFreeType

	// PersistedType specifies which persisted implementation to use
	PersistedType PersistedType

	// ReplicatedType specifies which replicated implementation to use
	ReplicatedType ReplicatedType

	// WriteOptimizedType specifies which write-optimized implementation to use
	WriteOptimizedType WriteOptimizedType

	// ReadOptimizedType specifies which read-optimized implementation to use
	ReadOptimizedType ReadOptimizedTypes

	// MemoryOptimizedType specifies which memory-optimized implementation to use
	MemoryOptimizedType MemoryOptimizedType

	// Additional configuration parameters
	ShardCount  int
	BufferSize  int
	StoragePath string
	NodeID      string
	CacheSize   int
	MemoryLimit int64
}
