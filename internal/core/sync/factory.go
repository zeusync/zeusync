package sync

import (
	"context"
	"reflect"
	"time"
)

type VariableFactory interface {
	// Basic creation

	Create(initialValue any, options ...VariableOption) (Variable, error)

	// Strategy-specific creation

	CreateWithStrategy(strategy StorageStrategy, initialValue any, options ...VariableOption) (Variable, error)

	// Auto-optimized creation

	CreateOptimized(initialValue any, expectedPattern AccessPattern, options ...VariableOption) (Variable, error)

	// Registry management

	RegisterStrategy(name string, constructor StrategyConstructor)
	ListStrategies() []string
	GetRecommendedStrategy(valueType reflect.Type, pattern AccessPattern) StorageStrategy
}

type TypedVariableFactory[T any] interface {
	// Basic creation

	Create(initialValue T, options ...VariableOption) (TypedVariable[T], error)

	// Strategy-specific creation

	CreateWithStrategy(strategy StorageStrategy, initialValue T, options ...VariableOption) (TypedVariable[T], error)

	// Auto-optimized creation

	CreateOptimized(initialValue T, expectedPattern AccessPattern, options ...VariableOption) (TypedVariable[T], error)

	// Registry management

	RegisterStrategy(name string, constructor StrategyConstructor)
	ListStrategies() []string
	GetRecommendedStrategy(valueType reflect.Type, pattern AccessPattern) StorageStrategy
}

type VariableAnalyzer interface {
	// Real-time analysis

	AnalyzeVariable(v Variable) AnalysisResult
	MonitorVariable(ctx context.Context, v Variable, interval time.Duration) <-chan AnalysisResult

	// Pattern detection

	DetectAccessPattern(metrics VariableMetrics) AccessPattern
	PredictOptimalStrategy(metrics VariableMetrics) StorageStrategy

	// Performance recommendations

	GetOptimizationSuggestions(v Variable) []OptimizationSuggestion
	ShouldMigrate(v Variable) (bool, StorageStrategy, string)
}

type VariableMigrator interface {
	// Migration operations

	Migrate(v Variable, targetStrategy StorageStrategy) (Variable, error)

	// Migration planning

	PlanMigration(v Variable, targetStrategy StorageStrategy) MigrationPlan
	EstimateMigrationCost(plan MigrationPlan) MigrationCost

	// Hot migration (zero-downtime)

	HotMigrate(ctx context.Context, v Variable, targetStrategy StorageStrategy) (Variable, error)
}

type TypedVariableMigrator[T any] interface {
	// Migration operations

	Migrate(v TypedVariable[T], targetStrategy StorageStrategy) (TypedVariable[T], error)

	// Migration planning

	PlanMigration(v Variable, targetStrategy StorageStrategy) MigrationPlan
	EstimateMigrationCost(plan MigrationPlan) MigrationCost

	// Hot migration (zero-downtime)

	HotMigrate(ctx context.Context, v Variable, targetStrategy StorageStrategy) (Variable, error)
}

type VariableOption func(*VariableConfig)

type VariableConfig struct {
	MaxHistory       uint8
	Permissions      PermissionMask
	ConflictResolver ConflictResolver
	StorageStrategy  StorageStrategy
	EnableMetrics    bool
	EnableHistory    bool
	TTL              time.Duration
	Tags             map[string]string
}

type VariableSpec struct {
	Name         string
	InitialValue any
	Options      []VariableOption
}

type StrategyConstructor func(config VariableConfig) (Variable, error)

func WithMaxHistory(maxHistory uint8) VariableOption {
	return func(c *VariableConfig) { c.MaxHistory = maxHistory }
}

func WithPermissions(permissions PermissionMask) VariableOption {
	return func(c *VariableConfig) { c.Permissions = permissions }
}

func WithStrategy(strategy StorageStrategy) VariableOption {
	return func(c *VariableConfig) { c.StorageStrategy = strategy }
}

func WithMetrics(enabled bool) VariableOption {
	return func(c *VariableConfig) { c.EnableMetrics = enabled }
}

func WithTTL(ttl time.Duration) VariableOption {
	return func(c *VariableConfig) { c.TTL = ttl }
}

func WithTags(tags map[string]string) VariableOption {
	return func(c *VariableConfig) { c.Tags = tags }
}
