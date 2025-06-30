package sync

import "time"

// MetricsCollector provides lightweight metrics collection without affecting variable performance
type MetricsCollector interface {
	RecordRead(varName string, latency time.Duration)
	RecordWrite(varName string, latency time.Duration)
	RecordConflict(varName string)
	RecordError(varName string, err error)
	GetMetrics(varName string) VariableMetrics
	Close() error
}

type MetricEvent struct {
	VarName   string
	Type      MetricType
	Latency   time.Duration
	Timestamp time.Time
	Error     error
}

type MetricType uint8

const (
	MetricRead MetricType = iota
	MetricWrite
	MetricConflict
	MetricError
)
