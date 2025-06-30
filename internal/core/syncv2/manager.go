package sync

import (
	"context"
	"time"
)

// VariableManager defines the interface for managing a collection of synchronized variables.
// It provides methods for the entire lifecycle of variables, from creation to deletion.
// It also includes features for bulk operations, analytics, auto-optimization, and monitoring.
type VariableManager interface {
	// Create creates a new synchronized variable and adds it to the manager.
	Create(name string, initialValue any, options ...Option) (Variable, error)
	// Get retrieves a synchronized variable by its name.
	Get(name string) (Variable, bool)
	// Delete removes a synchronized variable from the manager.
	Delete(name string) error
	// List returns the names of all managed variables.
	List() []string

	// CreateBatch creates multiple synchronized variables at once.
	CreateBatch(specs []VariableSpec) error
	// MigrateBatch migrates multiple variables to a new storage strategy.
	MigrateBatch(names []string, strategy StorageStrategy) error

	// SetAnalyzer sets the variable analyzer for the manager.
	SetAnalyzer(Analyzer)
	// SetMigrator sets the variable migrator for the manager.
	SetMigrator(Migrator)

	// EnableAutoOptimization starts the auto-optimization loop.
	EnableAutoOptimization(ctx context.Context, interval time.Duration)
	// DisableAutoOptimization stops the auto-optimization loop.
	DisableAutoOptimization()

	// GetGlobalMetrics returns a summary of metrics for all managed variables.
	GetGlobalMetrics() GlobalMetrics
	// GetVariable retrieves a synchronized variable by its name.
	GetVariable(name string) (Variable, bool)

	// Close stops the auto-optimization loop and clears all variables.
	Close() error
	// GarbageCollect performs garbage collection on unused variables.
	GarbageCollect() error
}
