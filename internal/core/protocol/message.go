package protocol

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/zeusync/zeusync/internal/core/observability/log"
)

// Serializable represents an object that can be serialized to bytes
type Serializable interface {
	Serialize() ([]byte, error)
	Deserialize(data []byte) error
}

// Message represents a network message with binary payload
type Message struct {
	ID        MessageID
	Type      MessageType
	Priority  Priority
	Payload   []byte
	Timestamp time.Time
	TTL       time.Duration

	// Routing information
	SourceClientID ClientID
	TargetClientID ClientID
	GroupID        GroupID
	ScopeID        ScopeID

	// Internal fields for pool management
	pooled   bool
	refCount int32
}

// MessageBuilder provides a fluent interface for building messages
type MessageBuilder struct {
	msg    *Message
	logger log.Log
}

// NewMessage creates a new message with the given parameters
func NewMessage(msgType MessageType, payload []byte) *Message {
	return &Message{
		ID:        GenerateMessageID(),
		Type:      msgType,
		Priority:  PriorityNormal,
		Payload:   payload,
		Timestamp: time.Now(),
		refCount:  1,
	}
}

// NewMessageFromBytes creates a message from raw bytes
func NewMessageFromBytes(msgType MessageType, data []byte) *Message {
	payload := make([]byte, len(data))
	copy(payload, data)

	return &Message{
		ID:        GenerateMessageID(),
		Type:      msgType,
		Priority:  PriorityNormal,
		Payload:   payload,
		Timestamp: time.Now(),
		refCount:  1,
	}
}

// NewMessageFromString creates a message from a string
func NewMessageFromString(msgType MessageType, text string) *Message {
	return &Message{
		ID:        GenerateMessageID(),
		Type:      msgType,
		Priority:  PriorityNormal,
		Payload:   []byte(text),
		Timestamp: time.Now(),
		refCount:  1,
	}
}

// NewMessageFromJSON creates a message from a JSON-serializable object
func NewMessageFromJSON(msgType MessageType, obj interface{}) (*Message, error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}

	return &Message{
		ID:        GenerateMessageID(),
		Type:      msgType,
		Priority:  PriorityNormal,
		Payload:   data,
		Timestamp: time.Now(),
		refCount:  1,
	}, nil
}

// NewMessageFromSerializable creates a message from a Serializable object
func NewMessageFromSerializable(msgType MessageType, obj Serializable) (*Message, error) {
	data, err := obj.Serialize()
	if err != nil {
		return nil, err
	}

	return &Message{
		ID:        GenerateMessageID(),
		Type:      msgType,
		Priority:  PriorityNormal,
		Payload:   data,
		Timestamp: time.Now(),
		refCount:  1,
	}, nil
}

// NewMessageBuilder creates a new message builder
func NewMessageBuilder(logger log.Log) *MessageBuilder {
	return &MessageBuilder{
		msg:    GetMessage(),
		logger: logger,
	}
}

// WithType sets the message type
func (b *MessageBuilder) WithType(msgType MessageType) *MessageBuilder {
	b.msg.Type = msgType
	return b
}

// WithPriority sets the message priority
func (b *MessageBuilder) WithPriority(priority Priority) *MessageBuilder {
	b.msg.Priority = priority
	return b
}

// WithPayload sets the message payload from bytes
func (b *MessageBuilder) WithPayload(payload []byte) *MessageBuilder {
	b.msg.Payload = make([]byte, len(payload))
	copy(b.msg.Payload, payload)
	return b
}

// WithString sets the message payload from string
func (b *MessageBuilder) WithString(text string) *MessageBuilder {
	b.msg.Payload = []byte(text)
	return b
}

// WithJSON sets the message payload from JSON object
func (b *MessageBuilder) WithJSON(obj interface{}) *MessageBuilder {
	data, err := json.Marshal(obj)
	if err != nil {
		b.logger.Error("Failed to marshal JSON payload", log.Error(err))
		return b
	}
	b.msg.Payload = data
	return b
}

// WithSerializable sets the message payload from Serializable object
func (b *MessageBuilder) WithSerializable(obj Serializable) *MessageBuilder {
	data, err := obj.Serialize()
	if err != nil {
		b.logger.Error("Failed to serialize payload", log.Error(err))
		return b
	}
	b.msg.Payload = data
	return b
}

// WithTTL sets the message TTL
func (b *MessageBuilder) WithTTL(ttl time.Duration) *MessageBuilder {
	b.msg.TTL = ttl
	return b
}

// WithSource sets the source client ID
func (b *MessageBuilder) WithSource(clientID ClientID) *MessageBuilder {
	b.msg.SourceClientID = clientID
	return b
}

// WithTarget sets the target client ID
func (b *MessageBuilder) WithTarget(clientID ClientID) *MessageBuilder {
	b.msg.TargetClientID = clientID
	return b
}

// WithGroup sets the group ID
func (b *MessageBuilder) WithGroup(groupID GroupID) *MessageBuilder {
	b.msg.GroupID = groupID
	return b
}

// WithScope sets the scope ID
func (b *MessageBuilder) WithScope(scopeID ScopeID) *MessageBuilder {
	b.msg.ScopeID = scopeID
	return b
}

// Build returns the constructed message
func (b *MessageBuilder) Build() *Message {
	msg := b.msg
	b.msg = nil // Prevent reuse
	return msg
}

// Clone creates a deep copy of the message
func (m *Message) Clone() *Message {
	payload := make([]byte, len(m.Payload))
	copy(payload, m.Payload)

	return &Message{
		ID:             GenerateMessageID(),
		Type:           m.Type,
		Priority:       m.Priority,
		Payload:        payload,
		Timestamp:      m.Timestamp,
		TTL:            m.TTL,
		SourceClientID: m.SourceClientID,
		TargetClientID: m.TargetClientID,
		GroupID:        m.GroupID,
		ScopeID:        m.ScopeID,
		refCount:       1,
	}
}

// Retain increments the reference count
func (m *Message) Retain() {
	atomic.AddInt32(&m.refCount, 1)
}

// Release decrements the reference count and returns the message to pool if needed
func (m *Message) Release() {
	if atomic.AddInt32(&m.refCount, -1) == 0 && m.pooled {
		defaultMessagePool.Put(m)
	}
}

// IsExpired checks if the message has exceeded its TTL
func (m *Message) IsExpired() bool {
	if m.TTL == 0 {
		return false
	}
	return time.Since(m.Timestamp) > m.TTL
}

// Size returns the total size of the message in bytes
func (m *Message) Size() int {
	return int(unsafe.Sizeof(*m)) + len(m.Payload)
}

// AsString returns the payload as a string
func (m *Message) AsString() string {
	return string(m.Payload)
}

// AsJSON unmarshal the payload into the provided object
func (m *Message) AsJSON(obj interface{}) error {
	return json.Unmarshal(m.Payload, obj)
}

// AsSerializable deserializes the payload into a Serializable object
func (m *Message) AsSerializable(obj Serializable) error {
	return obj.Deserialize(m.Payload)
}

// MessagePool provides lock-free message pooling for high performance
type MessagePool struct {
	pool   sync.Pool
	gets   uint64
	puts   uint64
	logger log.Log
}

// NewMessagePool creates a new message pool
func NewMessagePool(logger log.Log) *MessagePool {
	return &MessagePool{
		pool: sync.Pool{
			New: func() any {
				return &Message{}
			},
		},
		logger: logger,
	}
}

// Get retrieves a message from the pool
func (p *MessagePool) Get() *Message {
	atomic.AddUint64(&p.gets, 1)
	msg := p.pool.Get().(*Message)

	// Reset message fields
	msg.ID = GenerateMessageID()
	msg.Type = MessageTypeData
	msg.Priority = PriorityNormal
	msg.Payload = msg.Payload[:0] // Keep underlying array
	msg.Timestamp = time.Now()
	msg.TTL = 0
	msg.SourceClientID = ""
	msg.TargetClientID = ""
	msg.GroupID = ""
	msg.ScopeID = ""
	msg.pooled = true
	msg.refCount = 1

	return msg
}

// Put returns a message to the pool
func (p *MessagePool) Put(msg *Message) {
	if msg == nil || !msg.pooled {
		return
	}

	atomic.AddUint64(&p.puts, 1)

	// Clear sensitive data
	if cap(msg.Payload) > 1024 { // Prevent memory bloat
		msg.Payload = nil
	} else {
		msg.Payload = msg.Payload[:0]
	}

	msg.refCount = 0
	p.pool.Put(msg)

	if p.logger != nil {
		p.logger.Debug("Message returned to pool",
			log.String("message_id", string(msg.ID)),
			log.Uint64("pool_gets", atomic.LoadUint64(&p.gets)),
			log.Uint64("pool_puts", atomic.LoadUint64(&p.puts)))
	}
}

// Stats returns pool statistics
func (p *MessagePool) Stats() (gets, puts uint64) {
	return atomic.LoadUint64(&p.gets), atomic.LoadUint64(&p.puts)
}

// Default global message pool
var defaultMessagePool = NewMessagePool(log.Provide())

// GetMessage retrieves a message from the default pool
func GetMessage() *Message {
	return defaultMessagePool.Get()
}

// PutMessage returns a message to the default pool
func PutMessage(msg *Message) {
	defaultMessagePool.Put(msg)
}

// MessageHandler defines a function type for handling incoming messages
type MessageHandler func(msg *Message) error

// MessageFilter defines a function type for filtering messages
type MessageFilter func(msg *Message) bool

// StreamHandler defines a function type for handling streaming data
type StreamHandler func(stream Stream) error

// Common message filters

// FilterByType creates a filter that matches messages of a specific type
func FilterByType(msgType MessageType) MessageFilter {
	return func(msg *Message) bool {
		return msg.Type == msgType
	}
}

// FilterByPriority creates a filter that matches messages with minimum priority
func FilterByPriority(minPriority Priority) MessageFilter {
	return func(msg *Message) bool {
		return msg.Priority >= minPriority
	}
}

// FilterBySource creates a filter that matches messages from a specific source
func FilterBySource(sourceID ClientID) MessageFilter {
	return func(msg *Message) bool {
		return msg.SourceClientID == sourceID
	}
}

// FilterByGroup creates a filter that matches messages for a specific group
func FilterByGroup(groupID GroupID) MessageFilter {
	return func(msg *Message) bool {
		return msg.GroupID == groupID
	}
}

// FilterByScope creates a filter that matches messages for a specific scope
func FilterByScope(scopeID ScopeID) MessageFilter {
	return func(msg *Message) bool {
		return msg.ScopeID == scopeID
	}
}

// CombineFilters combines multiple filters with AND logic
func CombineFilters(filters ...MessageFilter) MessageFilter {
	return func(msg *Message) bool {
		for _, filter := range filters {
			if !filter(msg) {
				return false
			}
		}
		return true
	}
}

// AnyFilter combines multiple filters with OR logic
func AnyFilter(filters ...MessageFilter) MessageFilter {
	return func(msg *Message) bool {
		for _, filter := range filters {
			if filter(msg) {
				return true
			}
		}
		return false
	}
}
