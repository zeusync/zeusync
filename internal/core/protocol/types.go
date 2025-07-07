package protocol

import (
	"github.com/google/uuid"
	"time"
)

// ConnectionID represents a unique identifier for a connection
type ConnectionID string

// ClientID represents a unique identifier for a client
type ClientID string

// GroupID represents a unique identifier for a client group
type GroupID string

// ScopeID represents a unique identifier for a data scope
type ScopeID string

// MessageID represents a unique identifier for a message
type MessageID string

// StreamID represents a unique identifier for a stream
type StreamID string

// ConnectionState represents the current state of a connection
type ConnectionState int

const (
	ConnectionStateConnecting ConnectionState = iota
	ConnectionStateConnected
	ConnectionStateDisconnecting
	ConnectionStateDisconnected
	ConnectionStateError
)

// MessageType defines the type of message being sent
type MessageType uint8

const (
	MessageTypeData MessageType = iota
	MessageTypeControl
	MessageTypeHeartbeat
	MessageTypeAck
	MessageTypeStream
	MessageTypeGroupcast
	MessageTypeScopecast
)

// MessageType string representation
func (mt MessageType) String() string {
	switch mt {
	case MessageTypeData:
		return "data"
	case MessageTypeControl:
		return "control"
	case MessageTypeHeartbeat:
		return "heartbeat"
	case MessageTypeAck:
		return "ack"
	case MessageTypeStream:
		return "stream"
	case MessageTypeGroupcast:
		return "groupcast"
	case MessageTypeScopecast:
		return "scopecast"
	default:
		return "unknown"
	}
}

// Priority defines message priority levels
type Priority uint8

const (
	PriorityLow Priority = iota
	PriorityNormal
	PriorityHigh
	PriorityCritical
)

// TransportType defines the underlying transport protocol
type TransportType string

const (
	TransportQUIC TransportType = "quic"
	TransportWS   TransportType = "websocket"
	TransportGRPC TransportType = "grpc"
	TransportTCP  TransportType = "tcp"
	TransportUDP  TransportType = "udp"
)

// ConnectionInfo contains metadata about a connection
type ConnectionInfo struct {
	ID            ConnectionID
	RemoteAddr    string
	LocalAddr     string
	Transport     TransportType
	ConnectedAt   time.Time
	LastActivity  time.Time
	BytesSent     uint64
	BytesReceived uint64
	State         ConnectionState
}

// ClientInfo contains metadata about a client
type ClientInfo struct {
	ID           ClientID
	ConnectionID ConnectionID
	Groups       []GroupID
	Scopes       []ScopeID
	ConnectedAt  time.Time
	LastSeen     time.Time
	Metadata     map[string]interface{}
}

// GroupInfo contains metadata about a client group
type GroupInfo struct {
	ID          GroupID
	Name        string
	ClientCount int
	CreatedAt   time.Time
	Metadata    map[string]interface{}
}

// ScopeInfo contains metadata about a data scope
type ScopeInfo struct {
	ID          ScopeID
	Name        string
	ClientCount int
	DataSize    uint64
	CreatedAt   time.Time
	Metadata    map[string]interface{}
}

// HealthStatus represents the health status of a component
type HealthStatus int

const (
	HealthStatusHealthy HealthStatus = iota
	HealthStatusDegraded
	HealthStatusUnhealthy
)

// HealthCheck represents a health check result
type HealthCheck struct {
	Status    HealthStatus
	Timestamp time.Time
	Latency   time.Duration
	Error     error
}

// UUID generation functions for optimized ID creation

// GenerateConnectionID generates a unique connection ID
func GenerateConnectionID() ConnectionID {
	return ConnectionID(generateUUID())
}

// GenerateClientID generates a unique client ID
func GenerateClientID() ClientID {
	return ClientID(generateUUID())
}

// GenerateGroupID generates a unique group ID
func GenerateGroupID() GroupID {
	return GroupID(generateUUID())
}

// GenerateScopeID generates a unique scope ID
func GenerateScopeID() ScopeID {
	return ScopeID(generateUUID())
}

// GenerateMessageID generates a unique message ID
func GenerateMessageID() MessageID {
	return MessageID(generateUUID())
}

// GenerateStreamID generates a unique stream ID
func GenerateStreamID() StreamID {
	return StreamID(generateUUID())
}

// generateUUID generates a UUID v4 string optimized for performance
func generateUUID() string {
	return uuid.NewString()
}
