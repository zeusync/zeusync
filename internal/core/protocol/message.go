package protocol

import (
	"encoding/binary"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// BinaryMessage - высокопроизводительная реализация Message с бинарной сериализацией
type BinaryMessage struct {
	mu sync.RWMutex

	id        string
	msgType   string
	payload   []byte
	timestamp time.Time
	headers   map[string]string
}

// NewMessage создает новое сообщение
func NewMessage(msgType string, payload []byte) Message {
	return &BinaryMessage{
		id:        uuid.New().String(),
		msgType:   msgType,
		payload:   payload,
		timestamp: time.Now(),
		headers:   make(map[string]string),
	}
}

// NewMessageWithID создает сообщение с заданным ID
func NewMessageWithID(id, msgType string, payload []byte) Message {
	return &BinaryMessage{
		id:        id,
		msgType:   msgType,
		payload:   payload,
		timestamp: time.Now(),
		headers:   make(map[string]string),
	}
}

// ID возвращает уникальный идентификатор сообщения
func (m *BinaryMessage) ID() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.id
}

// Type возвращает тип сообщения
func (m *BinaryMessage) Type() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.msgType
}

// Payload возвращает полезную нагрузку сообщения
func (m *BinaryMessage) Payload() []byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	// Возвращаем копию для безопасности
	if m.payload == nil {
		return nil
	}
	result := make([]byte, len(m.payload))
	copy(result, m.payload)
	return result
}

// Timestamp возвращает время создания сообщения
func (m *BinaryMessage) Timestamp() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.timestamp
}

// GetHeader возвращает значение заголовка
func (m *BinaryMessage) GetHeader(key string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.headers[key]
}

// SetHeader устанавливает значение заголовка
func (m *BinaryMessage) SetHeader(key, value string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.headers == nil {
		m.headers = make(map[string]string)
	}
	m.headers[key] = value
}

// Headers возвращает копию всех заголовков
func (m *BinaryMessage) Headers() map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]string, len(m.headers))
	for k, v := range m.headers {
		result[k] = v
	}
	return result
}

// Size возвращает размер сообщения в байтах
func (m *BinaryMessage) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	size := 0
	size += 16                 // UUID (фиксированный размер)
	size += 8                  // timestamp
	size += 2 + len(m.msgType) // длина типа + тип
	size += 4 + len(m.payload) // длина payload + payload
	size += 2                  // количество заголовков

	for k, v := range m.headers {
		size += 2 + len(k) // длина ключа + ключ
		size += 2 + len(v) // длина значения + значение
	}

	return size
}

// Clone создает копию сообщения
func (m *BinaryMessage) Clone() Message {
	m.mu.RLock()
	defer m.mu.RUnlock()

	clone := &BinaryMessage{
		id:        m.id,
		msgType:   m.msgType,
		timestamp: m.timestamp,
		headers:   make(map[string]string, len(m.headers)),
	}

	// Копируем payload
	if m.payload != nil {
		clone.payload = make([]byte, len(m.payload))
		copy(clone.payload, m.payload)
	}

	// Копируем заг��ловки
	for k, v := range m.headers {
		clone.headers[k] = v
	}

	return clone
}

// Marshal сериализует сообщение в бинарный формат
// Формат:
// [16 bytes] UUID
// [8 bytes]  timestamp (unix nano)
// [2 bytes]  type length
// [N bytes]  type
// [4 bytes]  payload length
// [N bytes]  payload
// [2 bytes]  headers count
// [headers]  key_len(2) + key + value_len(2) + value
func (m *BinaryMessage) Marshal() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Вычисляем общий размер
	totalSize := m.Size()
	buf := make([]byte, totalSize)
	offset := 0

	// UUID (16 bytes)
	uuidBytes, err := uuid.Parse(m.id)
	if err != nil {
		return nil, fmt.Errorf("invalid UUID: %w", err)
	}
	copy(buf[offset:], uuidBytes[:])
	offset += 16

	// Timestamp (8 bytes)
	binary.LittleEndian.PutUint64(buf[offset:], uint64(m.timestamp.UnixNano()))
	offset += 8

	// Type length + type
	typeBytes := []byte(m.msgType)
	binary.LittleEndian.PutUint16(buf[offset:], uint16(len(typeBytes)))
	offset += 2
	copy(buf[offset:], typeBytes)
	offset += len(typeBytes)

	// Payload length + payload
	binary.LittleEndian.PutUint32(buf[offset:], uint32(len(m.payload)))
	offset += 4
	if len(m.payload) > 0 {
		copy(buf[offset:], m.payload)
		offset += len(m.payload)
	}

	// Headers count
	binary.LittleEndian.PutUint16(buf[offset:], uint16(len(m.headers)))
	offset += 2

	// Headers
	for k, v := range m.headers {
		keyBytes := []byte(k)
		valueBytes := []byte(v)

		// Key length + key
		binary.LittleEndian.PutUint16(buf[offset:], uint16(len(keyBytes)))
		offset += 2
		copy(buf[offset:], keyBytes)
		offset += len(keyBytes)

		// Value length + value
		binary.LittleEndian.PutUint16(buf[offset:], uint16(len(valueBytes)))
		offset += 2
		copy(buf[offset:], valueBytes)
		offset += len(valueBytes)
	}

	return buf, nil
}

// Unmarshal десериализует сообщение из бинарного формата
func (m *BinaryMessage) Unmarshal(data []byte) error {
	if len(data) < 30 { // Минимальный размер: 16+8+2+0+4+0+2 = 32, но может быть пустой тип
		return fmt.Errorf("data too short: %d bytes", len(data))
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	offset := 0

	// UUID (16 bytes)
	uuidBytes := make([]byte, 16)
	copy(uuidBytes, data[offset:offset+16])
	m.id = uuid.Must(uuid.FromBytes(uuidBytes)).String()
	offset += 16

	// Timestamp (8 bytes)
	timestampNano := binary.LittleEndian.Uint64(data[offset : offset+8])
	m.timestamp = time.Unix(0, int64(timestampNano))
	offset += 8

	// Type length + type
	if offset+2 > len(data) {
		return fmt.Errorf("insufficient data for type length")
	}
	typeLen := binary.LittleEndian.Uint16(data[offset : offset+2])
	offset += 2

	if offset+int(typeLen) > len(data) {
		return fmt.Errorf("insufficient data for type")
	}
	m.msgType = string(data[offset : offset+int(typeLen)])
	offset += int(typeLen)

	// Payload length + payload
	if offset+4 > len(data) {
		return fmt.Errorf("insufficient data for payload length")
	}
	payloadLen := binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4

	if offset+int(payloadLen) > len(data) {
		return fmt.Errorf("insufficient data for payload")
	}
	if payloadLen > 0 {
		m.payload = make([]byte, payloadLen)
		copy(m.payload, data[offset:offset+int(payloadLen)])
		offset += int(payloadLen)
	} else {
		m.payload = nil
	}

	// Headers count
	if offset+2 > len(data) {
		return fmt.Errorf("insufficient data for headers count")
	}
	headersCount := binary.LittleEndian.Uint16(data[offset : offset+2])
	offset += 2

	// Headers
	m.headers = make(map[string]string, headersCount)
	for i := uint16(0); i < headersCount; i++ {
		// Key length + key
		if offset+2 > len(data) {
			return fmt.Errorf("insufficient data for header key length")
		}
		keyLen := binary.LittleEndian.Uint16(data[offset : offset+2])
		offset += 2

		if offset+int(keyLen) > len(data) {
			return fmt.Errorf("insufficient data for header key")
		}
		key := string(data[offset : offset+int(keyLen)])
		offset += int(keyLen)

		// Value length + value
		if offset+2 > len(data) {
			return fmt.Errorf("insufficient data for header value length")
		}
		valueLen := binary.LittleEndian.Uint16(data[offset : offset+2])
		offset += 2

		if offset+int(valueLen) > len(data) {
			return fmt.Errorf("insufficient data for header value")
		}
		value := string(data[offset : offset+int(valueLen)])
		offset += int(valueLen)

		m.headers[key] = value
	}

	return nil
}

// MessagePool - пул для переиспользования сообщений
type MessagePool struct {
	pool sync.Pool
}

// NewMessagePool создает новый пул сообщений
func NewMessagePool() *MessagePool {
	return &MessagePool{
		pool: sync.Pool{
			New: func() interface{} {
				return &BinaryMessage{
					headers: make(map[string]string),
				}
			},
		},
	}
}

// Get получает сообщение из пула
func (p *MessagePool) Get() *BinaryMessage {
	msg := p.pool.Get().(*BinaryMessage)
	msg.reset()
	return msg
}

// Put возвращает сообщение в пул
func (p *MessagePool) Put(msg *BinaryMessage) {
	if msg != nil {
		p.pool.Put(msg)
	}
}

// reset очищает сообщение для переиспользования
func (m *BinaryMessage) reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.id = ""
	m.msgType = ""
	m.payload = nil
	m.timestamp = time.Time{}

	// Очищаем map, но сохраняем выделенную память
	for k := range m.headers {
		delete(m.headers, k)
	}
}

// Release возвращает сообщение в глобальный пул
func (m *BinaryMessage) Release() {
	defaultMessagePool.Put(m)
}

// Глобальный пул сообщений
var defaultMessagePool = NewMessagePool()

// GetPooledMessage получает сообщение из глобального пула
func GetPooledMessage(msgType string, payload []byte) *BinaryMessage {
	msg := defaultMessagePool.Get()
	msg.id = uuid.New().String()
	msg.msgType = msgType
	msg.timestamp = time.Now()

	if payload != nil {
		msg.payload = make([]byte, len(payload))
		copy(msg.payload, payload)
	}

	return msg
}
