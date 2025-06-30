package sync

import (
	"reflect"
	"time"
)

type Variable interface {
	// Core operations

	Get() (any, error)
	Set(any) error

	// State management

	IsDirty() bool
	MarkClean()
	GetVersion() uint64

	// Delta operations

	GetDelta(sinceVersion uint64) ([]Delta, error)
	ApplyDelta(...Delta) error

	// Conflict resolution

	SetConflictResolver(ConflictResolver)
	OnChange(func(oldValue, newValue any))
	OnConflict(func(local, remote any) any)

	// Permissions

	GetPermissions() PermissionMask
	SetPermissions(PermissionMask)

	// Performance and lifecycle

	GetMetrics() VariableMetrics
	GetStorageStrategy() StorageStrategy
	CanMigrateTo(StorageStrategy) bool

	// Metadata

	GetType() reflect.Type
	GetHistory() []HistoryEntry

	// Resource management

	Close() error
	Size() int64 // Memory footprint
}

type TypedVariable[T any] interface {
	// Core operations

	Get() (T, error)
	Set(T) error

	// State management

	IsDirty() bool
	MarkClean()
	GetVersion() uint64

	// Delta operations

	GetDelta(sinceVersion uint64) ([]Delta, error)
	ApplyDelta(...Delta) error

	// Conflict resolution

	SetConflictResolver(ConflictResolver)
	OnChange(func(oldValue, newValue T))
	OnConflict(func(local, remote T) T)

	// Permissions

	GetPermissions() PermissionMask
	SetPermissions(PermissionMask)

	// Metadata

	GetHistory() []HistoryEntry

	// Performance and lifecycle

	GetMetrics() VariableMetrics
	GetStorageStrategy() StorageStrategy
	CanMigrateTo(StorageStrategy) bool

	// Resource management

	Close() error
	Size() int64

	// Access to untyped interface

	AsUntyped() Variable
}

type StorageStrategy uint8

const (
	// Basic strategies

	StrategyBasic StorageStrategy = iota // Basic locking and synchronization

	StrategyAtomic   // atomic.Value
	StrategyMutex    // RWMutex protected
	StrategyChannels // Channel-based

	// Advanced strategies

	StrategySharded    // Sharded for high concurrency
	StrategyLockFree   // Lock-free structures
	StrategyPersisted  // Disk-backed
	StrategyReplicated // Multi-node replication

	// Specialized strategies

	StrategyWriteOptimized  // Write-heavy workloads
	StrategyReadOptimized   // Read-heavy workloads
	StrategyMemoryOptimized // Low memory usage
)

type Permissions struct {
	Read  []string
	Write []string
}

type HistoryEntry struct {
	Version   uint64
	Timestamp int64
	Value     any
	ClientID  string
	Operation string // "set", "delete", "migrate"
	Metadata  map[string]any
}

type TypedHistoryEntry[T any] struct {
	Version   uint64
	Timestamp int64
	Value     T
	ClientID  string
	Operation string
	Metadata  map[string]any
}

type ConflictResolver interface {
	Resolve(local, remote any, metadata ConflictMetadata) any
	CanResolve(local, remote any) bool
	Priority() int
}

type TypedConflictResolver[T any] interface {
	Resolve(local, remote T, metadata ConflictMetadata) T
	CanResolve(local, remote T) bool
	Priority() int
}

type ConflictMetadata map[string]any

type Delta struct {
	Version       uint64
	Path          string
	PreviousValue any
	Value         any
	Timestamp     time.Time
	ClientID      string
	Metadata      map[string]any
}

type TypedDelta[T any] struct {
	Version   uint64
	Path      string
	OldValue  T
	NewValue  T
	Timestamp time.Time
	ClientID  string
	Metadata  map[string]any
}

type VariableMetrics struct {
	// Operation counts
	ReadCount     uint64
	WriteCount    uint64
	ConflictCount uint64

	// Performance metrics
	AvgReadLatency  time.Duration
	AvgWriteLatency time.Duration
	P99ReadLatency  time.Duration
	P99WriteLatency time.Duration

	// Concurrency metrics
	MaxConcurrentReaders uint32
	MaxConcurrentWriters uint32
	ContentionEvents     uint64

	// Memory metrics
	MemoryUsage    int64
	AllocationRate float64 // allocs/sec

	// Pattern analysis
	AccessPattern  AccessPattern
	HotSpotRatio   float64 // ratio of operations in last 10% of time
	ReadWriteRatio float64 // reads/writes

	// Time-based
	CreatedAt    time.Time
	LastAccessed time.Time
	LastModified time.Time

	// Migration history
	MigrationHistory []MigrationEvent
}

type AccessPattern uint8

const (
	PatternRandom AccessPattern = iota
	PatternBurst
	PatternSteady
	PatternReadHeavy
	PatternWriteHeavy
	PatternMixed
	PatternRareAccess
)

type AnalysisResult struct {
	Strategy        StorageStrategy
	Confidence      float64
	Reasoning       string
	Metrics         VariableMetrics
	Recommendations []OptimizationSuggestion
	Timestamp       time.Time
}

type OptimizationSuggestion struct {
	Type        string // "migrate", "tune", "replace"
	Priority    int    // 1-10
	Description string
	Impact      string // "high", "medium", "low"
	Effort      string // "high", "medium", "low"
	Strategy    StorageStrategy
}

type MigrationPlan struct {
	Source        StorageStrategy
	Target        StorageStrategy
	Steps         []MigrationStep
	EstimatedTime time.Duration
	RiskLevel     string
}

type MigrationStep struct {
	Name        string
	Description string
	Duration    time.Duration
	Reversible  bool
}

type MigrationCost struct {
	TimeEstimate   time.Duration
	MemoryOverhead int64
	CPUOverhead    float64
	RiskAssessment string
}

type MigrationEvent struct {
	From      StorageStrategy
	To        StorageStrategy
	Reason    string
	Timestamp time.Time
	Success   bool
	Duration  time.Duration
}

type GlobalMetrics struct {
	TotalVariables       int
	MemoryUsage          int64
	TotalOperations      uint64
	AverageLatency       time.Duration
	StrategyDistribution map[StorageStrategy]int
	MigrationCount       uint64
	OptimizationSavings  float64 // % improvement
}
