package factory

import (
	"fmt"
	"reflect"
	sc "sync"

	"github.com/zeusync/zeusync/internal/core/sync/metrics"

	"github.com/zeusync/zeusync/internal/core/sync"
	"github.com/zeusync/zeusync/internal/core/sync/analyzer"
	"github.com/zeusync/zeusync/internal/core/sync/vars"
)

var _ sync.VariableFactory = (*DefaultVariableFactory)(nil)

// DefaultVariableFactory provides a default implementation of the sync.VariableFactory.
// It manages the creation of synchronized variables with various strategies.
type DefaultVariableFactory struct {
	strategies map[sync.StorageStrategy]sync.StrategyConstructor // Map of registered strategies
	analyzer   sync.VariableAnalyzer                             // Analyzer for recommending strategies
	mu         sc.RWMutex                                        // Mutex for thread-safe access to strategies
}

// NewVariableFactory creates and initializes a new DefaultVariableFactory.
// It also registers the default synchronization strategies.
func NewVariableFactory() *DefaultVariableFactory {
	f := &DefaultVariableFactory{
		strategies: make(map[sync.StorageStrategy]sync.StrategyConstructor),
		analyzer:   analyzer.NewDefaultAnalyzer(),
	}

	// Register default strategies
	f.RegisterStrategy(sync.StrategyAtomic, f.createAtomicVariable)
	f.RegisterStrategy(sync.StrategyMutex, f.createMutexVariable)
	f.RegisterStrategy(sync.StrategySharded, f.createShardedVariable)
	f.RegisterStrategy(sync.StrategyReadOptimized, f.createReadOptimizedVariable)
	f.RegisterStrategy(sync.StrategyWriteOptimized, f.createWriteOptimizedVariable)

	return f
}

// Create creates a new synchronized variable with an automatically selected strategy.
// It analyzes the initial value and access pattern to choose the most optimal strategy.
func (f *DefaultVariableFactory) Create(initialValue any, options ...sync.VariableOption) (sync.Variable, error) {
	config := f.buildConfig(options...)

	// Auto-select strategy if not specified
	if config.StorageStrategy == sync.StrategyBasic {
		config.StorageStrategy = f.selectOptimalStrategy(initialValue, sync.PatternMixed)
	}

	return f.CreateWithStrategy(config.StorageStrategy, initialValue, options...)
}

// CreateWithStrategy creates a new synchronized variable with a specified strategy.
func (f *DefaultVariableFactory) CreateWithStrategy(strategy sync.StorageStrategy, initialValue any, options ...sync.VariableOption) (sync.Variable, error) {
	config := f.buildConfig(options...)
	config.StorageStrategy = strategy

	f.mu.RLock()
	constructor, exists := f.strategies[strategy]
	f.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("unknown strategy: %d", strategy)
	}

	variable, err := constructor(config)
	if err != nil {
		return nil, err
	}

	// Initialize with value
	if err = variable.Set(initialValue); err != nil {
		return nil, err
	}

	// Wrap with metrics if enabled
	if config.EnableMetrics {
		variable = metrics.NewMetricsWrapper(variable)
	}

	return variable, nil
}

// CreateOptimized creates a new synchronized variable optimized for a specific access pattern.
func (f *DefaultVariableFactory) CreateOptimized(initialValue any, expectedPattern sync.AccessPattern, options ...sync.VariableOption) (sync.Variable, error) {
	strategy := f.selectOptimalStrategy(initialValue, expectedPattern)
	return f.CreateWithStrategy(strategy, initialValue, options...)
}

// RegisterStrategy registers a new synchronization strategy.
func (f *DefaultVariableFactory) RegisterStrategy(strategy sync.StorageStrategy, constructor sync.StrategyConstructor) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.strategies[strategy] = constructor
}

// ListStrategies returns a list of all registered strategy names.
func (f *DefaultVariableFactory) ListStrategies() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	strategies := make([]string, 0, len(f.strategies))
	for name := range f.strategies {
		strategies = append(strategies, string(name))
	}

	return strategies
}

// GetRecommendedStrategy recommends a storage strategy based on the value type and access pattern.
func (f *DefaultVariableFactory) GetRecommendedStrategy(valueType reflect.Type, pattern sync.AccessPattern) sync.StorageStrategy {
	return f.selectOptimalStrategy(reflect.Zero(valueType).Interface(), pattern)
}

func (f *DefaultVariableFactory) selectOptimalStrategy(value any, pattern sync.AccessPattern) sync.StorageStrategy {
	valueType := reflect.TypeOf(value)

	// Type-based optimization
	switch valueType.Kind() {
	case reflect.Int, reflect.Int32, reflect.Int64:
		switch pattern {
		case sync.PatternReadHeavy:
			return sync.StrategyAtomic
		case sync.PatternWriteHeavy:
			return sync.StrategyWriteOptimized
		default:
			return sync.StrategyAtomic
		}

	case reflect.String:
		switch pattern {
		case sync.PatternReadHeavy:
			return sync.StrategyReadOptimized
		case sync.PatternWriteHeavy:
			return sync.StrategyMutex
		default:
			return sync.StrategyMutex
		}

	case reflect.Slice, reflect.Map:
		// Complex types benefit from sharding under high concurrency
		switch pattern {
		case sync.PatternReadHeavy, sync.PatternWriteHeavy:
			return sync.StrategySharded
		default:
			return sync.StrategyMutex
		}

	default:
		return sync.StrategyMutex
	}
}

func (f *DefaultVariableFactory) buildConfig(options ...sync.VariableOption) sync.VariableConfig {
	config := sync.VariableConfig{
		MaxHistory:    10,
		EnableMetrics: false,
		EnableHistory: false,
		Permissions:   sync.PermissionRead | sync.PermissionWrite,
	}

	for _, option := range options {
		option(&config)
	}

	return config
}

func (f *DefaultVariableFactory) createAtomicVariable(config sync.VariableConfig) (sync.Variable, error) {
	return vars.NewAtomicVariable(config), nil
}

func (f *DefaultVariableFactory) createMutexVariable(config sync.VariableConfig) (sync.Variable, error) {
	return vars.NewMutexVariable(config), nil
}

func (f *DefaultVariableFactory) createShardedVariable(config sync.VariableConfig) (sync.Variable, error) {
	// return NewShardedVariable(config), nil
	return vars.NewAtomicVariable(config), nil
}

func (f *DefaultVariableFactory) createReadOptimizedVariable(config sync.VariableConfig) (sync.Variable, error) {
	// return NewReadOptimizedVariable(config), nil
	return vars.NewAtomicVariable(config), nil
}

func (f *DefaultVariableFactory) createWriteOptimizedVariable(config sync.VariableConfig) (sync.Variable, error) {
	// return NewWriteOptimizedVariable(config), nil
	return vars.NewAtomicVariable(config), nil
}
