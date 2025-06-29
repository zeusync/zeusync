package intrefaces

import (
	"context"
	"time"
)

// Protocol defines network communication interface
// Supports multiple protocols: HTTP, gRPC, WebSocket, TCP, UDP
type Protocol interface {
	// Identity

	Name() string
	Version() string
	Type() ProtocolType

	// Lifecycle

	Start(ctx context.Context, config ProtocolConfig) error
	Stop(ctx context.Context) error
	Restart(ctx context.Context) error
	IsRunning() bool

	// Message handling

	RegisterHandler(messageType string, handler MessageHandler) error
	UnregisterHandler(messageType string) error
	GetHandler(messageType string) (MessageHandler, bool)
	SetDefaultHandler(handler MessageHandler)

	// Client management

	Send(clientID string, message Message) error
	SendToMultiple(clientIDs []string, message Message) error
	Broadcast(message Message) error
	BroadcastExcept(excludeClientIDs []string, message Message) error

	// Connection management

	GetClient(clientID string) (ClientInfo, bool)
	GetAllClients() []ClientInfo
	DisconnectClient(clientID string, reason string) error
	GetConnectionCount() int

	// Groups/Rooms support

	CreateGroup(groupID string) error
	DeleteGroup(groupID string) error
	JoinGroup(clientID, groupID string) error
	LeaveGroup(clientID, groupID string) error
	SendToGroup(groupID string, message Message) error
	GetGroupMembers(groupID string) ([]string, error)

	// Middleware support

	AddMiddleware(middleware ProtocolMiddleware) error
	RemoveMiddleware(name string) error

	// Metrics and monitoring

	GetMetrics() ProtocolMetrics
	GetClientMetrics(clientID string) (ClientMetrics, bool)

	// Configuration

	GetConfig() ProtocolConfig
	UpdateConfig(config ProtocolConfig) error
}

// ProtocolMiddleware provides cross-cutting concerns
type ProtocolMiddleware interface {
	Name() string
	Priority() uint16

	// Request/Response cycle

	BeforeHandle(ctx context.Context, client ClientInfo, message Message) error
	AfterHandle(ctx context.Context, client ClientInfo, message Message, response Message, err error) error

	// Connection lifecycle

	OnConnect(ctx context.Context, client ClientInfo) error
	OnDisconnect(ctx context.Context, client ClientInfo, reason string) error
}

// ProtocolType defines the type of protocol
type ProtocolType uint8

const (
	ProtocolHTTP ProtocolType = iota
	ProtocolHTTPS
	ProtocolWebSocket
	ProtocolWebSocketSecure
	ProtocolTCP
	ProtocolUDP
	ProtocolGRPC
	ProtocolCustom
)

// ProtocolConfig holds protocol configuration
type ProtocolConfig struct {
	// Network settings
	Host             string
	Port             int
	MaxConnections   uint64
	ReadTimeout      time.Duration
	WriteTimeout     time.Duration
	KeepAliveTimeout time.Duration

	// Message settings
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

// ProtocolMetrics provides protocol performance metrics
type ProtocolMetrics struct {
	// Connection metrics
	ActiveConnections    uint64
	TotalConnections     int64
	FailedConnections    int64
	ConnectionsPerSecond float64

	// Message metrics
	MessagesSent       uint64
	MessagesReceived   uint64
	MessagesPerSecond  float64
	AverageMessageSize float64
}
