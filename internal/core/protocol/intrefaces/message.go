package intrefaces

import (
	"context"
	"time"
)

// Message represents a protocol message
type Message interface {
	// Content

	Type() string
	Data() []byte
	Payload() any
	SetPayload(any) error

	// Metadata

	ID() string
	Timestamp() time.Time
	Source() string
	Target() string
	Headers() map[string]string
	SetHeader(key, value string)
	GetHeader(key string) string

	// Routing

	Route() string
	SetRoute(string)
	IsResponse() bool
	ResponseTo() string
	CreateResponse(payload any) Message

	// Serialization

	Marshal() ([]byte, error)
	Unmarshal([]byte) error
	Clone() Message

	// Size and limits

	Size() int
	MaxSize() int
	Compress() error
	Decompress() error

	// Priority and QoS

	Priority() MessagePriority
	SetPriority(MessagePriority)
	QoS() QualityOfService
	SetQoS(QualityOfService)
}

// MessageHandler processes incoming messages
type MessageHandler func(ctx context.Context, client ClientInfo, message Message) error

// MessagePriority defines message priority levels
type MessagePriority uint8

const (
	PriorityLow MessagePriority = iota
	PriorityNormal
	PriorityHigh
	PriorityCritical
)

// QualityOfService defines delivery guarantees
type QualityOfService int

const (
	QoSAtMostOnce  QualityOfService = iota // Fire and forget
	QoSAtLeastOnce                         // Acknowledged delivery
	QoSExactlyOnce                         // Assured delivery
)
