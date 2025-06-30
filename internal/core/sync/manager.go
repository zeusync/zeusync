package sync

import (
	"context"
	"time"
)

type VariableManager interface {
	// Variable lifecycle

	Create(name string, initialValue any, options ...VariableOption) (Variable, error)
	Get(name string) (Variable, bool)
	Delete(name string) error
	List() []string

	// Bulk operations

	CreateBatch(specs []VariableSpec) error
	MigrateBatch(names []string, strategy StorageStrategy) error

	// Analytics integration

	SetAnalyzer(VariableAnalyzer)
	SetMigrator(VariableMigrator)

	// Auto-optimization

	EnableAutoOptimization(ctx context.Context, interval time.Duration)
	DisableAutoOptimization()

	// Monitoring

	GetGlobalMetrics() GlobalMetrics
	GetVariable(name string) (Variable, bool)

	// Resource management

	Close() error
	GarbageCollect() error
}
