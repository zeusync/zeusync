package sync

import (
	"time"
	"unsafe"
)

type Root[T any] interface {
	Get() T
	Set(T)
	CompareAndSwap(old, new T) bool

	Version() uint64

	IsDirty() bool
	MarkClean()
}

type Networked[T any] interface {
	Delta(since uint64) []Delta[T]
	ApplyDelta(...Delta[T]) error

	SetResolver(ConflictResolver[T])

	Marshal() ([]byte, error)
	Unmarshal([]byte) error
}

type Serializable[T any] interface {
	Serialize() ([]byte, error)
	Deserialize([]byte) error
}

type Observable[T any] interface {
	OnChange(func(old, new T))
	OnConflict(func(local, remote T) T)

	Metrics() Metrics
}

type Permissioned interface {
	CheckRead(clientID string) bool
	CheckWrite(clientID string) bool
	SetPermissions(read, write []string)
}

type Core[T any] interface {
	Root[T]
	Networked[T]
	Observable[T]
	Serializable[T]
	Permissioned

	Close() error
	Size() int
}

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

// Permissions defines the read and write permissions for a variable.
type Permissions struct {
	Read  []string // List of client IDs with read access
	Write []string // List of client IDs with write access
}

// HistoryEntry represents a single entry in the history of a variable.
type HistoryEntry[T any] struct {
	Version   uint64         // Version of the variable
	Timestamp int64          // Timestamp of the change
	Value     T              // Value of the variable at this version
	ClientID  string         // ID of the client that made the change
	Operation string         // Type of operation (e.g., "set", "delete")
	Metadata  map[string]any // Additional metadata
}

// ConflictResolver defines the interface for resolving conflicts between local and remote versions of a variable.
type ConflictResolver[T any] interface {
	// Resolve resolves a conflict between a local and T value.
	Resolve(local, remote T, metadata ConflictMetadata) T
	// Priority returns the priority of the resolver.
	Priority() int
}

// ConflictMetadata contains additional information about a conflict.
type ConflictMetadata struct {
	LocalVersion  uint64
	RemoteVersion uint64
	LocalTime     int64
	RemoteTime    int64
	ClientID      string
}

// Delta represents a change to a variable.
type Delta[T any] struct {
	Version   uint64 // Version of the change
	Value     T      // New value of the field
	Timestamp int64  // Timestamp of the change
	ClientID  string // ID of the client that made the change
	OpType    OpType // Type of operation (e.g., "set", "update", "delete")
}

type OpType uint8

const (
	OpSet OpType = iota
	OpUpdate
	OpDelete
)

// Metrics contains performance and usage metrics for a variable.
type Metrics struct {
	Reads     uint64 // Number of read operations
	Writes    uint64 // Number of write operations
	Conflicts uint64 // Number of conflicts
	Errors    uint64 // Number of errors

	ReadLatency  int64 // Read latency in microseconds
	WriteLatency int64 // Write latency in microseconds

	DeltasSent     uint64 // Number of deltas sent
	DeltasReceived uint64 // Number of deltas received
	SyncLatency    int64  // Sync latency in microseconds

	MemUsage int64 // Memory usage in bytes

	Pattern AccessPattern // Access pattern detected by the analysis
}

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

// AnalysisResult contains the results of a variable analysis.
type AnalysisResult struct {
	Strategy        StorageStrategy          // Recommended storage strategy
	Confidence      float64                  // Confidence in the recommendation
	Reasoning       string                   // Explanation for the recommendation
	Metrics         Metrics                  // Metrics used for the analysis
	Recommendations []OptimizationSuggestion // List of optimization suggestions
	Timestamp       time.Time                // Timestamp of the analysis
}

// OptimizationSuggestion provides a suggestion for optimizing a variable.
type OptimizationSuggestion struct {
	Type        string          // Type of suggestion (e.g., "migrate", "tune")
	Priority    int             // Priority of the suggestion (1-10)
	Description string          // Description of the suggestion
	Impact      string          // Estimated impact of the optimization
	Effort      string          // Estimated effort to implement the optimization
	Strategy    StorageStrategy // Recommended strategy (if applicable)
}

// MigrationPlan outlines the steps for migrating a variable to a new strategy.
type MigrationPlan struct {
	Source        StorageStrategy // Source storage strategy
	Target        StorageStrategy // Target storage strategy
	Steps         []MigrationStep // Steps in the migration plan
	EstimatedTime time.Duration   // Estimated time for the migration
	RiskLevel     string          // Estimated risk level of the migration
}

// MigrationStep represents a single step in a migration plan.
type MigrationStep struct {
	Name        string        // Name of the step
	Description string        // Description of the step
	Duration    time.Duration // Estimated duration of the step
	Reversible  bool          // Whether the step is reversible
}

// MigrationCost represents the estimated cost of a migration.
type MigrationCost struct {
	TimeEstimate   time.Duration // Estimated time for the migration
	MemoryOverhead int64         // Estimated memory overhead
	CPUOverhead    float64       // Estimated CPU overhead
	RiskAssessment string        // Assessment of the migration risk
}

// MigrationEvent represents a migration event.
type MigrationEvent struct {
	From      StorageStrategy // Source storage strategy
	To        StorageStrategy // Target storage strategy
	Reason    string          // Reason for the migration
	Timestamp time.Time       // Timestamp of the migration
	Success   bool            // Whether the migration was successful
	Duration  time.Duration   // Duration of the migration
}

// GlobalMetrics contains metrics for the entire variable manager.
type GlobalMetrics struct {
	TotalVariables       int                     // Total number of variables
	MemoryUsage          int64                   // Total memory usage
	TotalOperations      uint64                  // Total number of operations
	AverageLatency       time.Duration           // Average latency of operations
	StrategyDistribution map[StorageStrategy]int // Distribution of storage strategies
	MigrationCount       uint64                  // Total number of migrations
	OptimizationSavings  float64                 // Estimated savings from optimizations
}

type FastString struct {
	data unsafe.Pointer
	len  int
}
