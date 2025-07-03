package protocol

import (
	"encoding/json"
	"github.com/google/uuid"
	"sync"
	"time"
)

// MessagePool provides object pooling for messages to reduce GC pressure
type MessagePool struct {
	originalPool  sync.Pool
	optimizedPool sync.Pool
	bufferPool    sync.Pool
}

// NewMessagePool creates a new message pool
func NewMessagePool() *MessagePool {
	return &MessagePool{
		originalPool: sync.Pool{
			New: func() interface{} {
				return &Message{
					headers: make(map[string]string),
				}
			},
		},
		optimizedPool: sync.Pool{
			New: func() interface{} {
				return &OptimizedMessage{
					headers:      make(map[string]string),
					marshalBuf:   make([]byte, 0, 1024),
					unmarshalBuf: make([]byte, 0, 1024),
				}
			},
		},
		bufferPool: sync.Pool{
			New: func() interface{} {
				return make([]byte, 0, 1024)
			},
		},
	}
}

// GetOriginalMessage gets a message from the pool
func (p *MessagePool) GetOriginalMessage() *Message {
	msg := p.originalPool.Get().(*Message)
	msg.reset()
	return msg
}

// PutOriginalMessage returns a message to the pool
func (p *MessagePool) PutOriginalMessage(msg *Message) {
	if msg != nil {
		p.originalPool.Put(msg)
	}
}

// GetOptimizedMessage gets an optimized message from the pool
func (p *MessagePool) GetOptimizedMessage() *OptimizedMessage {
	msg := p.optimizedPool.Get().(*OptimizedMessage)
	msg.reset()
	return msg
}

// PutOptimizedMessage returns an optimized message to the pool
func (p *MessagePool) PutOptimizedMessage(msg *OptimizedMessage) {
	if msg != nil {
		p.optimizedPool.Put(msg)
	}
}

// GetBuffer gets a buffer from the pool
func (p *MessagePool) GetBuffer() []byte {
	return p.bufferPool.Get().([]byte)[:0]
}

// PutBuffer returns a buffer to the pool
func (p *MessagePool) PutBuffer(buf []byte) {
	if buf != nil && cap(buf) <= 4096 { // Don't pool very large buffers
		p.bufferPool.Put(buf)
	}
}

// reset clears the message for reuse
func (m *Message) reset() {
	m.id = ""
	m.timestamp = time.Time{}
	m.messageType = ""
	m.data = nil
	m.payload = nil
	m.route = ""
	m.isResponse = false
	m.responseTo = ""
	m.priority = 0
	m.qos = 0
	m.compressed = false
	m.maxSize = 1024 * 1024

	// Clear headers map but keep the underlying storage
	for k := range m.headers {
		delete(m.headers, k)
	}
}

// reset clears the optimized message for reuse
func (m *OptimizedMessage) reset() {
	m.id = ""
	m.timestamp = time.Time{}
	m.messageType = ""
	m.data = nil
	m.payload = nil
	m.route = ""
	m.isResponse = false
	m.responseTo = ""
	m.priority = 0
	m.qos = 0
	m.compressed = false
	m.maxSize = 1024 * 1024

	// Clear headers map but keep the underlying storage
	for k := range m.headers {
		delete(m.headers, k)
	}

	// Reset buffers but keep capacity
	m.marshalBuf = m.marshalBuf[:0]
	m.unmarshalBuf = m.unmarshalBuf[:0]
}

// Global message pool instance
var DefaultMessagePool = NewMessagePool()

// NewPooledMessage creates a new message using the pool
func NewPooledMessage(messageType string, payload interface{}, opts ...MessageOptions) *Message {
	msg := DefaultMessagePool.GetOriginalMessage()

	var options MessageOptions
	if len(opts) > 0 {
		options = opts[0]
	}

	msg.id = options.ID
	if msg.id == "" {
		msg.id = uuid.New().String()
	}

	if options.Headers != nil {
		for k, v := range options.Headers {
			msg.headers[k] = v
		}
	}

	msg.maxSize = options.MaxSize
	if msg.maxSize == 0 {
		msg.maxSize = 1024 * 1024
	}

	msg.timestamp = time.Now()
	msg.messageType = messageType
	msg.payload = payload
	msg.route = options.Route
	msg.priority = options.Priority
	msg.qos = options.QoS
	msg.compressed = options.Compressed

	return msg
}

// NewPooledOptimizedMessage creates a new optimized message using the pool
func NewPooledOptimizedMessage(messageType string, payload interface{}, opts ...MessageOptions) *OptimizedMessage {
	msg := DefaultMessagePool.GetOptimizedMessage()

	var options MessageOptions
	if len(opts) > 0 {
		options = opts[0]
	}

	msg.id = options.ID
	if msg.id == "" {
		msg.id = uuid.New().String()
	}

	if options.Headers != nil {
		for k, v := range options.Headers {
			msg.headers[k] = v
		}
	}

	msg.maxSize = options.MaxSize
	if msg.maxSize == 0 {
		msg.maxSize = 1024 * 1024
	}

	msg.timestamp = time.Now()
	msg.messageType = messageType
	msg.route = options.Route
	msg.priority = options.Priority
	msg.qos = options.QoS
	msg.compressed = options.Compressed

	// Serialize payload once
	if payload != nil {
		if data, err := json.Marshal(payload); err == nil {
			msg.payload = data
		}
	}

	return msg
}

// Release returns the message to the pool
func (m *Message) Release() {
	DefaultMessagePool.PutOriginalMessage(m)
}

// Release returns the optimized message to the pool
func (m *OptimizedMessage) Release() {
	DefaultMessagePool.PutOptimizedMessage(m)
}
