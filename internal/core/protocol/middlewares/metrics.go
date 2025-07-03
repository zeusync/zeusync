package middlewares

import (
	"context"
	"github.com/zeusync/zeusync/internal/core/observability/log"
	"github.com/zeusync/zeusync/internal/core/protocol/intrefaces"
	"sync"
	"time"
)

// MetricsMiddleware collects detailed metrics
type MetricsMiddleware struct {
	logger         log.Log
	messageMetrics sync.Map // message type -> *messageTypeMetrics
}

type messageTypeMetrics struct {
	count       int64
	totalTime   time.Duration
	errors      int64
	lastUpdated time.Time
	mu          sync.Mutex
}

// Name returns the middleware name
func (m *MetricsMiddleware) Name() string {
	return "metrics"
}

// Priority returns the middleware priority
func (m *MetricsMiddleware) Priority() uint16 {
	return 100 // Low priority, runs last
}

// BeforeHandle records start time
func (m *MetricsMiddleware) BeforeHandle(ctx context.Context, _ intrefaces.ClientInfo, _ intrefaces.Message) error {
	// Store start time in context
	ctx = context.WithValue(ctx, "start_time", time.Now())
	return nil
}

// AfterHandle records metrics
func (m *MetricsMiddleware) AfterHandle(ctx context.Context, _ intrefaces.ClientInfo, message intrefaces.Message, _ intrefaces.Message, err error) error {
	startTime, ok := ctx.Value("start_time").(time.Time)
	if !ok {
		return nil
	}

	duration := time.Since(startTime)
	metrics := m.getMessageMetrics(message.Type())

	metrics.mu.Lock()
	metrics.count++
	metrics.totalTime += duration
	metrics.lastUpdated = time.Now()
	if err != nil {
		metrics.errors++
	}
	metrics.mu.Unlock()

	return nil
}

// OnConnect records connection metrics
func (m *MetricsMiddleware) OnConnect(_ context.Context, _ intrefaces.ClientInfo) error {
	return nil
}

// OnDisconnect records disconnection metrics
func (m *MetricsMiddleware) OnDisconnect(_ context.Context, _ intrefaces.ClientInfo, _ string) error {
	return nil
}

// getMessageMetrics gets or creates metrics for a message type
func (m *MetricsMiddleware) getMessageMetrics(messageType string) *messageTypeMetrics {
	if metrics, exists := m.messageMetrics.Load(messageType); exists {
		return metrics.(*messageTypeMetrics)
	}

	metrics := &messageTypeMetrics{
		count:       0,
		totalTime:   0,
		errors:      0,
		lastUpdated: time.Now(),
	}
	m.messageMetrics.Store(messageType, metrics)
	return metrics
}

// GetMetrics returns collected metrics
func (m *MetricsMiddleware) GetMetrics() map[string]any {
	result := make(map[string]any)

	m.messageMetrics.Range(func(key, value any) bool {
		messageType := key.(string)
		metrics := value.(*messageTypeMetrics)

		metrics.mu.Lock()
		result[messageType] = map[string]any{
			"count":        metrics.count,
			"total_time":   metrics.totalTime,
			"average_time": metrics.totalTime / time.Duration(metrics.count),
			"errors":       metrics.errors,
			"last_updated": metrics.lastUpdated,
		}
		metrics.mu.Unlock()

		return true
	})

	return result
}
