package metrics

import (
	"time"

	"github.com/zeusync/zeusync/internal/core/syncv2/types"
)

// Metrics provides basic statistics about variables usage
type Metrics struct {
	TotalVariables   uint64
	TotalOperations  uint64
	AverageLatency   float32
	MemoryUsage      uint64
	OptimizationHits int64
}

// VariableMetrics contains performance and usage metrics for specific variable.
type VariableMetrics struct {
	Reads     uint64 // Number of read operations
	Writes    uint64 // Number of write operations
	Conflicts uint64 // Number of conflicts
	Errors    uint64 // Number of errors

	ReadLatency  int64 // Read latency in microseconds
	WriteLatency int64 // Write latency in microseconds

	DeltasSent     uint64 // Number of deltas sent
	DeltasReceived uint64 // Number of deltas received
	SyncLatency    int64  // Sync latency in microseconds

	MemUsage int64 // Memory usage in bytes

	Pattern types.AccessPattern // Access pattern detected by the analysis
}

// GlobalMetrics contains metrics for the entire variable manager.
type GlobalMetrics struct {
	TotalVariables       int                           // Total number of variables
	MemoryUsage          int64                         // Total memory usage
	TotalOperations      uint64                        // Total number of operations
	AverageLatency       time.Duration                 // Average latency of operations
	StrategyDistribution map[types.StorageStrategy]int // Distribution of storage strategies
	MigrationCount       uint64                        // Total number of migrations
	OptimizationSavings  float64                       // Estimated savings from optimizations
}
