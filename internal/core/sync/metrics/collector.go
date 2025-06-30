package metrics

import (
	"context"
	sc "sync"
	"sync/atomic"
	"time"

	"github.com/zeusync/zeusync/internal/core/sync"
)

var _ sync.MetricsCollector = (*AsyncMetricsCollector)(nil)

// AsyncMetricsCollector collects metrics asynchronously to avoid blocking variable operations
type AsyncMetricsCollector struct {
	events    chan sync.MetricEvent
	metrics   sc.Map // varName -> *VariableMetrics
	ctx       context.Context
	cancel    context.CancelFunc
	batchSize int
}

func NewAsyncMetricsCollector(parent context.Context, bufferSize int) *AsyncMetricsCollector {
	ctx, cancel := context.WithCancel(parent)
	collector := &AsyncMetricsCollector{
		events:    make(chan sync.MetricEvent, bufferSize),
		ctx:       ctx,
		cancel:    cancel,
		batchSize: 100,
	}
	go collector.processEvents()
	return collector
}

func (c *AsyncMetricsCollector) RecordRead(varName string, latency time.Duration) {
	select {
	case c.events <- sync.MetricEvent{VarName: varName, Type: sync.MetricRead, Latency: latency, Timestamp: time.Now()}:
	default:
		// Drop event if buffer is full to avoid blocking
	}
}

func (c *AsyncMetricsCollector) RecordWrite(varName string, latency time.Duration) {
	select {
	case c.events <- sync.MetricEvent{VarName: varName, Type: sync.MetricWrite, Latency: latency, Timestamp: time.Now()}:
	default:
	}
}

func (c *AsyncMetricsCollector) RecordConflict(varName string) {
	select {
	case c.events <- sync.MetricEvent{VarName: varName, Type: sync.MetricConflict, Timestamp: time.Now()}:
	default:
	}
}

func (c *AsyncMetricsCollector) RecordError(varName string, err error) {
	select {
	case c.events <- sync.MetricEvent{VarName: varName, Type: sync.MetricError, Error: err, Timestamp: time.Now()}:
	default:
	}
}

func (c *AsyncMetricsCollector) GetMetrics(varName string) sync.VariableMetrics {
	if value, ok := c.metrics.Load(varName); ok {
		return *value.(*sync.VariableMetrics)
	}
	return sync.VariableMetrics{}
}

func (c *AsyncMetricsCollector) Close() error {
	c.cancel()
	close(c.events)
	return nil
}

func (c *AsyncMetricsCollector) processEvents() {
	batch := make([]sync.MetricEvent, 0, c.batchSize)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case event := <-c.events:
			batch = append(batch, event)
			if len(batch) >= c.batchSize {
				c.processBatch(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				c.processBatch(batch)
				batch = batch[:0]
			}
		}
	}
}

func (c *AsyncMetricsCollector) processBatch(batch []sync.MetricEvent) {
	for _, event := range batch {
		c.updateMetrics(event)
	}
}

func (c *AsyncMetricsCollector) updateMetrics(event sync.MetricEvent) {
	value, _ := c.metrics.LoadOrStore(event.VarName, &sync.VariableMetrics{
		CreatedAt: time.Now(),
	})
	metrics := value.(*sync.VariableMetrics)

	switch event.Type {
	case sync.MetricRead:
		atomic.AddUint64(&metrics.ReadCount, 1)
		c.updateLatency(&metrics.AvgReadLatency, event.Latency)
	case sync.MetricWrite:
		atomic.AddUint64(&metrics.WriteCount, 1)
		c.updateLatency(&metrics.AvgWriteLatency, event.Latency)
		metrics.LastModified = event.Timestamp
	case sync.MetricConflict:
		atomic.AddUint64(&metrics.ConflictCount, 1)
	case sync.MetricError:
		atomic.AddUint64(&metrics.ErrorCount, 1)
	}
	metrics.LastAccessed = event.Timestamp
}

func (c *AsyncMetricsCollector) updateLatency(avgLatency *time.Duration, newLatency time.Duration) {
	// Simple exponential moving average
	current := time.Duration(atomic.LoadInt64((*int64)(avgLatency)))
	newAvg := time.Duration(float64(current)*0.9 + float64(newLatency)*0.1)
	atomic.StoreInt64((*int64)(avgLatency), int64(newAvg))
}
