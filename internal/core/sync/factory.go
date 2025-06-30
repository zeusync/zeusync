package sync

import (
	"context"
	"reflect"
	"time"
)

// VariableFactory defines the interface for creating synchronized variables.
// It supports different creation methods, including basic, strategy-specific, and optimized creation.
type VariableFactory interface {
	// Create creates a new synchronized variable with an automatically selected strategy.
	Create(initialValue any, options ...VariableOption) (Variable, error)

	// CreateWithStrategy creates a new synchronized variable with a specified strategy.
	CreateWithStrategy(strategy StorageStrategy, initialValue any, options ...VariableOption) (Variable, error)

	// CreateOptimized creates a new synchronized variable optimized for a specific access pattern.
	CreateOptimized(initialValue any, expectedPattern AccessPattern, options ...VariableOption) (Variable, error)

	// RegisterStrategy registers a new synchronization strategy.
	RegisterStrategy(strategy StorageStrategy, constructor StrategyConstructor)
	// ListStrategies returns a list of all registered strategy names.
	ListStrategies() []string
	// GetRecommendedStrategy recommends a storage strategy based on the value type and access pattern.
	GetRecommendedStrategy(valueType reflect.Type, pattern AccessPattern) StorageStrategy
}

// TypedVariableFactory defines the interface for creating type-safe synchronized variables.
type TypedVariableFactory[T any] interface {
	// Create creates a new type-safe synchronized variable with an automatically selected strategy.
	Create(initialValue T, options ...VariableOption) (TypedVariable[T], error)

	// CreateWithStrategy creates a new type-safe synchronized variable with a specified strategy.
	CreateWithStrategy(strategy StorageStrategy, initialValue T, options ...VariableOption) (TypedVariable[T], error)

	// CreateOptimized creates a new type-safe synchronized variable optimized for a specific access pattern.
	CreateOptimized(initialValue T, expectedPattern AccessPattern, options ...VariableOption) (TypedVariable[T], error)

	// RegisterStrategy registers a new synchronization strategy.
	RegisterStrategy(name string, constructor StrategyConstructor)
	// ListStrategies returns a list of all registered strategy names.
	ListStrategies() []string
	// GetRecommendedStrategy recommends a storage strategy based on the value type and access pattern.
	GetRecommendedStrategy(valueType reflect.Type, pattern AccessPattern) StorageStrategy
}

// VariableAnalyzer defines the interface for analyzing the performance of synchronized variables.
type VariableAnalyzer interface {
	// AnalyzeVariable performs a real-time analysis of a variable.
	AnalyzeVariable(v Variable) AnalysisResult
	// MonitorVariable continuously monitors a variable and sends analysis results to a channel.
	MonitorVariable(ctx context.Context, v Variable, interval time.Duration) <-chan AnalysisResult

	// DetectAccessPattern detects the access pattern of a variable based on its metrics.
	DetectAccessPattern(metrics VariableMetrics) AccessPattern
	// PredictOptimalStrategy predicts the optimal storage strategy for a variable based on its metrics.
	PredictOptimalStrategy(metrics VariableMetrics) StorageStrategy

	// GetOptimizationSuggestions provides a list of optimization suggestions for a variable.
	GetOptimizationSuggestions(v Variable) []OptimizationSuggestion
	// ShouldMigrate determines if a variable should be migrated to a different strategy.
	ShouldMigrate(v Variable) (bool, StorageStrategy, string)
}

// VariableMigrator defines the interface for migrating a variable from one storage strategy to another.
type VariableMigrator interface {
	// Migrate migrates a variable to a new storage strategy.
	Migrate(v Variable, targetStrategy StorageStrategy) (Variable, error)

	// PlanMigration creates a migration plan for a variable.
	PlanMigration(v Variable, targetStrategy StorageStrategy) MigrationPlan
	// EstimateMigrationCost estimates the cost of a migration plan.
	EstimateMigrationCost(plan MigrationPlan) MigrationCost

	// HotMigrate performs a hot migration with zero downtime.
	HotMigrate(ctx context.Context, v Variable, targetStrategy StorageStrategy) (Variable, error)
}

// TypedVariableMigrator defines the interface for migrating a type-safe variable.
type TypedVariableMigrator[T any] interface {
	// Migrate migrates a type-safe variable to a new storage strategy.
	Migrate(v TypedVariable[T], targetStrategy StorageStrategy) (TypedVariable[T], error)

	// PlanMigration creates a migration plan for a variable.
	PlanMigration(v Variable, targetStrategy StorageStrategy) MigrationPlan
	// EstimateMigrationCost estimates the cost of a migration plan.
	EstimateMigrationCost(plan MigrationPlan) MigrationCost

	// HotMigrate performs a hot migration with zero downtime.
	HotMigrate(ctx context.Context, v Variable, targetStrategy StorageStrategy) (Variable, error)
}

// VariableOption is a function that configures a variable.
type VariableOption func(*VariableConfig)

// VariableConfig holds the configuration for a synchronized variable.
type VariableConfig struct {
	Name             string            // Name of the variable
	MaxHistory       uint8             // Maximum number of historical versions to keep
	Permissions      PermissionMask    // Access permissions for the variable
	ConflictResolver ConflictResolver  // Conflict resolver for concurrent writes
	StorageStrategy  StorageStrategy   // The underlying storage strategy
	EnableMetrics    bool              // Flag to enable/disable metrics collection
	EnableHistory    bool              // Flag to enable/disable history tracking
	TTL              time.Duration     // Time-to-live for the variable
	Tags             map[string]string // Tags for categorizing and filtering variables
}

// VariableSpec defines the specification for creating a new variable.
type VariableSpec struct {
	Name         string           // Name of the variable
	InitialValue any              // Initial value of the variable
	Options      []VariableOption // Configuration options for the variable
}

// StrategyConstructor is a function that creates a new variable with a specific strategy.
type StrategyConstructor func(config VariableConfig) (Variable, error)

// WithMaxHistory sets the maximum number of historical versions to keep.
func WithMaxHistory(maxHistory uint8) VariableOption {
	return func(c *VariableConfig) { c.MaxHistory = maxHistory }
}

// WithPermissions sets the access permissions for the variable.
func WithPermissions(permissions PermissionMask) VariableOption {
	return func(c *VariableConfig) { c.Permissions = permissions }
}

// WithStrategy sets the storage strategy for the variable.
func WithStrategy(strategy StorageStrategy) VariableOption {
	return func(c *VariableConfig) { c.StorageStrategy = strategy }
}

// WithMetrics enables or disables metrics collection for the variable.
func WithMetrics(enabled bool) VariableOption {
	return func(c *VariableConfig) { c.EnableMetrics = enabled }
}

// WithTTL sets the time-to-live for the variable.
func WithTTL(ttl time.Duration) VariableOption {
	return func(c *VariableConfig) { c.TTL = ttl }
}

// WithTags sets the tags for the variable.
func WithTags(tags map[string]string) VariableOption {
	return func(c *VariableConfig) { c.Tags = tags }
}
