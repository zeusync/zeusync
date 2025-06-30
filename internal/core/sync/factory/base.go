package factory

import (
	"fmt"
	"reflect"
	sc "sync"

	"github.com/zeusync/zeusync/internal/core/sync"
	"github.com/zeusync/zeusync/internal/core/sync/analyzer"
	"github.com/zeusync/zeusync/internal/core/sync/vars"
)

var _ sync.VariableFactory = (*DefaultVariableFactory)(nil)

type DefaultVariableFactory struct {
	strategies map[sync.StorageStrategy]sync.StrategyConstructor
	analyzer   sync.VariableAnalyzer
	mu         sc.RWMutex
}

func NewVariableFactory() *DefaultVariableFactory {
	f := &DefaultVariableFactory{
		strategies: make(map[sync.StorageStrategy]sync.StrategyConstructor),
		analyzer:   analyzer.NewDefaultAnalyzer(),
	}

	// Register default strategies
	f.RegisterStrategy(string(sync.StrategyAtomic), f.createAtomicVariable)
	f.RegisterStrategy(string(sync.StrategyMutex), f.createMutexVariable)
	f.RegisterStrategy(string(sync.StrategySharded), f.createShardedVariable)
	f.RegisterStrategy(string(sync.StrategyReadOptimized), f.createReadOptimizedVariable)
	f.RegisterStrategy(string(sync.StrategyWriteOptimized), f.createWriteOptimizedVariable)

	return f
}

func (f *DefaultVariableFactory) Create(initialValue any, options ...sync.VariableOption) (sync.Variable, error) {
	config := f.buildConfig(options...)

	// Auto-select strategy if not specified
	if config.StorageStrategy == sync.StrategyBasic {
		config.StorageStrategy = f.selectOptimalStrategy(initialValue, sync.PatternMixed)
	}

	return f.CreateWithStrategy(config.StorageStrategy, initialValue, options...)
}

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
		// variable = sync.NewMetricsWrapper(variable)
	}

	return variable, nil
}

func (f *DefaultVariableFactory) CreateOptimized(initialValue any, expectedPattern sync.AccessPattern, options ...sync.VariableOption) (sync.Variable, error) {
	strategy := f.selectOptimalStrategy(initialValue, expectedPattern)
	return f.CreateWithStrategy(strategy, initialValue, options...)
}

func (f *DefaultVariableFactory) RegisterStrategy(name string, constructor sync.StrategyConstructor) {
	// TODO implement me
	panic("implement me")
}

func (f *DefaultVariableFactory) ListStrategies() []string {
	// TODO implement me
	panic("implement me")
}

func (f *DefaultVariableFactory) GetRecommendedStrategy(valueType reflect.Type, pattern sync.AccessPattern) sync.StorageStrategy {
	// TODO implement me
	panic("implement me")
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
