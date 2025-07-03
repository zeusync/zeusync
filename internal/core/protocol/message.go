package protocol

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
)

var _ IMessage = (*Message)(nil)

// Message represents a protocol message with thread-safe operations
type Message struct {
	mu sync.RWMutex

	// Core fields
	id          string
	timestamp   time.Time
	messageType string
	data        []byte
	payload     interface{}
	headers     map[string]string
	route       string
	isResponse  bool
	responseTo  string
	priority    MessagePriority
	qos         QualityOfService

	// Compression state
	compressed bool
	maxSize    int
}

// MessageOptions provides configuration for message creation
type MessageOptions struct {
	ID         string
	MaxSize    int
	Priority   MessagePriority
	QoS        QualityOfService
	Headers    map[string]string
	Route      string
	Compressed bool
}

// NewMessage creates a new message with the given type and payload
func NewMessage(messageType string, payload any, opts ...MessageOptions) *Message {
	var options MessageOptions
	if len(opts) > 0 {
		options = opts[0]
	}

	id := options.ID
	if id == "" {
		id = uuid.New().String()
	}

	headers := make(map[string]string)
	if options.Headers != nil {
		for k, v := range options.Headers {
			headers[k] = v
		}
	}

	maxSize := options.MaxSize
	if maxSize == 0 {
		maxSize = 1024 * 1024 // 1MB default
	}

	return &Message{
		id:          id,
		timestamp:   time.Now(),
		messageType: messageType,
		payload:     payload,
		headers:     headers,
		route:       options.Route,
		priority:    options.Priority,
		qos:         options.QoS,
		compressed:  options.Compressed,
		maxSize:     maxSize,
	}
}

// Type returns the message type
func (m *Message) Type() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.messageType
}

// Data returns the raw message data
func (m *Message) Data() []byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.data == nil {
		// Lazy marshaling
		data, _ := m.marshal()
		return data
	}
	return append([]byte(nil), m.data...)
}

// Payload returns the message payload
func (m *Message) Payload() interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.payload
}

// SetPayload sets the message payload
func (m *Message) SetPayload(payload interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.payload = payload
	m.data = nil // Invalidate cached data
	return nil
}

// ID returns the message ID
func (m *Message) ID() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.id
}

// Timestamp returns the message timestamp
func (m *Message) Timestamp() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.timestamp
}

// Source returns the message source
func (m *Message) Source() string {
	return m.GetHeader("source")
}

// Target returns the message target
func (m *Message) Target() string {
	return m.GetHeader("target")
}

// Headers returns a copy of the message headers
func (m *Message) Headers() map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	headers := make(map[string]string, len(m.headers))
	for k, v := range m.headers {
		headers[k] = v
	}
	return headers
}

// SetHeader sets a message header
func (m *Message) SetHeader(key, value string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.headers == nil {
		m.headers = make(map[string]string)
	}
	m.headers[key] = value
	m.data = nil // Invalidate cached data
}

// GetHeader gets a message header
func (m *Message) GetHeader(key string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.headers[key]
}

// Route returns the message route
func (m *Message) Route() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.route
}

// SetRoute sets the message route
func (m *Message) SetRoute(route string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.route = route
	m.data = nil // Invalidate cached data
}

// IsResponse returns true if the message is a response
func (m *Message) IsResponse() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isResponse
}

// ResponseTo returns the ID of the message this is a response to
func (m *Message) ResponseTo() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.responseTo
}

// CreateResponse creates a response message
func (m *Message) CreateResponse(payload interface{}) IMessage {
	m.mu.RLock()
	// Получаем все необходимые данные пока держим блокировку
	messageType := m.messageType
	maxSize := m.maxSize
	priority := m.priority
	qos := m.qos
	route := m.route
	id := m.id

	// Получаем заголовки для обмена source/target
	var sourceHeader, targetHeader string
	if m.headers != nil {
		sourceHeader = m.headers["target"] // target становится source в ответе
		targetHeader = m.headers["source"] // source становится target в ответе
	}
	m.mu.RUnlock()

	// Создаем заголовки для нового сообщения
	headers := make(map[string]string)
	if sourceHeader != "" {
		headers["source"] = sourceHeader
	}
	if targetHeader != "" {
		headers["target"] = targetHeader
	}

	resp := NewMessage(messageType, payload, MessageOptions{
		MaxSize:  maxSize,
		Priority: priority,
		QoS:      qos,
		Route:    route,
		Headers:  headers,
	})

	// Устанавливаем поля ответа без дополнительных блокировок
	resp.mu.Lock()
	resp.isResponse = true
	resp.responseTo = id
	resp.mu.Unlock()

	return resp
}

// Marshal serializes the message to bytes
func (m *Message) Marshal() ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.marshal()
}

func (m *Message) marshal() ([]byte, error) {
	if m.data != nil {
		return append([]byte(nil), m.data...), nil
	}

	// Create message envelope
	envelope := struct {
		ID         string            `json:"id"`
		Timestamp  time.Time         `json:"timestamp"`
		Type       string            `json:"type"`
		Payload    interface{}       `json:"payload"`
		Headers    map[string]string `json:"headers"`
		Route      string            `json:"route,omitempty"`
		IsResponse bool              `json:"is_response,omitempty"`
		ResponseTo string            `json:"response_to,omitempty"`
		Priority   MessagePriority   `json:"priority"`
		QoS        QualityOfService  `json:"qos"`
		Compressed bool              `json:"compressed,omitempty"`
	}{
		ID:         m.id,
		Timestamp:  m.timestamp,
		Type:       m.messageType,
		Payload:    m.payload,
		Headers:    m.headers,
		Route:      m.route,
		IsResponse: m.isResponse,
		ResponseTo: m.responseTo,
		Priority:   m.priority,
		QoS:        m.qos,
		Compressed: m.compressed,
	}

	data, err := json.Marshal(envelope)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal message")
	}

	// Check size limit
	if len(data) > m.maxSize {
		return nil, errors.Errorf("message size %d exceeds limit %d", len(data), m.maxSize)
	}

	m.data = data
	return append([]byte(nil), data...), nil
}

// Unmarshal deserializes a message from bytes
func (m *Message) Unmarshal(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	envelope := struct {
		ID         string            `json:"id"`
		Timestamp  time.Time         `json:"timestamp"`
		Type       string            `json:"type"`
		Payload    json.RawMessage   `json:"payload"`
		Headers    map[string]string `json:"headers"`
		Route      string            `json:"route,omitempty"`
		IsResponse bool              `json:"is_response,omitempty"`
		ResponseTo string            `json:"response_to,omitempty"`
		Priority   MessagePriority   `json:"priority"`
		QoS        QualityOfService  `json:"qos"`
		Compressed bool              `json:"compressed,omitempty"`
	}{}

	if err := json.Unmarshal(data, &envelope); err != nil {
		return errors.Wrap(err, "failed to unmarshal message envelope")
	}

	m.id = envelope.ID
	m.timestamp = envelope.Timestamp
	m.messageType = envelope.Type
	m.payload = envelope.Payload
	m.headers = envelope.Headers
	m.route = envelope.Route
	m.isResponse = envelope.IsResponse
	m.responseTo = envelope.ResponseTo
	m.priority = envelope.Priority
	m.qos = envelope.QoS
	m.compressed = envelope.Compressed
	m.data = append([]byte(nil), data...)

	return nil
}

// Clone creates a deep copy of the message
func (m *Message) Clone() IMessage {
	m.mu.RLock()
	defer m.mu.RUnlock()

	headers := make(map[string]string, len(m.headers))
	for k, v := range m.headers {
		headers[k] = v
	}

	clone := &Message{
		id:          m.id,
		timestamp:   m.timestamp,
		messageType: m.messageType,
		payload:     m.payload, // Shallow copy of payload
		headers:     headers,
		route:       m.route,
		isResponse:  m.isResponse,
		responseTo:  m.responseTo,
		priority:    m.priority,
		qos:         m.qos,
		compressed:  m.compressed,
		maxSize:     m.maxSize,
	}

	if m.data != nil {
		clone.data = append([]byte(nil), m.data...)
	}

	return clone
}

// Size returns the message size in bytes
func (m *Message) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.data == nil {
		// Вычисляем размер без проверки лимита
		return m.calculateSizeWithoutLimit()
	}
	return len(m.data)
}

// MaxSize returns the maximum allowed message size
func (m *Message) MaxSize() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.maxSize
}

// calculateSizeWithoutLimit вычисляет размер сообщения без проверки лимита
func (m *Message) calculateSizeWithoutLimit() int {
	envelope := struct {
		ID         string            `json:"id"`
		Timestamp  time.Time         `json:"timestamp"`
		Type       string            `json:"type"`
		Payload    interface{}       `json:"payload"`
		Headers    map[string]string `json:"headers"`
		Route      string            `json:"route,omitempty"`
		IsResponse bool              `json:"is_response,omitempty"`
		ResponseTo string            `json:"response_to,omitempty"`
		Priority   MessagePriority   `json:"priority"`
		QoS        QualityOfService  `json:"qos"`
		Compressed bool              `json:"compressed,omitempty"`
	}{
		ID:         m.id,
		Timestamp:  m.timestamp,
		Type:       m.messageType,
		Payload:    m.payload,
		Headers:    m.headers,
		Route:      m.route,
		IsResponse: m.isResponse,
		ResponseTo: m.responseTo,
		Priority:   m.priority,
		QoS:        m.qos,
		Compressed: m.compressed,
	}

	data, err := json.Marshal(envelope)
	if err != nil {
		return 0
	}
	return len(data)
}

// Compress compresses the message data using gzip
func (m *Message) Compress() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.compressed {
		return nil // Already compressed
	}

	data, err := m.marshal()
	if err != nil {
		return errors.Wrap(err, "failed to marshal before compression")
	}

	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)

	if _, err = gzWriter.Write(data); err != nil {
		_ = gzWriter.Close()
		return errors.Wrap(err, "failed to compress data")
	}

	if err = gzWriter.Close(); err != nil {
		return errors.Wrap(err, "failed to close gzip writer")
	}

	m.data = buf.Bytes()
	m.compressed = true
	return nil
}

// Decompress decompresses the message data
func (m *Message) Decompress() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.compressed {
		return nil // Not compressed
	}

	if m.data == nil {
		return errors.New("no data to decompress")
	}

	gzReader, err := gzip.NewReader(bytes.NewReader(m.data))
	if err != nil {
		return errors.Wrap(err, "failed to create gzip reader")
	}
	defer func() {
		_ = gzReader.Close()
	}()

	var buf bytes.Buffer
	if _, err = buf.ReadFrom(gzReader); err != nil {
		return errors.Wrap(err, "failed to decompress data")
	}

	m.data = buf.Bytes()
	m.compressed = false
	return nil
}

// IsCompressed returns true if the message is compressed
func (m *Message) IsCompressed() bool {
	return m.compressed
}

// Priority returns the message priority
func (m *Message) Priority() MessagePriority {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.priority
}

// SetPriority sets the message priority
func (m *Message) SetPriority(priority MessagePriority) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.priority = priority
	m.data = nil // Invalidate cached data
}

// QoS returns the quality of service
func (m *Message) QoS() QualityOfService {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.qos
}

// SetQoS sets the quality of service
func (m *Message) SetQoS(qos QualityOfService) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.qos = qos
	m.data = nil // Invalidate cached data
}

// Validate validates the message structure
func (m *Message) Validate() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.id == "" {
		return errors.New("message ID is required")
	}
	if m.messageType == "" {
		return errors.New("message type is required")
	}
	if m.timestamp.IsZero() {
		return errors.New("message timestamp is required")
	}

	// Вычисляем размер без вызова m.Size() чтобы избежать дедлока
	var size int
	if m.data == nil {
		// Вычисляем размер без проверки лимита в marshal
		size = m.calculateSizeWithoutLimit()
	} else {
		size = len(m.data)
	}

	if size > m.maxSize {
		return fmt.Errorf("message size %d exceeds maximum %d", size, m.maxSize)
	}

	return nil
}
