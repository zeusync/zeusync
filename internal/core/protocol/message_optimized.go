package protocol

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sync"
	"time"
	"unsafe"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/zeusync/zeusync/internal/core/protocol/intrefaces"
)

// OptimizedMessage represents an optimized protocol message with faster serialization
type OptimizedMessage struct {
	mu sync.RWMutex

	// Core fields
	id          string
	timestamp   time.Time
	messageType string
	data        []byte
	payload     []byte // Store payload as raw bytes
	headers     map[string]string
	route       string
	isResponse  bool
	responseTo  string
	priority    intrefaces.MessagePriority
	qos         intrefaces.QualityOfService

	// Compression state
	compressed bool
	maxSize    int

	// Optimization: pre-allocated buffers
	marshalBuf   []byte
	unmarshalBuf []byte
}

// Binary protocol format:
// [4 bytes: total length]
// [16 bytes: UUID]
// [8 bytes: timestamp]
// [1 byte: message type length][message type]
// [4 bytes: payload length][payload]
// [2 bytes: headers count][headers...]
// [1 byte: route length][route]
// [1 byte: flags (isResponse, compressed, priority, qos)]
// [16 bytes: responseTo UUID if isResponse]

const (
	headerSizeFixed = 4 + 16 + 8 + 1 + 4 + 2 + 1 + 1 + 16 // Fixed size parts
	uuidSize        = 16
)

// NewOptimizedMessage creates a new optimized message
func NewOptimizedMessage(messageType string, payload interface{}, opts ...MessageOptions) *OptimizedMessage {
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

	// Serialize payload once
	var payloadBytes []byte
	if payload != nil {
		if data, err := json.Marshal(payload); err == nil {
			payloadBytes = data
		}
	}

	return &OptimizedMessage{
		id:           id,
		timestamp:    time.Now(),
		messageType:  messageType,
		payload:      payloadBytes,
		headers:      headers,
		route:        options.Route,
		priority:     options.Priority,
		qos:          options.QoS,
		compressed:   options.Compressed,
		maxSize:      maxSize,
		marshalBuf:   make([]byte, 0, 1024), // Pre-allocate 1KB
		unmarshalBuf: make([]byte, 0, 1024), // Pre-allocate 1KB
	}
}

// Marshal serializes the message using binary format
func (m *OptimizedMessage) Marshal() ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.marshalBinary()
}

func (m *OptimizedMessage) marshalBinary() ([]byte, error) {
	if m.data != nil {
		return append([]byte(nil), m.data...), nil
	}

	// Reset buffer but keep capacity
	m.marshalBuf = m.marshalBuf[:0]

	// Calculate total size first
	totalSize := headerSizeFixed + len(m.messageType) + len(m.payload) + len(m.route)
	for k, v := range m.headers {
		totalSize += 2 + len(k) + len(v) // 1 byte for key len, 1 byte for value len
	}

	// Ensure buffer capacity
	if cap(m.marshalBuf) < totalSize {
		m.marshalBuf = make([]byte, 0, totalSize*2)
	}

	buf := bytes.NewBuffer(m.marshalBuf)

	// Write total length (4 bytes)
	if err := binary.Write(buf, binary.LittleEndian, uint32(totalSize-4)); err != nil {
		return nil, err
	}

	// Write UUID (16 bytes)
	uuidBytes, err := uuid.Parse(m.id)
	if err != nil {
		return nil, err
	}
	buf.Write(uuidBytes[:])

	// Write timestamp (8 bytes)
	if err := binary.Write(buf, binary.LittleEndian, m.timestamp.UnixNano()); err != nil {
		return nil, err
	}

	// Write message type
	buf.WriteByte(byte(len(m.messageType)))
	buf.WriteString(m.messageType)

	// Write payload
	if err := binary.Write(buf, binary.LittleEndian, uint32(len(m.payload))); err != nil {
		return nil, err
	}
	buf.Write(m.payload)

	// Write headers
	if err := binary.Write(buf, binary.LittleEndian, uint16(len(m.headers))); err != nil {
		return nil, err
	}
	for k, v := range m.headers {
		buf.WriteByte(byte(len(k)))
		buf.WriteString(k)
		buf.WriteByte(byte(len(v)))
		buf.WriteString(v)
	}

	// Write route
	buf.WriteByte(byte(len(m.route)))
	buf.WriteString(m.route)

	// Write flags (1 byte)
	var flags byte
	if m.isResponse {
		flags |= 1
	}
	if m.compressed {
		flags |= 2
	}
	flags |= byte(m.priority) << 2
	flags |= byte(m.qos) << 4
	buf.WriteByte(flags)

	// Write responseTo UUID if response
	if m.isResponse {
		if responseUUID, err := uuid.Parse(m.responseTo); err == nil {
			buf.Write(responseUUID[:])
		} else {
			buf.Write(make([]byte, 16)) // Empty UUID
		}
	} else {
		buf.Write(make([]byte, 16)) // Empty UUID
	}

	data := buf.Bytes()

	// Check size limit
	if len(data) > m.maxSize {
		return nil, errors.Errorf("message size %d exceeds limit %d", len(data), m.maxSize)
	}

	m.data = append([]byte(nil), data...)
	return data, nil
}

// Unmarshal deserializes a message from binary format
func (m *OptimizedMessage) Unmarshal(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(data) < 4 {
		return errors.New("data too short")
	}

	buf := bytes.NewReader(data)

	// Read total length
	var totalLen uint32
	if err := binary.Read(buf, binary.LittleEndian, &totalLen); err != nil {
		return err
	}

	// Read UUID
	uuidBytes := make([]byte, 16)
	if _, err := buf.Read(uuidBytes); err != nil {
		return err
	}
	m.id = uuid.Must(uuid.FromBytes(uuidBytes)).String()

	// Read timestamp
	var timestampNano int64
	if err := binary.Read(buf, binary.LittleEndian, &timestampNano); err != nil {
		return err
	}
	m.timestamp = time.Unix(0, timestampNano)

	// Read message type
	msgTypeLen, err := buf.ReadByte()
	if err != nil {
		return err
	}
	msgTypeBytes := make([]byte, msgTypeLen)
	if _, err := buf.Read(msgTypeBytes); err != nil {
		return err
	}
	m.messageType = *(*string)(unsafe.Pointer(&msgTypeBytes)) // Zero-copy string conversion

	// Read payload
	var payloadLen uint32
	if err := binary.Read(buf, binary.LittleEndian, &payloadLen); err != nil {
		return err
	}
	m.payload = make([]byte, payloadLen)
	if _, err := buf.Read(m.payload); err != nil {
		return err
	}

	// Read headers
	var headerCount uint16
	if err := binary.Read(buf, binary.LittleEndian, &headerCount); err != nil {
		return err
	}
	m.headers = make(map[string]string, headerCount)
	for i := uint16(0); i < headerCount; i++ {
		keyLen, err := buf.ReadByte()
		if err != nil {
			return err
		}
		keyBytes := make([]byte, keyLen)
		if _, err := buf.Read(keyBytes); err != nil {
			return err
		}

		valueLen, err := buf.ReadByte()
		if err != nil {
			return err
		}
		valueBytes := make([]byte, valueLen)
		if _, err := buf.Read(valueBytes); err != nil {
			return err
		}

		// Zero-copy string conversion
		key := *(*string)(unsafe.Pointer(&keyBytes))
		value := *(*string)(unsafe.Pointer(&valueBytes))
		m.headers[key] = value
	}

	// Read route
	routeLen, err := buf.ReadByte()
	if err != nil {
		return err
	}
	if routeLen > 0 {
		routeBytes := make([]byte, routeLen)
		if _, err := buf.Read(routeBytes); err != nil {
			return err
		}
		m.route = *(*string)(unsafe.Pointer(&routeBytes))
	}

	// Read flags
	flags, err := buf.ReadByte()
	if err != nil {
		return err
	}
	m.isResponse = (flags & 1) != 0
	m.compressed = (flags & 2) != 0
	m.priority = intrefaces.MessagePriority((flags >> 2) & 3)
	m.qos = intrefaces.QualityOfService((flags >> 4) & 3)

	// Read responseTo UUID
	responseUUIDBytes := make([]byte, 16)
	if _, err = buf.Read(responseUUIDBytes); err != nil {
		return err
	}
	if m.isResponse {
		m.responseTo = uuid.Must(uuid.FromBytes(responseUUIDBytes)).String()
	}

	m.data = append([]byte(nil), data...)
	return nil
}

func (m *OptimizedMessage) Type() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.messageType
}

func (m *OptimizedMessage) Data() []byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.data == nil {
		data, _ := m.marshalBinary()
		return data
	}
	return append([]byte(nil), m.data...)
}

func (m *OptimizedMessage) Payload() interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.payload) == 0 {
		return nil
	}
	var result interface{}
	_ = json.Unmarshal(m.payload, &result)
	return result
}

func (m *OptimizedMessage) SetPayload(payload interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if payload == nil {
		m.payload = nil
	} else {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		m.payload = data
	}
	m.data = nil // Invalidate cached data
	return nil
}

func (m *OptimizedMessage) ID() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.id
}

func (m *OptimizedMessage) Timestamp() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.timestamp
}

func (m *OptimizedMessage) Source() string {
	return m.GetHeader("source")
}

func (m *OptimizedMessage) Target() string {
	return m.GetHeader("target")
}

func (m *OptimizedMessage) Headers() map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	headers := make(map[string]string, len(m.headers))
	for k, v := range m.headers {
		headers[k] = v
	}
	return headers
}

func (m *OptimizedMessage) SetHeader(key, value string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.headers == nil {
		m.headers = make(map[string]string)
	}
	m.headers[key] = value
	m.data = nil // Invalidate cached data
}

func (m *OptimizedMessage) GetHeader(key string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.headers[key]
}

func (m *OptimizedMessage) Route() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.route
}

func (m *OptimizedMessage) SetRoute(route string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.route = route
	m.data = nil // Invalidate cached data
}

func (m *OptimizedMessage) IsResponse() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isResponse
}

func (m *OptimizedMessage) ResponseTo() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.responseTo
}

func (m *OptimizedMessage) CreateResponse(payload interface{}) intrefaces.Message {
	m.mu.RLock()
	messageType := m.messageType
	maxSize := m.maxSize
	priority := m.priority
	qos := m.qos
	route := m.route
	id := m.id

	var sourceHeader, targetHeader string
	if m.headers != nil {
		sourceHeader = m.headers["target"]
		targetHeader = m.headers["source"]
	}
	m.mu.RUnlock()

	headers := make(map[string]string)
	if sourceHeader != "" {
		headers["source"] = sourceHeader
	}
	if targetHeader != "" {
		headers["target"] = targetHeader
	}

	resp := NewOptimizedMessage(messageType, payload, MessageOptions{
		MaxSize:  maxSize,
		Priority: priority,
		QoS:      qos,
		Route:    route,
		Headers:  headers,
	})

	resp.mu.Lock()
	resp.isResponse = true
	resp.responseTo = id
	resp.mu.Unlock()

	return resp
}

func (m *OptimizedMessage) Clone() intrefaces.Message {
	m.mu.RLock()
	defer m.mu.RUnlock()

	headers := make(map[string]string, len(m.headers))
	for k, v := range m.headers {
		headers[k] = v
	}

	clone := &OptimizedMessage{
		id:           m.id,
		timestamp:    m.timestamp,
		messageType:  m.messageType,
		payload:      append([]byte(nil), m.payload...),
		headers:      headers,
		route:        m.route,
		isResponse:   m.isResponse,
		responseTo:   m.responseTo,
		priority:     m.priority,
		qos:          m.qos,
		compressed:   m.compressed,
		maxSize:      m.maxSize,
		marshalBuf:   make([]byte, 0, cap(m.marshalBuf)),
		unmarshalBuf: make([]byte, 0, cap(m.unmarshalBuf)),
	}

	if m.data != nil {
		clone.data = append([]byte(nil), m.data...)
	}

	return clone
}

func (m *OptimizedMessage) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.data == nil {
		return m.calculateSizeWithoutLimit()
	}
	return len(m.data)
}

func (m *OptimizedMessage) MaxSize() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.maxSize
}

func (m *OptimizedMessage) calculateSizeWithoutLimit() int {
	totalSize := headerSizeFixed + len(m.messageType) + len(m.payload) + len(m.route)
	for k, v := range m.headers {
		totalSize += 2 + len(k) + len(v)
	}
	return totalSize
}

func (m *OptimizedMessage) Compress() error {
	// TODO: Implement compression for binary format
	return nil
}

func (m *OptimizedMessage) Decompress() error {
	// TODO: Implement decompression for binary format
	return nil
}

func (m *OptimizedMessage) Priority() intrefaces.MessagePriority {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.priority
}

func (m *OptimizedMessage) SetPriority(priority intrefaces.MessagePriority) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.priority = priority
	m.data = nil
}

func (m *OptimizedMessage) QoS() intrefaces.QualityOfService {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.qos
}

func (m *OptimizedMessage) SetQoS(qos intrefaces.QualityOfService) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.qos = qos
	m.data = nil
}

func (m *OptimizedMessage) Validate() error {
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

	var size int
	if m.data == nil {
		size = m.calculateSizeWithoutLimit()
	} else {
		size = len(m.data)
	}

	if size > m.maxSize {
		return fmt.Errorf("message size %d exceeds maximum %d", size, m.maxSize)
	}

	return nil
}
