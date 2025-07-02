package sync

import (
	"github.com/zeusync/zeusync/pkg/encoding"
	"time"

	"github.com/zeusync/zeusync/internal/core/syncv2/metrics"
	"github.com/zeusync/zeusync/internal/core/syncv2/types"
)

// Root provides a clean, simple interface for synchronized variables
type Root[T any] interface {
	// Get returns the current value
	Get() T
	// Set updates the value
	Set(T)
	// Version returns the current version number
	Version() uint64
	// IsDirty returns true if the value has been modified since last clean
	IsDirty() bool
	// MarkClean marks the value as clean
	MarkClean()
}

// Networked provides a clean, simple interface for networked variables
type Networked[T any] interface {
	Delta(since uint64) Delta[T]
	ApplyDelta(...Delta[T]) error

	SetResolver(ConflictResolver[T])

	Marshal() ([]byte, error)
	Unmarshal([]byte) error
}

// Observable provides a clean, simple interface for observing variable changes.
type Observable[T any] interface {
	OnChange(func(old, new T))
	OnConflict(func(local, remote T) T)

	Metrics() metrics.VariableMetrics
}

// Permissioned provides a clean, simple interface for managing permissions for variable access.
type Permissioned interface {
	CheckRead(identifier string) bool
	CheckWrite(identifier string) bool
	SetPermissions(permissions Permissions)
}

// Core provides a clean, simple interface for synchronized variables
type Core[T any] interface {
	Root[T]
	Networked[T]
	Observable[T]
	encoding.Serializable[T]
	Permissioned

	Close() error
	Size() int
}

// Variable provides a clean, simple interface for synchronized variables
// This is the interface that users will interact with, hiding all internal complexity
type Variable[T any] interface {
	// Get returns the current value
	Get() T

	// Set updates the value
	Set(value T)

	// Update applies a function to update the value atomically
	Update(callback func(current T) T) T

	// Subscribe to value changes
	Subscribe(callback func(oldValue, newValue T)) UnsubscribeFunc

	// GetVersion returns the current version number
	GetVersion() uint64

	// Close releases resources
	Close() error
}

type Marshaller[T any] interface {
	Marshal(T) ([]byte, error)
	Unmarshal([]byte) (T, error)
}

// UnsubscribeFunc is returned by Subscribe to allow unsubscribing
type UnsubscribeFunc func()

// Option provides configuration options for creating simple variables
type Option struct {
	// Strategy specifies the storage strategy to use
	Strategy types.StorageStrategy

	// AutoOptimize enables automatic strategy optimization based on usage patterns
	AutoOptimize bool

	// MetricsEnabled enables metrics collection
	MetricsEnabled bool

	// NetworkEnabled enables network synchronization
	NetworkEnabled bool

	// PersistenceEnabled enables automatic persistence
	PersistenceEnabled bool

	// Custom configuration for specific strategies
	Config types.Configuration
}

// Permissions defines the read and write permissions for a variable.
type Permissions struct {
	Read  PermissionSet // Unique list of identifiers with read access
	Write PermissionSet // Unique list of identifiers with write access
}

// PermissionSet is a map of identifiers to permissions. Alias for `map[string]struct{}`
type PermissionSet map[string]struct{}

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

// AnalysisResult contains the results of a variable analysis.
type AnalysisResult struct {
	Strategy        types.StorageStrategy    // Recommended storage strategy
	Confidence      float64                  // Confidence in the recommendation
	Reasoning       string                   // Explanation for the recommendation
	Metrics         metrics.Metrics          // Metrics used for the analysis
	Recommendations []OptimizationSuggestion // List of optimization suggestions
	Timestamp       time.Time                // Timestamp of the analysis
}

// OptimizationSuggestion provides a suggestion for optimizing a variable.
type OptimizationSuggestion struct {
	Type        string                // Type of suggestion (e.g., "migrate", "tune")
	Priority    int                   // Priority of the suggestion (1-10)
	Description string                // Description of the suggestion
	Impact      string                // Estimated impact of the optimization
	Effort      string                // Estimated effort to implement the optimization
	Strategy    types.StorageStrategy // Recommended strategy (if applicable)
}

// MigrationPlan outlines the steps for migrating a variable to a new strategy.
type MigrationPlan struct {
	Source        types.StorageStrategy // Source storage strategy
	Target        types.StorageStrategy // Target storage strategy
	Steps         []MigrationStep       // Steps in the migration plan
	EstimatedTime time.Duration         // Estimated time for the migration
	RiskLevel     string                // Estimated risk level of the migration
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
	From      types.StorageStrategy // Source storage strategy
	To        types.StorageStrategy // Target storage strategy
	Reason    string                // Reason for the migration
	Timestamp time.Time             // Timestamp of the migration
	Success   bool                  // Whether the migration was successful
	Duration  time.Duration         // Duration of the migration
}
