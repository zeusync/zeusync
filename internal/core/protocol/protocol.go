package protocol

import (
	"context"
)

// Protocol defines network communication interface
// Supports multiple protocols: HTTP, gRPC, WebSocket, TCP, UDP
type Protocol interface {
	// Identity

	Name() string
	Version() string
	Type() Type

	// Lifecycle

	Start(ctx context.Context, config Config) error
	Stop(ctx context.Context) error
	Restart(ctx context.Context) error
	IsRunning() bool

	// IMessage handling

	RegisterHandler(messageType string, handler MessageHandler) error
	UnregisterHandler(messageType string) error
	GetHandler(messageType string) (MessageHandler, bool)
	SetDefaultHandler(handler MessageHandler)

	// Client management

	Send(clientID string, message IMessage) error
	SendToMultiple(clientIDs []string, message IMessage) error
	Broadcast(message IMessage) error
	BroadcastExcept(excludeClientIDs []string, message IMessage) error

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
	SendToGroup(groupID string, message IMessage) error
	GetGroupMembers(groupID string) ([]string, error)

	// Middleware support

	AddMiddleware(middleware Middleware) error
	RemoveMiddleware(name string) error

	// Metrics and monitoring

	GetMetrics() Metrics
	GetClientMetrics(clientID string) (ClientMetrics, bool)

	// Configuration

	GetConfig() Config
	UpdateConfig(config Config) error
}

// Middleware provides cross-cutting concerns
type Middleware interface {
	Name() string
	Priority() uint16

	// Request/Response cycle

	BeforeHandle(ctx context.Context, client ClientInfo, message IMessage) error
	AfterHandle(ctx context.Context, client ClientInfo, message IMessage, response IMessage, err error) error

	// Connection lifecycle

	OnConnect(ctx context.Context, client ClientInfo) error
	OnDisconnect(ctx context.Context, client ClientInfo, reason string) error
}

// Type defines the type of protocol
type Type uint8

const (
	HTTP Type = iota
	HTTPS
	WebSocket
	WebSocketSecure
	TCP
	UDP
	GRPC
	Custom
)
