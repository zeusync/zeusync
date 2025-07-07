package protocol

import (
	"context"
	"github.com/zeusync/zeusync/internal/core/observability/log"
	"sync/atomic"
	"time"
)

// Transport represents an abstract network transport layer
type Transport interface {
	// Identity

	Type() TransportType
	Name() string

	// Listener operations

	Listen(ctx context.Context, addr string) (ConnectionListener, error)
	ListenWithConfig(ctx context.Context, addr string, config *GlobalConfig) (ConnectionListener, error)

	// Dialer operations

	Dial(ctx context.Context, addr string) (Connection, error)
	DialWithConfig(ctx context.Context, addr string, config *GlobalConfig) (Connection, error)

	// Configuration

	DefaultConfig() *GlobalConfig
	SupportedFeatures() TransportFeatures

	// Health and metrics

	HealthCheck(ctx context.Context) HealthCheck
	Stats() TransportStats

	// Lifecycle

	Close() error
}

// TransportFeatures describes the capabilities of a transport
type TransportFeatures struct {
	Reliable        bool
	Ordered         bool
	Multiplexing    bool
	Encryption      bool
	Compression     bool
	FlowControl     bool
	Bidirectional   bool
	LowLatency      bool
	HighThroughput  bool
	ConnectionReuse bool
	StreamSupport   bool
	MessageFraming  bool
}

// TransportStats contains transport-level statistics
type TransportStats struct {
	ConnectionsAccepted uint64
	ConnectionsDialed   uint64
	ConnectionsActive   uint64
	ConnectionsFailed   uint64
	BytesSent           uint64
	BytesReceived       uint64
	MessagesProcessed   uint64
	ErrorsEncountered   uint64
	Uptime              time.Duration
}

// GlobalConfig holds unified configuration for all protocol components
// This reduces memory usage by sharing configuration across clients
type GlobalConfig struct {
	// Connection settings
	MaxConnections    int
	ConnectionTimeout time.Duration
	KeepAlive         time.Duration
	IdleTimeout       time.Duration
	WriteTimeout      time.Duration
	ReadTimeout       time.Duration

	// Message settings
	MaxMessageSize    int
	MessageBufferSize int

	// Stream settings
	MaxStreams       int
	StreamBufferSize int

	// Group settings
	MaxGroups       int
	MaxGroupMembers int

	// Scope settings
	MaxScopes        int
	MaxScopeDataSize uint64
	MaxScopeKeys     int

	// Performance settings
	WorkerPoolSize    int
	EnableCompression bool
	CompressionLevel  int

	// Security settings
	EnableTLS bool
	TLSConfig interface{}

	// Monitoring settings
	EnableMetrics       bool
	MetricsInterval     time.Duration
	HealthCheckInterval time.Duration

	// Resource limits
	MaxMemoryUsage uint64
	MaxBandwidth   uint64

	// Custom settings
	CustomOptions map[string]interface{}
}

// DefaultGlobalConfig returns default global configuration
func DefaultGlobalConfig() *GlobalConfig {
	return &GlobalConfig{
		MaxConnections:      10000,
		ConnectionTimeout:   30 * time.Second,
		KeepAlive:           30 * time.Second,
		IdleTimeout:         5 * time.Minute,
		WriteTimeout:        10 * time.Second,
		ReadTimeout:         10 * time.Second,
		MaxMessageSize:      1024 * 1024, // 1MB
		MessageBufferSize:   1000,
		MaxStreams:          100,
		StreamBufferSize:    64 * 1024, // 64KB
		MaxGroups:           1000,
		MaxGroupMembers:     10000,
		MaxScopes:           500,
		MaxScopeDataSize:    100 * 1024 * 1024, // 100MB
		MaxScopeKeys:        10000,
		WorkerPoolSize:      100,
		EnableCompression:   true,
		CompressionLevel:    1,
		EnableTLS:           true,
		EnableMetrics:       true,
		MetricsInterval:     10 * time.Second,
		HealthCheckInterval: 30 * time.Second,
		MaxMemoryUsage:      1024 * 1024 * 1024, // 1GB
		MaxBandwidth:        100 * 1024 * 1024,  // 100MB/s
		CustomOptions:       make(map[string]interface{}),
	}
}

// BaseTransport provides common transport functionality
type BaseTransport struct {
	transportType TransportType
	name          string
	config        *GlobalConfig
	stats         TransportStats
	startTime     time.Time
	closed        int32 // atomic bool
	logger        log.Log
}

// NewBaseTransport creates a new base transport
func NewBaseTransport(transportType TransportType, name string, config *GlobalConfig, logger log.Log) *BaseTransport {
	if config == nil {
		config = DefaultGlobalConfig()
	}

	transport := &BaseTransport{
		transportType: transportType,
		name:          name,
		config:        config,
		startTime:     time.Now(),
		logger:        logger.With(log.String("transport", string(transportType)), log.String("name", name)),
	}

	transport.logger.Info("Transport created",
		log.String("type", string(transportType)),
		log.String("name", name))

	return transport
}

// Type returns the transport type
func (t *BaseTransport) Type() TransportType {
	return t.transportType
}

// Name returns the transport name
func (t *BaseTransport) Name() string {
	return t.name
}

// DefaultConfig returns the default configuration
func (t *BaseTransport) DefaultConfig() *GlobalConfig {
	return DefaultGlobalConfig()
}

// Stats returns transport statistics
func (t *BaseTransport) Stats() TransportStats {
	stats := t.stats
	stats.Uptime = time.Since(t.startTime)
	return stats
}

// UpdateStats atomically updates transport statistics
func (t *BaseTransport) UpdateStats(field string, delta uint64) {
	switch field {
	case "connections_accepted":
		atomic.AddUint64(&t.stats.ConnectionsAccepted, delta)
		t.logger.Debug("Connections accepted updated", log.Uint64("delta", delta))
	case "connections_dialed":
		atomic.AddUint64(&t.stats.ConnectionsDialed, delta)
		t.logger.Debug("Connections dialed updated", log.Uint64("delta", delta))
	case "connections_active":
		atomic.AddUint64(&t.stats.ConnectionsActive, delta)
		t.logger.Debug("Active connections updated", log.Uint64("delta", delta))
	case "connections_failed":
		atomic.AddUint64(&t.stats.ConnectionsFailed, delta)
		t.logger.Warn("Connection failed", log.Uint64("total_failures", atomic.LoadUint64(&t.stats.ConnectionsFailed)))
	case "bytes_sent":
		atomic.AddUint64(&t.stats.BytesSent, delta)
	case "bytes_received":
		atomic.AddUint64(&t.stats.BytesReceived, delta)
	case "messages_processed":
		atomic.AddUint64(&t.stats.MessagesProcessed, delta)
	case "errors_encountered":
		atomic.AddUint64(&t.stats.ErrorsEncountered, delta)
		t.logger.Error("Transport error encountered", log.Uint64("total_errors", atomic.LoadUint64(&t.stats.ErrorsEncountered)))
	}
}

// Close closes the transport
func (t *BaseTransport) Close() error {
	if !atomic.CompareAndSwapInt32(&t.closed, 0, 1) {
		return nil
	}

	t.logger.Info("Transport closing",
		log.Duration("uptime", time.Since(t.startTime)),
		log.Uint64("connections_accepted", atomic.LoadUint64(&t.stats.ConnectionsAccepted)),
		log.Uint64("connections_dialed", atomic.LoadUint64(&t.stats.ConnectionsDialed)),
		log.Uint64("messages_processed", atomic.LoadUint64(&t.stats.MessagesProcessed)))

	return nil
}

// IsClosed checks if the transport is closed
func (t *BaseTransport) IsClosed() bool {
	return atomic.LoadInt32(&t.closed) == 1
}

// Logger returns the transport logger
func (t *BaseTransport) Logger() log.Log {
	return t.logger
}

// Config returns the transport configuration
func (t *BaseTransport) Config() *GlobalConfig {
	return t.config
}

// TransportRegistry manages available transport implementations
type TransportRegistry interface {
	// Registration

	RegisterTransport(transport Transport) error
	UnregisterTransport(transportType TransportType) error

	// Queries

	GetTransport(transportType TransportType) (Transport, bool)
	ListTransports() []Transport
	SupportedTypes() []TransportType

	// Factory methods

	CreateListener(transportType TransportType, addr string, config *GlobalConfig) (ConnectionListener, error)
	CreateDialer(transportType TransportType, config *GlobalConfig) (ConnectionDialer, error)

	// Health monitoring

	HealthCheck(ctx context.Context) map[TransportType]HealthCheck

	// Lifecycle

	Close() error
}

// BaseTransportRegistry provides a basic transport registry implementation
type BaseTransportRegistry struct {
	transports map[TransportType]Transport
	closed     int32 // atomic bool
	logger     log.Log
}

// NewBaseTransportRegistry creates a new transport registry
func NewBaseTransportRegistry(logger log.Log) *BaseTransportRegistry {
	return &BaseTransportRegistry{
		transports: make(map[TransportType]Transport),
		logger:     logger.With(log.String("component", "transport_registry")),
	}
}

// RegisterTransport registers a transport implementation
func (r *BaseTransportRegistry) RegisterTransport(transport Transport) error {
	if atomic.LoadInt32(&r.closed) == 1 {
		return ErrRegistryClosed
	}

	r.transports[transport.Type()] = transport
	r.logger.Info("Transport registered",
		log.String("type", string(transport.Type())),
		log.String("name", transport.Name()))

	return nil
}

// UnregisterTransport unregisters a transport implementation
func (r *BaseTransportRegistry) UnregisterTransport(transportType TransportType) error {
	if transport, exists := r.transports[transportType]; exists {
		delete(r.transports, transportType)
		if err := transport.Close(); err != nil {
			r.logger.Error("Transport close failed", log.String("type", string(transportType)), log.Error(err))
		}
		r.logger.Info("Transport unregistered", log.String("type", string(transportType)))
	}
	return nil
}

// GetTransport gets transport by type
func (r *BaseTransportRegistry) GetTransport(transportType TransportType) (Transport, bool) {
	transport, exists := r.transports[transportType]
	return transport, exists
}

// ListTransports returns all registered transports
func (r *BaseTransportRegistry) ListTransports() []Transport {
	transports := make([]Transport, 0, len(r.transports))
	for _, transport := range r.transports {
		transports = append(transports, transport)
	}
	return transports
}

// SupportedTypes returns all supported transport types
func (r *BaseTransportRegistry) SupportedTypes() []TransportType {
	types := make([]TransportType, 0, len(r.transports))
	for transportType := range r.transports {
		types = append(types, transportType)
	}
	return types
}

// CreateListener creates a listener for the specified transport type
func (r *BaseTransportRegistry) CreateListener(transportType TransportType, addr string, config *GlobalConfig) (ConnectionListener, error) {
	transport, exists := r.GetTransport(transportType)
	if !exists {
		r.logger.Error("Transport not supported", log.String("type", string(transportType)))
		return nil, ErrTransportNotSupported
	}

	return transport.ListenWithConfig(context.Background(), addr, config)
}

// CreateDialer creates a dialer for the specified transport type
func (r *BaseTransportRegistry) CreateDialer(transportType TransportType, config *GlobalConfig) (ConnectionDialer, error) {
	transport, exists := r.GetTransport(transportType)
	if !exists {
		r.logger.Error("Transport not supported", log.String("type", string(transportType)))
		return nil, ErrTransportNotSupported
	}

	// Return a simple dialer wrapper
	return &transportDialer{transport: transport, config: config, logger: r.logger}, nil
}

// HealthCheck performs health checks on all registered transports
func (r *BaseTransportRegistry) HealthCheck(ctx context.Context) map[TransportType]HealthCheck {
	results := make(map[TransportType]HealthCheck)

	for transportType, transport := range r.transports {
		health := transport.HealthCheck(ctx)
		results[transportType] = health

		if health.Status != HealthStatusHealthy {
			r.logger.Warn("Transport health check failed",
				log.String("type", string(transportType)),
				log.String("status", health.Status.String()),
				log.Error(health.Error))
		}
	}

	return results
}

// Close closes the registry and all registered transports
func (r *BaseTransportRegistry) Close() error {
	if !atomic.CompareAndSwapInt32(&r.closed, 0, 1) {
		return nil
	}

	r.logger.Info("Closing transport registry")

	for transportType, transport := range r.transports {
		r.logger.Debug("Closing transport", log.String("type", string(transportType)))
		if err := transport.Close(); err != nil {
			r.logger.Error("Transport close failed", log.String("type", string(transportType)), log.Error(err))
		}
	}

	return nil
}

// transportDialer is a simple wrapper that implements ConnectionDialer
type transportDialer struct {
	transport Transport
	config    *GlobalConfig
	logger    log.Log
}

func (d *transportDialer) Dial(ctx context.Context, addr string) (Connection, error) {
	d.logger.Debug("Dialing connection", log.String("addr", addr))
	return d.transport.DialWithConfig(ctx, addr, d.config)
}

func (d *transportDialer) DialWithConfig(ctx context.Context, addr string, config *GlobalConfig) (Connection, error) {
	d.logger.Debug("Dialing connection with config", log.String("addr", addr))
	return d.transport.DialWithConfig(ctx, addr, config)
}

func (d *transportDialer) Transport() TransportType {
	return d.transport.Type()
}

func (d *transportDialer) Close() error {
	return nil // Transport is managed by registry
}

// TransportFactory creates transport instances
type TransportFactory interface {
	// Factory methods

	CreateTransport(transportType TransportType, config *GlobalConfig, logger log.Log) (Transport, error)
	SupportedTypes() []TransportType

	// Configuration

	DefaultConfig(transportType TransportType) *GlobalConfig
	ValidateConfig(transportType TransportType, config *GlobalConfig) error
}

// Global transport registry

var DefaultTransportRegistry = NewBaseTransportRegistry(log.Provide())

// RegisterTransport registers transport in the default registry
func RegisterTransport(transport Transport) error {
	return DefaultTransportRegistry.RegisterTransport(transport)
}

// GetTransport gets transport from the default registry
func GetTransport(transportType TransportType) (Transport, bool) {
	return DefaultTransportRegistry.GetTransport(transportType)
}

// HealthStatus string representation
func (h HealthStatus) String() string {
	switch h {
	case HealthStatusHealthy:
		return "healthy"
	case HealthStatusDegraded:
		return "degraded"
	case HealthStatusUnhealthy:
		return "unhealthy"
	default:
		return "unknown"
	}
}
