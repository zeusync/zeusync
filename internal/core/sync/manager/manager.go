package manager

import (
	"context"
	"fmt"
	"github.com/zeusync/zeusync/internal/core/sync/migrator"
	sc "sync"
	"time"

	"github.com/zeusync/zeusync/internal/core/sync"
	"github.com/zeusync/zeusync/internal/core/sync/analyzer"
	"github.com/zeusync/zeusync/internal/core/sync/factory"
)

var _ sync.VariableManager = (*DefaultVariableManager)(nil)

// DefaultVariableManager provides a default implementation of the sync.VariableManager.
// It manages the lifecycle of synchronized variables, including creation, retrieval, and deletion.
// It also supports advanced features like auto-optimization and metrics collection.
type DefaultVariableManager struct {
	variables map[string]sync.Variable // Map of managed variables
	factory   sync.VariableFactory     // Factory for creating new variables
	analyzer  sync.VariableAnalyzer    // Analyzer for performance monitoring and optimization
	migrator  sync.VariableMigrator    // Migrator for changing variable strategies
	mu        sc.RWMutex               // Mutex for thread-safe access to variables

	// Auto-optimization
	autoOptimize   bool               // Flag to enable/disable auto-optimization
	optimizeCancel context.CancelFunc // Function to cancel the auto-optimization loop
	optimizeWg     sc.WaitGroup       // WaitGroup to manage the optimization goroutine
}

// NewVariableManager creates and initializes a new DefaultVariableManager.
func NewVariableManager() *DefaultVariableManager {
	f := factory.NewVariableFactory()
	return &DefaultVariableManager{
		variables: make(map[string]sync.Variable),
		factory:   f,
		analyzer:  analyzer.NewDefaultAnalyzer(),
		migrator:  migrator.NewMigrator(f),
	}
}

// Create creates a new synchronized variable and adds it to the manager.
func (m *DefaultVariableManager) Create(name string, initialValue any, options ...sync.VariableOption) (sync.Variable, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.variables[name]; exists {
		return nil, fmt.Errorf("variable %s already exists", name)
	}

	variable, err := m.factory.Create(initialValue, options...)
	if err != nil {
		return nil, err
	}

	m.variables[name] = variable
	return variable, nil
}

// Get retrieves a synchronized variable by its name.
func (m *DefaultVariableManager) Get(name string) (sync.Variable, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	variable, exists := m.variables[name]
	return variable, exists
}

// Delete removes a synchronized variable from the manager.
func (m *DefaultVariableManager) Delete(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.variables[name]; !exists {
		return fmt.Errorf("variable %s not found", name)
	}

	delete(m.variables, name)
	return nil
}

// List returns the names of all managed variables.
func (m *DefaultVariableManager) List() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.variables))
	for name := range m.variables {
		names = append(names, name)
	}

	return names
}

// CreateBatch creates multiple synchronized variables at once.
func (m *DefaultVariableManager) CreateBatch(specs []sync.VariableSpec) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, spec := range specs {
		if _, exists := m.variables[spec.Name]; exists {
			return fmt.Errorf("variable %s already exists", spec.Name)
		}
	}

	for _, spec := range specs {
		variable, err := m.factory.Create(spec.InitialValue, spec.Options...)
		if err != nil {
			// Rollback created variables
			for _, s := range specs {
				if s.Name == spec.Name {
					break
				}
				delete(m.variables, s.Name)
			}
			return err
		}
		m.variables[spec.Name] = variable
	}

	return nil
}

// MigrateBatch migrates multiple variables to a new storage strategy.
func (m *DefaultVariableManager) MigrateBatch(names []string, strategy sync.StorageStrategy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, name := range names {
		variable, exists := m.variables[name]
		if !exists {
			return fmt.Errorf("variable %s not found", name)
		}

		newVariable, err := m.migrator.Migrate(variable, strategy)
		if err != nil {
			return fmt.Errorf("failed to migrate variable %s: %w", name, err)
		}

		m.variables[name] = newVariable
	}

	return nil
}

// SetAnalyzer sets the variable analyzer for the manager.
func (m *DefaultVariableManager) SetAnalyzer(variableAnalyzer sync.VariableAnalyzer) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.analyzer = variableAnalyzer
}

// SetMigrator sets the variable migrator for the manager.
func (m *DefaultVariableManager) SetMigrator(migrator sync.VariableMigrator) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.migrator = migrator
}

// EnableAutoOptimization starts the auto-optimization loop.
func (m *DefaultVariableManager) EnableAutoOptimization(ctx context.Context, interval time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.autoOptimize {
		return // Already enabled
	}

	m.autoOptimize = true
	ctx, m.optimizeCancel = context.WithCancel(ctx)

	m.optimizeWg.Add(1)
	go m.autoOptimizationLoop(ctx, interval)
}

// DisableAutoOptimization stops the auto-optimization loop.
func (m *DefaultVariableManager) DisableAutoOptimization() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.autoOptimize {
		return // Already disabled
	}

	m.autoOptimize = false
	if m.optimizeCancel != nil {
		m.optimizeCancel()
	}

	m.optimizeWg.Wait()
}

// GetGlobalMetrics returns a summary of metrics for all managed variables.
func (m *DefaultVariableManager) GetGlobalMetrics() sync.GlobalMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	globalMetrics := sync.GlobalMetrics{
		TotalVariables: len(m.variables),
	}

	// TODO: Improve with parallel workers
	for _, variable := range m.variables {
		metrics := variable.GetMetrics()

		globalMetrics.MemoryUsage += metrics.MemoryUsage
		globalMetrics.TotalOperations += metrics.ReadCount + metrics.WriteCount
		globalMetrics.AverageLatency /= metrics.AvgReadLatency + metrics.AvgWriteLatency

		for _, event := range metrics.MigrationHistory {
			globalMetrics.StrategyDistribution[event.From]++
			globalMetrics.StrategyDistribution[event.To]++
			globalMetrics.MigrationCount++
			if event.Success {
				globalMetrics.OptimizationSavings++
			}
		}
	}

	return globalMetrics
}

// GetVariable retrieves a synchronized variable by its name.
func (m *DefaultVariableManager) GetVariable(name string) (sync.Variable, bool) {
	return m.Get(name)
}

// Close stops the auto-optimization loop and clears all variables.
func (m *DefaultVariableManager) Close() error {
	m.DisableAutoOptimization()

	m.mu.Lock()
	defer m.mu.Unlock()

	m.variables = make(map[string]sync.Variable)

	return nil
}

// GarbageCollect performs garbage collection on unused variables.
func (m *DefaultVariableManager) GarbageCollect() error {
	// In a real-world scenario, this would involve more complex logic
	// to identify and remove unused variables.
	// For this example, we'll just log a message.
	fmt.Println("Garbage collection is not yet implemented.")
	return nil
}

func (m *DefaultVariableManager) autoOptimizationLoop(ctx context.Context, interval time.Duration) {
	defer m.optimizeWg.Done()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.performAutoOptimization()
		}
	}
}

func (m *DefaultVariableManager) performAutoOptimization() {
	m.mu.RLock()
	variables := make(map[string]sync.Variable)
	for name, variable := range m.variables {
		variables[name] = variable
	}
	m.mu.RUnlock()

	for name, variable := range variables {
		shouldMigrate, targetStrategy, reason := m.analyzer.ShouldMigrate(variable)
		if shouldMigrate {
			fmt.Printf("Auto-migrating variable %s to %d: %s\n", name, targetStrategy, reason)

			newVariable, err := m.migrator.Migrate(variable, targetStrategy)
			if err != nil {
				fmt.Printf("Failed to migrate variable %s: %v\n", name, err)
				continue
			}

			m.mu.Lock()
			m.variables[name] = newVariable
			m.mu.Unlock()
		}
	}
}
