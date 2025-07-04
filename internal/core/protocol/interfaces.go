package protocol

import (
	"context"
	"net"
	"time"
)

// Protocol - основной интерфейс для всех сетевых протоколов
// Универсальный и расширяемый для любых типов игр
type Protocol interface {
	// Lifecycle
	Start(ctx context.Context, config Config) error
	Stop(ctx context.Context) error
	IsRunning() bool

	// Client management
	OnClientConnect(handler ClientConnectHandler)
	OnClientDisconnect(handler ClientDisconnectHandler)
	OnMessage(handler MessageHandler)

	// Messaging
	SendToClient(clientID string, message Message) error
	SendToClients(clientIDs []string, message Message) error
	Broadcast(message Message) error
	BroadcastExcept(excludeClientIDs []string, message Message) error

	// Groups/Rooms - важно для игр
	CreateGroup(groupID string) error
	DeleteGroup(groupID string) error
	AddClientToGroup(clientID, groupID string) error
	RemoveClientFromGroup(clientID, groupID string) error
	SendToGroup(groupID string, message Message) error
	GetGroupClients(groupID string) []string

	// Metrics and monitoring
	GetMetrics() Metrics
	GetClients() []ClientInfo

	// Configuration
	GetConfig() Config
}

// Transport - абстракция транспортного уровня
type Transport interface {
	// Network operations
	Listen(address string) error
	Accept() (Connection, error)
	Dial(address string) (Connection, error)
	Close() error

	// Properties
	Type() TransportType
	LocalAddr() net.Addr
	IsListening() bool
}

// Connection - абстракция соединения
type Connection interface {
	// Identity
	ID() string
	RemoteAddr() net.Addr
	LocalAddr() net.Addr

	// Data transfer
	Send(data []byte) error
	Receive() ([]byte, error)
	SendMessage(message Message) error
	ReceiveMessage() (Message, error)

	// State
	IsAlive() bool
	IsClosed() bool
	LastActivity() time.Time
	Close() error

	// Configuration
	SetReadTimeout(timeout time.Duration)
	SetWriteTimeout(timeout time.Duration)
	SetKeepAlive(enabled bool)

	// Metadata
	GetMetadata(key string) (interface{}, bool)
	SetMetadata(key string, value interface{})
}

// Message - универсальный и��терфейс сообщения
type Message interface {
	// Core properties
	ID() string
	Type() string
	Payload() []byte
	Timestamp() time.Time

	// Headers for routing and metadata
	GetHeader(key string) string
	SetHeader(key, value string)
	Headers() map[string]string

	// Serialization
	Marshal() ([]byte, error)
	Unmarshal(data []byte) error

	// Utility
	Size() int
	Clone() Message
}

// Serializer - интерфейс для сериализации сообщений
type Serializer interface {
	Serialize(message Message) ([]byte, error)
	Deserialize(data []byte) (Message, error)
	ContentType() string
}

// Middleware - для расширяемости и плагинов
type Middleware interface {
	Name() string
	Priority() int
	OnMessage(ctx context.Context, client ClientInfo, message Message, next func() error) error
	OnConnect(ctx context.Context, client ClientInfo) error
	OnDisconnect(ctx context.Context, client ClientInfo, reason string) error
}

// Handler types
type ClientConnectHandler func(ctx context.Context, client ClientInfo) error
type ClientDisconnectHandler func(ctx context.Context, client ClientInfo, reason string) error
type MessageHandler func(ctx context.Context, client ClientInfo, message Message) error

// TransportType - типы транспорта
type TransportType int

const (
	TransportTCP TransportType = iota
	TransportUDP
	TransportQUIC
	TransportWebSocket
	TransportWebSocketSecure
)

func (t TransportType) String() string {
	switch t {
	case TransportTCP:
		return "TCP"
	case TransportUDP:
		return "UDP"
	case TransportQUIC:
		return "QUIC"
	case TransportWebSocket:
		return "WebSocket"
	case TransportWebSocketSecure:
		return "WebSocketSecure"
	default:
		return "Unknown"
	}
}

// ClientInfo - информация о клиенте
type ClientInfo struct {
	ID            string
	RemoteAddress string
	ConnectedAt   time.Time
	LastActivity  time.Time
	Transport     TransportType
	Groups        []string
	Metadata      map[string]interface{}
}

// Config - конфигурация протокола
type Config struct {
	// Network
	Host string
	Port int

	// Timeouts
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	IdleTimeout    time.Duration
	ConnectTimeout time.Duration

	// Limits
	MaxConnections int
	MaxMessageSize int
	MaxGroupSize   int
	BufferSize     int
	MaxHeaderSize  int

	// Performance
	WorkerCount       int
	QueueSize         int
	EnableCompression bool
	CompressionLevel  int

	// Security
	TLSEnabled bool
	CertFile   string
	KeyFile    string

	// Features
	EnableMetrics     bool
	EnableHeartbeat   bool
	HeartbeatInterval time.Duration

	// Serialization
	SerializerType string // "binary", "json", "msgpack", etc.

	// Custom options for extensibility
	Options map[string]interface{}
}

// Metrics - метрики производительности
type Metrics struct {
	// Connections
	ActiveConnections int64
	TotalConnections  int64
	FailedConnections int64

	// Messages
	MessagesSent     int64
	MessagesReceived int64
	MessagesDropped  int64
	MessagesQueued   int64

	// Performance
	MessagesPerSecond  float64
	AverageLatency     time.Duration
	AverageMessageSize float64
	BytesSent          int64
	BytesReceived      int64

	// Groups
	ActiveGroups int64
	TotalGroups  int64

	// Errors
	Errors map[string]int64

	// Custom metrics for extensibility
	Custom map[string]interface{}
}

// MessageType - предопределенные типы сообщений для удобства
const (
	// System messages
	MessageTypeHeartbeat  = "heartbeat"
	MessageTypeAuth       = "auth"
	MessageTypeDisconnect = "disconnect"
	MessageTypeError      = "error"

	// Group management
	MessageTypeJoinGroup   = "join_group"
	MessageTypeLeaveGroup  = "leave_group"
	MessageTypeGroupUpdate = "group_update"

	// Game messages (examples)
	MessageTypePlayerMove   = "player_move"
	MessageTypePlayerAction = "player_action"
	MessageTypeGameState    = "game_state"
	MessageTypeChat         = "chat"

	// Custom messages start from this prefix
	MessageTypeCustomPrefix = "custom_"
)

// Priority levels for message handling
type Priority int

const (
	PriorityLow Priority = iota
	PriorityNormal
	PriorityHigh
	PriorityCritical
)

// QoS levels for message delivery
type QoS int

const (
	QoSBestEffort QoS = iota // Fire and forget
	QoSReliable              // At least once delivery
	QoSOrdered               // Ordered delivery
)

// Error types
type ProtocolError struct {
	Code    string
	Message string
	Cause   error
}

func (e *ProtocolError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

// Common error codes
const (
	ErrorCodeInvalidMessage    = "INVALID_MESSAGE"
	ErrorCodeClientNotFound    = "CLIENT_NOT_FOUND"
	ErrorCodeGroupNotFound     = "GROUP_NOT_FOUND"
	ErrorCodeMessageTooLarge   = "MESSAGE_TOO_LARGE"
	ErrorCodeRateLimitExceeded = "RATE_LIMIT_EXCEEDED"
	ErrorCodeUnauthorized      = "UNAUTHORIZED"
	ErrorCodeInternalError     = "INTERNAL_ERROR"
)
