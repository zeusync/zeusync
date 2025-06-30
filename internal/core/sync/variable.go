package sync

import (
	"reflect"
	"time"
)

// Variable defines the interface for a synchronized variable.
// It provides methods for getting and setting the value, managing state,
// handling conflicts, and accessing metadata.
type Variable interface {
	// Get returns the current value of the variable.
	Get() (any, error)
	// Set updates the value of the variable.
	Set(any) error

	// IsDirty returns true if the variable has been modified since the last sync.
	IsDirty() bool
	// MarkClean marks the variable as clean (not modified).
	MarkClean()
	// GetVersion returns the current version of the variable.
	GetVersion() uint64

	// GetDelta returns a list of changes since a specific version.
	GetDelta(sinceVersion uint64) ([]Delta, error)
	// ApplyDelta applies a list of changes to the variable.
	ApplyDelta(...Delta) error

	// SetConflictResolver sets the conflict resolver for the variable.
	SetConflictResolver(ConflictResolver)
	// OnChange registers a callback function to be called when the variable's value changes.
	OnChange(func(oldValue, newValue any))
	// OnConflict registers a callback function to be called when a conflict occurs.
	OnConflict(func(local, remote any) any)

	// GetPermissions returns the access permissions for the variable.
	GetPermissions() PermissionMask
	// SetPermissions sets the access permissions for the variable.
	SetPermissions(PermissionMask)

	// GetMetrics returns the performance metrics for the variable.
	GetMetrics() VariableMetrics
	// GetStorageStrategy returns the current storage strategy of the variable.
	GetStorageStrategy() StorageStrategy
	// CanMigrateTo checks if the variable can be migrated to a new storage strategy.
	CanMigrateTo(StorageStrategy) bool

	// GetType returns the type of the variable's value.
	GetType() reflect.Type
	// GetHistory returns the history of changes for the variable.
	GetHistory() []HistoryEntry

	// Close releases any resources used by the variable.
	Close() error
	// Size returns the memory footprint of the variable in bytes.
	Size() int64
}

// TypedVariable defines the interface for a type-safe synchronized variable.
type TypedVariable[T any] interface {
	// Get returns the current value of the variable.
	Get() (T, error)
	// Set updates the value of the variable.
	Set(T) error

	// IsDirty returns true if the variable has been modified since the last sync.
	IsDirty() bool
	// MarkClean marks the variable as clean (not modified).
	MarkClean()
	// GetVersion returns the current version of the variable.
	GetVersion() uint64

	// GetDelta returns a list of changes since a specific version.
	GetDelta(sinceVersion uint64) ([]Delta, error)
	// ApplyDelta applies a list of changes to the variable.
	ApplyDelta(...Delta) error

	// SetConflictResolver sets the conflict resolver for the variable.
	SetConflictResolver(resolver ConflictResolver)
	// OnChange registers a callback function to be called when the variable's value changes.
	OnChange(func(oldValue, newValue T))
	// OnConflict registers a callback function to be called when a conflict occurs.
	OnConflict(func(local, remote T) T)

	// GetPermissions returns the access permissions for the variable.
	GetPermissions() PermissionMask
	// SetPermissions sets the access permissions for the variable.
	SetPermissions(PermissionMask)

	// GetHistory returns the history of changes for the variable.
	GetHistory() []HistoryEntry

	// GetMetrics returns the performance metrics for the variable.
	GetMetrics() VariableMetrics
	// GetStorageStrategy returns the current storage strategy of the variable.
	GetStorageStrategy() StorageStrategy
	// CanMigrateTo checks if the variable can be migrated to a new storage strategy.
	CanMigrateTo(StorageStrategy) bool

	// Close releases any resources used by the variable.
	Close() error
	// Size returns the memory footprint of the variable in bytes.
	Size() int64

	// AsUntyped returns the underlying non-type-safe variable.
	AsUntyped() Variable
}

// StorageStrategy defines the different storage strategies for a synchronized variable.
type StorageStrategy uint8

const (
	// StrategyBasic is the default strategy, using basic locking.
	StrategyBasic StorageStrategy = iota
	// StrategyAtomic uses atomic operations for synchronization.
	StrategyAtomic
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
type HistoryEntry struct {
	Version   uint64         // Version of the variable
	Timestamp int64          // Timestamp of the change
	Value     any            // Value of the variable at this version
	ClientID  string         // ID of the client that made the change
	Operation string         // Type of operation (e.g., "set", "delete")
	Metadata  map[string]any // Additional metadata
}

// TypedHistoryEntry represents a type-safe history entry.
type TypedHistoryEntry[T any] struct {
	Version   uint64         // Version of the variable
	Timestamp int64          // Timestamp of the change
	Value     T              // Value of the variable at this version
	ClientID  string         // ID of the client that made the change
	Operation string         // Type of operation
	Metadata  map[string]any // Additional metadata
}

// ConflictResolver defines the interface for resolving conflicts between local and remote versions of a variable.
type ConflictResolver interface {
	// Resolve resolves a conflict between a local and remote value.
	Resolve(local, remote any, metadata ConflictMetadata) any
	// CanResolve checks if the resolver can handle a specific conflict.
	CanResolve(local, remote any) bool
	// Priority returns the priority of the resolver.
	Priority() int
}

// TypedConflictResolver defines the interface for a type-safe conflict resolver.
type TypedConflictResolver[T any] interface {
	// Resolve resolves a conflict between a local and remote value.
	Resolve(local, remote T, metadata ConflictMetadata) T
	// CanResolve checks if the resolver can handle a specific conflict.
	CanResolve(local, remote T) bool
	// Priority returns the priority of the resolver.
	Priority() int
}

// ConflictMetadata contains additional information about a conflict.
type ConflictMetadata map[string]any

// Delta represents a change to a variable.
type Delta struct {
	Version       uint64         // Version of the change
	Path          string         // Path to the changed field (for nested objects)
	PreviousValue any            // Previous value of the field
	Value         any            // New value of the field
	Timestamp     time.Time      // Timestamp of the change
	ClientID      string         // ID of the client that made the change
	Metadata      map[string]any // Additional metadata
}

// TypedDelta represents a type-safe change to a variable.
type TypedDelta[T any] struct {
	Version   uint64         // Version of the change
	Path      string         // Path to the changed field
	OldValue  T              // Previous value
	NewValue  T              // New value
	Timestamp time.Time      // Timestamp of the change
	ClientID  string         // ID of the client
	Metadata  map[string]any // Additional metadata
}

// VariableMetrics contains performance and usage metrics for a variable.
type VariableMetrics struct {
	// Operation counts
	ReadCount     uint64 // Total number of read operations
	WriteCount    uint64 // Total number of write operations
	ConflictCount uint64 // Total number of conflicts
	ErrorCount    uint64 // Total number of errors

	// Performance metrics
	AvgReadLatency  time.Duration // Average latency of read operations
	AvgWriteLatency time.Duration // Average latency of write operations
	P99ReadLatency  time.Duration // 99th percentile read latency
	P99WriteLatency time.Duration // 99th percentile write latency

	// Concurrency metrics
	MaxConcurrentReaders uint32 // Maximum number of concurrent readers
	MaxConcurrentWriters uint32 // Maximum number of concurrent writers
	ContentionEvents     uint64 // Number of contention events

	// Memory metrics
	MemoryUsage    int64   // Memory usage in bytes
	AllocationRate float64 // Allocation rate in allocs/sec

	// Pattern analysis
	AccessPattern  AccessPattern // Detected access pattern
	HotSpotRatio   float64       // Ratio of operations in the last 10% of time
	ReadWriteRatio float64       // Ratio of reads to writes

	// Time-based
	CreatedAt    time.Time // Time when the variable was created
	LastAccessed time.Time // Time when the variable was last accessed
	LastModified time.Time // Time when the variable was last modified

	// Migration history
	MigrationHistory []MigrationEvent // History of migrations
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
	Metrics         VariableMetrics          // Metrics used for the analysis
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
