package analyzer

import (
	"context"
	"fmt"
	sc "sync"
	"time"

	"github.com/zeusync/zeusync/internal/core/sync"
)

var _ sync.VariableAnalyzer = (*DefaultAnalyzer)(nil)

// DefaultAnalyzer provides a default implementation of the sync.VariableAnalyzer.
// It analyzes variable metrics to detect access patterns and recommend optimal storage strategies.
type DefaultAnalyzer struct {
	mu               sc.RWMutex
	thresholds       AnalysisThresholds
	migrationHistory map[string][]sync.MigrationEvent
}

// AnalysisThresholds defines the thresholds used for performance analysis.
type AnalysisThresholds struct {
	HighContentionRatio     float64       // Ratio of contention events to total operations
	ReadHeavyRatio          float64       // Read/write ratio to be considered read-heavy
	WriteHeavyRatio         float64       // Read/write ratio to be considered write-heavy
	HighLatencyThreshold    time.Duration // Latency threshold for detecting performance issues
	MemoryPressureThreshold int64         // Memory usage threshold for detecting memory pressure
	MigrationCooldown       time.Duration // Cooldown period between migrations
}

// NewDefaultAnalyzer creates and initializes a new DefaultAnalyzer.
func NewDefaultAnalyzer() *DefaultAnalyzer {
	return &DefaultAnalyzer{
		thresholds: AnalysisThresholds{
			HighContentionRatio:     0.1,
			ReadHeavyRatio:          10.0,
			WriteHeavyRatio:         0.1,
			HighLatencyThreshold:    time.Millisecond,
			MemoryPressureThreshold: 100 * 1024 * 1024, // 100MB
			MigrationCooldown:       5 * time.Minute,
		},
		migrationHistory: make(map[string][]sync.MigrationEvent),
	}
}

// AnalyzeVariable performs a one-time analysis of a variable.
func (a *DefaultAnalyzer) AnalyzeVariable(v sync.Variable) sync.AnalysisResult {
	metrics := v.GetMetrics()
	currentStrategy := v.GetStorageStrategy()

	// Detect access pattern
	pattern := a.DetectAccessPattern(metrics)

	// Predict optimal strategy
	optimalStrategy := a.PredictOptimalStrategy(metrics)

	// Calculate confidence
	confidence := a.calculateConfidence(metrics, pattern)

	// Generate recommendations
	recommendations := a.generateRecommendations(v, metrics, pattern, optimalStrategy)

	return sync.AnalysisResult{
		Strategy:        optimalStrategy,
		Confidence:      confidence,
		Reasoning:       a.buildReasoning(metrics, pattern, currentStrategy, optimalStrategy),
		Metrics:         metrics,
		Recommendations: recommendations,
		Timestamp:       time.Now(),
	}
}

// MonitorVariable continuously monitors a variable and sends analysis results to a channel.
func (a *DefaultAnalyzer) MonitorVariable(ctx context.Context, v sync.Variable, interval time.Duration) <-chan sync.AnalysisResult {
	results := make(chan sync.AnalysisResult, 1)

	go func() {
		defer close(results)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				results <- a.AnalyzeVariable(v)
			}
		}
	}()

	return results
}

// DetectAccessPattern detects the access pattern of a variable based on its metrics.
func (a *DefaultAnalyzer) DetectAccessPattern(metrics sync.VariableMetrics) sync.AccessPattern {
	if metrics.ReadCount == 0 && metrics.WriteCount == 0 {
		return sync.PatternRareAccess
	}

	readWriteRatio := float64(metrics.ReadCount) / float64(metrics.WriteCount+1)

	switch {
	case readWriteRatio > a.thresholds.ReadHeavyRatio:
		return sync.PatternReadHeavy
	case readWriteRatio < a.thresholds.WriteHeavyRatio:
		return sync.PatternWriteHeavy
	case metrics.ContentionEvents > uint64(float64(metrics.ReadCount+metrics.WriteCount)*a.thresholds.HighContentionRatio):
		return sync.PatternBurst
	default:
		return sync.PatternMixed
	}
}

// PredictOptimalStrategy predicts the optimal storage strategy for a variable based on its metrics.
func (a *DefaultAnalyzer) PredictOptimalStrategy(metrics sync.VariableMetrics) sync.StorageStrategy {
	pattern := a.DetectAccessPattern(metrics)

	// High-level strategy selection
	switch pattern {
	case sync.PatternReadHeavy:
		if metrics.MaxConcurrentReaders > 10 {
			return sync.StrategyReadOptimized
		}
		return sync.StrategyAtomic

	case sync.PatternWriteHeavy:
		if metrics.MaxConcurrentWriters > 5 {
			return sync.StrategyWriteOptimized
		}
		return sync.StrategyMutex

	case sync.PatternBurst:
		return sync.StrategySharded

	case sync.PatternRareAccess:
		return sync.StrategyMemoryOptimized

	default:
		if metrics.MemoryUsage > a.thresholds.MemoryPressureThreshold {
			return sync.StrategyMemoryOptimized
		}
		return sync.StrategyAtomic
	}
}

// GetOptimizationSuggestions provides a list of optimization suggestions for a variable.
func (a *DefaultAnalyzer) GetOptimizationSuggestions(v sync.Variable) []sync.OptimizationSuggestion {
	metrics := v.GetMetrics()

	// Detect access pattern
	pattern := a.DetectAccessPattern(metrics)

	// Predict optimal strategy
	optimalStrategy := a.PredictOptimalStrategy(metrics)

	return a.generateRecommendations(v, metrics, pattern, optimalStrategy)
}

// ShouldMigrate determines if a variable should be migrated to a different strategy.
func (a *DefaultAnalyzer) ShouldMigrate(v sync.Variable) (bool, sync.StorageStrategy, string) {
	result := a.AnalyzeVariable(v)
	currentStrategy := v.GetStorageStrategy()

	if result.Strategy == currentStrategy {
		return false, currentStrategy, "Already using optimal strategy"
	}

	if result.Confidence < 0.7 {
		return false, currentStrategy, "Low confidence in recommendation"
	}

	// Check migration cooldown
	if a.isInMigrationCooldown(v) {
		return false, currentStrategy, "Migration cooldown active"
	}

	// Check if migration is beneficial
	if !a.isMigrationBeneficial(v, result) {
		return false, currentStrategy, "Migration not beneficial"
	}

	return true, result.Strategy, result.Reasoning
}

func (a *DefaultAnalyzer) calculateConfidence(metrics sync.VariableMetrics, pattern sync.AccessPattern) float64 {
	// Base confidence on sample size
	totalOps := metrics.ReadCount + metrics.WriteCount
	if totalOps < 1000 {
		return 0.3 // Low confidence with few operations
	}

	// Adjust based on pattern consistency
	confidence := 0.8

	// Higher confidence for clear patterns
	switch pattern {
	case sync.PatternReadHeavy, sync.PatternWriteHeavy:
		confidence = 0.9
	case sync.PatternBurst:
		if metrics.ContentionEvents > totalOps/10 {
			confidence = 0.95
		}
	case sync.PatternRareAccess:
		confidence = 0.6
	default:
	}

	return confidence
}

func (a *DefaultAnalyzer) generateRecommendations(v sync.Variable, metrics sync.VariableMetrics, _ sync.AccessPattern, optimalStrategy sync.StorageStrategy) []sync.OptimizationSuggestion {
	var suggestions []sync.OptimizationSuggestion

	currentStrategy := v.GetStorageStrategy()

	// Migration suggestion
	if optimalStrategy != currentStrategy {
		suggestions = append(suggestions, sync.OptimizationSuggestion{
			Type:        "migrate",
			Priority:    8,
			Description: fmt.Sprintf("Migrate from %d to %d for better performance", currentStrategy, optimalStrategy),
			Impact:      "high",
			Effort:      "medium",
			Strategy:    optimalStrategy,
		})
	}

	// Memory optimization
	if metrics.MemoryUsage > 50*1024*1024 { // 50MB
		suggestions = append(suggestions, sync.OptimizationSuggestion{
			Type:        "tune",
			Priority:    6,
			Description: "Consider reducing history size or enabling compression",
			Impact:      "medium",
			Effort:      "low",
		})
	}

	// Latency optimization
	if metrics.P99ReadLatency > time.Millisecond {
		suggestions = append(suggestions, sync.OptimizationSuggestion{
			Type:        "tune",
			Priority:    7,
			Description: "High read latency detected, consider read-optimized strategy",
			Impact:      "high",
			Effort:      "medium",
			Strategy:    sync.StrategyReadOptimized,
		})
	}

	return suggestions
}

func (a *DefaultAnalyzer) buildReasoning(metrics sync.VariableMetrics, pattern sync.AccessPattern, current, optimal sync.StorageStrategy) string {
	if current == optimal {
		return fmt.Sprintf("Current strategy %d is optimal for %d pattern", current, pattern)
	}

	return fmt.Sprintf("Pattern: %d, R/W ratio: %.2f, Contention: %d events, Current: %d → Recommended: %d",
		pattern, metrics.ReadWriteRatio, metrics.ContentionEvents, current, optimal)
}

func (a *DefaultAnalyzer) isInMigrationCooldown(v sync.Variable) bool {
	metrics := v.GetMetrics()
	if len(metrics.MigrationHistory) == 0 {
		return false
	}

	lastMigration := metrics.MigrationHistory[len(metrics.MigrationHistory)-1]
	return time.Since(lastMigration.Timestamp) < a.thresholds.MigrationCooldown
}

func (a *DefaultAnalyzer) isMigrationBeneficial(v sync.Variable, result sync.AnalysisResult) bool {
	// Simple heuristic: migrate if confidence is high and performance gain is expected
	return result.Confidence > 0.8 && len(result.Recommendations) > 0
}
