package protocol

import "time"

// Config holds protocol configuration
type Config struct {
	// Network settings
	Host             string
	Port             int
	MaxConnections   uint64
	ReadTimeout      time.Duration
	WriteTimeout     time.Duration
	KeepAliveTimeout time.Duration

	// IMessage settings
	MaxMessageSize    uint32
	CompressionLevel  uint8
	EnableCompression bool

	// Security settings
	TLSEnabled  bool
	CertFile    string
	KeyFile     string
	RequireAuth bool

	// Performance tuning
	BufferSize       uint32
	WorkerCount      uint32
	QueueSize        uint32
	EnablePipelining bool

	// Features
	EnableGroups     bool
	EnableMiddleware bool
	EnableMetrics    bool
	EnableTracing    bool

	// Custom options
	Options map[string]any
}

// Metrics provides protocol performance metrics
type Metrics struct {
	// Connection metrics
	ActiveConnections    uint64
	TotalConnections     int64
	FailedConnections    int64
	ConnectionsPerSecond float64

	// IMessage metrics
	MessagesSent       uint64
	MessagesReceived   uint64
	MessagesPerSecond  float64
	AverageMessageSize float64
}
