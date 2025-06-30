package sync

import (
	"context"
	"reflect"
	"time"
)

// Factory defines the interface for creating synchronized variables.
// It supports different creation methods, including basic, strategy-specific, and optimized creation.
type Factory[T any] interface {
	// Create creates a new synchronized variable with an automatically selected strategy.
	Create(initialValue any, options ...Option) (Core[T], error)

	// CreateWithStrategy creates a new synchronized variable with a specified strategy.
	CreateWithStrategy(strategy StorageStrategy, initialValue T, options ...Option) (Core[T], error)

	// CreateOptimized creates a new synchronized variable optimized for a specific access pattern.
	CreateOptimized(initialValue T, expectedPattern AccessPattern, options ...Option) (Core[T], error)

	// RegisterStrategy registers a new synchronization strategy.
	RegisterStrategy(strategy StorageStrategy, constructor StrategyConstructor[T])
	// ListStrategies returns a list of all registered strategy names.
	ListStrategies() []string
	// GetRecommendedStrategy recommends a storage strategy based on the value type and access pattern.
	GetRecommendedStrategy(valueType reflect.Type, pattern AccessPattern) StorageStrategy
}

// Analyzer defines the interface for analyzing the performance of synchronized variables.
type Analyzer[T any] interface {
	// AnalyzeVariable performs a real-time analysis of a variable.
	AnalyzeVariable(v Core[T]) AnalysisResult
	// MonitorVariable continuously monitors a variable and sends analysis results to a channel.
	MonitorVariable(ctx context.Context, v Core[T], interval time.Duration) <-chan AnalysisResult

	// DetectAccessPattern detects the access pattern of a variable based on its metrics.
	DetectAccessPattern(metrics Metrics) AccessPattern
	// PredictOptimalStrategy predicts the optimal storage strategy for a variable based on its metrics.
	PredictOptimalStrategy(metrics Metrics) StorageStrategy

	// GetOptimizationSuggestions provides a list of optimization suggestions for a variable.
	GetOptimizationSuggestions(v Core[T]) []OptimizationSuggestion
	// ShouldMigrate determines if a variable should be migrated to a different strategy.
	ShouldMigrate(v Core[T]) (bool, StorageStrategy, string)
}

// Migrator defines the interface for migrating a variable from one storage strategy to another.
type Migrator[T any] interface {
	// Migrate migrates a variable to a new storage strategy.
	Migrate(v Core[T], targetStrategy StorageStrategy) (Core[T], error)

	// PlanMigration creates a migration plan for a variable.
	PlanMigration(v Core[T], targetStrategy StorageStrategy) MigrationPlan
	// EstimateMigrationCost estimates the cost of a migration plan.
	EstimateMigrationCost(plan MigrationPlan) MigrationCost

	// HotMigrate performs a hot migration with zero downtime.
	HotMigrate(ctx context.Context, v Core[T], targetStrategy StorageStrategy) (Core[T], error)
}

// Option is a function that configures a variable.
type Option func(*Config)

// Config holds the configuration for a synchronized variable.
type Config struct {
	Name            string            // Name of the variable
	MaxHistory      uint8             // Maximum number of historical versions to keep
	Permissions     PermissionMask    // Access permissions for the variable
	StorageStrategy StorageStrategy   // The underlying storage strategy
	EnableMetrics   bool              // Flag to enable/disable metrics collection
	EnableHistory   bool              // Flag to enable/disable history tracking
	TTL             time.Duration     // Time-to-live for the variable
	Tags            map[string]string // Tags for categorizing and filtering variables
}

// VariableSpec defines the specification for creating a new variable.
type VariableSpec struct {
	Name         string   // Name of the variable
	InitialValue any      // Initial value of the variable
	Options      []Option // Configuration options for the variable
}

// StrategyConstructor is a function that creates a new variable with a specific strategy.
type StrategyConstructor[T any] func(config Config) (Core[T], error)

// WithMaxHistory sets the maximum number of historical versions to keep.
func WithMaxHistory(maxHistory uint8) Option {
	return func(c *Config) { c.MaxHistory = maxHistory }
}

// WithPermissions sets the access permissions for the variable.
func WithPermissions(permissions PermissionMask) Option {
	return func(c *Config) { c.Permissions = permissions }
}

// WithStrategy sets the storage strategy for the variable.
func WithStrategy(strategy StorageStrategy) Option {
	return func(c *Config) { c.StorageStrategy = strategy }
}

// WithMetrics enables or disables metrics collection for the variable.
func WithMetrics(enabled bool) Option {
	return func(c *Config) { c.EnableMetrics = enabled }
}

// WithTTL sets the time-to-live for the variable.
func WithTTL(ttl time.Duration) Option {
	return func(c *Config) { c.TTL = ttl }
}

// WithTags sets the tags for the variable.
func WithTags(tags map[string]string) Option {
	return func(c *Config) { c.Tags = tags }
}
