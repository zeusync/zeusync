package protocol

import (
	"context"
	"errors"
	"github.com/zeusync/zeusync/internal/core/observability/log"
	"sync/atomic"
	"time"
)

// Protocol represents the main networking protocol interface
type Protocol interface {
	// Identity

	Name() string
	Version() string

	// Transport management

	RegisterTransport(transport Transport) error
	GetTransport(transportType TransportType) (Transport, bool)

	// Server operations

	Listen(ctx context.Context, transportType TransportType, addr string) (Server, error)
	ListenWithConfig(ctx context.Context, transportType TransportType, addr string, config ServerConfig) (Server, error)

	// Client operations

	Connect(ctx context.Context, transportType TransportType, addr string) (Client, error)
	ConnectWithConfig(ctx context.Context, transportType TransportType, addr string, config ClientConfig) (Client, error)

	// Health and metrics

	HealthCheck(ctx context.Context) HealthCheck
	Stats() Stats

	// Lifecycle

	Close() error
}

// Server represents a network server that accepts client connections
type Server interface {
	// Identity

	Addr() string
	Transport() TransportType

	// Client management

	AcceptClient(ctx context.Context) (Client, error)
	GetClient(id ClientID) (Client, bool)
	ListClients() []Client
	ClientCount() int
	DisconnectClient(id ClientID) error

	// Group management

	CreateGroup(name string) (Group, error)
	GetGroup(id GroupID) (Group, bool)
	DeleteGroup(id GroupID) error
	BroadcastToGroup(groupID GroupID, msg *Message) error

	// Scope management

	CreateScope(name string) (Scope, error)
	GetScope(id ScopeID) (Scope, bool)
	DeleteScope(id ScopeID) error

	// Broadcasting

	Broadcast(msg *Message, filter MessageFilter) error
	BroadcastAsync(msg *Message, filter MessageFilter) <-chan error

	// Event handling

	OnClientConnected(handler func(client Client))
	OnClientDisconnected(handler func(client Client))
	OnMessage(handler MessageHandler)
	OnStream(handler StreamHandler)

	// Health and metrics

	HealthCheck(ctx context.Context) HealthCheck
	Stats() ServerStats

	// Lifecycle

	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Close() error
}

// ServerConfig holds configuration for servers
type ServerConfig struct {
	// Connection settings
	MaxClients    int
	AcceptTimeout time.Duration
	ClientTimeout time.Duration
	KeepAlive     time.Duration

	// Resource limits
	MaxGroups      int
	MaxScopes      int
	MaxMemoryUsage uint64
	MaxBandwidth   uint64

	// Security settings
	EnableTLS   bool
	TLSConfig   interface{}
	AuthHandler func(ctx context.Context, credentials interface{}) (ClientID, error)

	// Performance settings
	WorkerPoolSize    int
	MessageBufferSize int
	EnableCompression bool
	CompressionLevel  int

	// Monitoring settings
	EnableMetrics       bool
	MetricsInterval     time.Duration
	HealthCheckInterval time.Duration

	// Custom settings
	CustomOptions map[string]interface{}
}

// DefaultServerConfig returns default server configuration
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		MaxClients:          10000,
		AcceptTimeout:       30 * time.Second,
		ClientTimeout:       5 * time.Minute,
		KeepAlive:           30 * time.Second,
		MaxGroups:           1000,
		MaxScopes:           500,
		MaxMemoryUsage:      1024 * 1024 * 1024, // 1GB
		MaxBandwidth:        100 * 1024 * 1024,  // 100MB/s
		EnableTLS:           true,
		WorkerPoolSize:      100,
		MessageBufferSize:   10000,
		EnableCompression:   true,
		CompressionLevel:    1,
		EnableMetrics:       true,
		MetricsInterval:     10 * time.Second,
		HealthCheckInterval: 30 * time.Second,
		CustomOptions:       make(map[string]interface{}),
	}
}

// ServerStats contains server-level statistics
type ServerStats struct {
	// Connection statistics
	ClientsConnected    uint64
	ClientsDisconnected uint64
	ClientsActive       uint64
	ConnectionsAccepted uint64
	ConnectionsRejected uint64

	// Message statistics
	MessagesSent      uint64
	MessagesReceived  uint64
	MessagesDropped   uint64
	MessagesBroadcast uint64

	// Stream statistics
	StreamsOpened uint64
	StreamsClosed uint64
	StreamsActive uint64

	// Group statistics
	GroupsCreated uint64
	GroupsDeleted uint64
	GroupsActive  uint64

	// Scope statistics
	ScopesCreated uint64
	ScopesDeleted uint64
	ScopesActive  uint64

	// Resource statistics
	MemoryUsage    uint64
	CPUUsage       float64
	BandwidthUsage uint64

	// Error statistics
	ErrorsEncountered uint64

	// Timing statistics
	Uptime          time.Duration
	LastHealthCheck time.Time
}

// Stats contains protocol-level statistics
type Stats struct {
	// Transport statistics
	TransportsRegistered uint64
	TransportsActive     uint64

	// Server statistics
	ServersCreated uint64
	ServersActive  uint64

	// Client statistics
	ClientsCreated uint64
	ClientsActive  uint64

	// Message statistics
	MessagesProcessed uint64
	MessagePoolHits   uint64
	MessagePoolMisses uint64

	// Performance statistics
	AverageLatency time.Duration
	ThroughputMBps float64

	// Error statistics
	ErrorsTotal uint64

	// Timing statistics
	Uptime time.Duration
}

// BaseProtocol provides a basic protocol implementation
type BaseProtocol struct {
	name              string
	version           string
	transportRegistry TransportRegistry
	servers           map[string]Server
	logger            log.Log
	stats             Stats
	startTime         time.Time
	closed            int32 // atomic bool
}

// NewBaseProtocol creates a new base protocol
func NewBaseProtocol(name, version string) *BaseProtocol {
	logger := log.Provide()
	return &BaseProtocol{
		name:              name,
		version:           version,
		transportRegistry: NewBaseTransportRegistry(logger),
		servers:           make(map[string]Server),
		logger:            logger,
		startTime:         time.Now(),
	}
}

// Name returns the protocol name
func (p *BaseProtocol) Name() string {
	return p.name
}

// Version returns the protocol version
func (p *BaseProtocol) Version() string {
	return p.version
}

// RegisterTransport registers a transport implementation
func (p *BaseProtocol) RegisterTransport(transport Transport) error {
	if atomic.LoadInt32(&p.closed) == 1 {
		return ErrProtocolClosed
	}

	atomic.AddUint64(&p.stats.TransportsRegistered, 1)
	return p.transportRegistry.RegisterTransport(transport)
}

// GetTransport gets transport by type
func (p *BaseProtocol) GetTransport(transportType TransportType) (Transport, bool) {
	return p.transportRegistry.GetTransport(transportType)
}

// Stats returns protocol statistics
func (p *BaseProtocol) Stats() Stats {
	stats := p.stats
	stats.Uptime = time.Since(p.startTime)
	stats.TransportsActive = uint64(len(p.transportRegistry.ListTransports()))
	stats.ServersActive = uint64(len(p.servers))
	return stats
}

// UpdateStats atomically updates protocol statistics
func (p *BaseProtocol) UpdateStats(field string, delta uint64) {
	switch field {
	case "transports_registered":
		atomic.AddUint64(&p.stats.TransportsRegistered, delta)
	case "transports_active":
		atomic.AddUint64(&p.stats.TransportsActive, delta)
	case "servers_created":
		atomic.AddUint64(&p.stats.ServersCreated, delta)
	case "servers_active":
		atomic.AddUint64(&p.stats.ServersActive, delta)
	case "clients_created":
		atomic.AddUint64(&p.stats.ClientsCreated, delta)
	case "clients_active":
		atomic.AddUint64(&p.stats.ClientsActive, delta)
	case "messages_processed":
		atomic.AddUint64(&p.stats.MessagesProcessed, delta)
	case "message_pool_hits":
		atomic.AddUint64(&p.stats.MessagePoolHits, delta)
	case "message_pool_misses":
		atomic.AddUint64(&p.stats.MessagePoolMisses, delta)
	case "errors_total":
		atomic.AddUint64(&p.stats.ErrorsTotal, delta)
	}
}

// HealthCheck performs a health check on the protocol
func (p *BaseProtocol) HealthCheck(ctx context.Context) HealthCheck {
	if atomic.LoadInt32(&p.closed) == 1 {
		return HealthCheck{
			Status:    HealthStatusUnhealthy,
			Timestamp: time.Now(),
			Error:     ErrProtocolClosed,
		}
	}

	// Check transport health
	transportHealth := p.transportRegistry.HealthCheck(ctx)
	for _, health := range transportHealth {
		if health.Status != HealthStatusHealthy {
			return HealthCheck{
				Status:    HealthStatusDegraded,
				Timestamp: time.Now(),
				Error:     health.Error,
			}
		}
	}

	return HealthCheck{
		Status:    HealthStatusHealthy,
		Timestamp: time.Now(),
	}
}

// Close closes the protocol and all associated resources
func (p *BaseProtocol) Close() error {
	atomic.StoreInt32(&p.closed, 1)

	// Close all servers
	for _, server := range p.servers {
		if err := server.Close(); err != nil {
			p.logger.Error("Failed to close server", log.Error(err))
		}
	}

	// Close transport registry
	return p.transportRegistry.Close()
}

// IsClosed checks if the protocol is closed
func (p *BaseProtocol) IsClosed() bool {
	return atomic.LoadInt32(&p.closed) == 1
}

// EventHandler represents a generic event handler
type EventHandler func(event any) error

// EventBus provides event publishing and subscription
type EventBus interface {
	// Subscription

	Subscribe(eventType string, handler EventHandler) error
	Unsubscribe(eventType string, handler EventHandler) error

	// Publishing

	Publish(eventType string, event any) error
	PublishAsync(eventType string, event any) <-chan error

	// Lifecycle

	Close() error
}

// Event types
const (
	EventClientConnected    = "client.connected"
	EventClientDisconnected = "client.disconnected"
	EventMessageReceived    = "message.received"
	EventMessageSent        = "message.sent"
	EventStreamOpened       = "stream.opened"
	EventStreamClosed       = "stream.closed"
	EventGroupCreated       = "group.created"
	EventGroupDeleted       = "group.deleted"
	EventScopeCreated       = "scope.created"
	EventScopeDeleted       = "scope.deleted"
	EventError              = "error"
)

// Additional protocol errors

var (
	ErrProtocolClosed = errors.New("protocol is closed")
)

// Global protocol instance

var DefaultProtocol = NewBaseProtocol("ZeuSync", "1.0.0")
