package websocket

import (
	"github.com/zeusync/zeusync/internal/core/protocol"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

var _ protocol.Connection = (*Connection)(nil)

// Connection represents a WebSocket client connection
type Connection struct {
	id           string
	conn         *websocket.Conn
	config       protocol.Config
	mu           sync.RWMutex
	metadata     map[string]interface{}
	lastActivity int64 // Unix timestamp
	connectedAt  time.Time
	closed       int32

	// Callbacks
	onClose   func(string)
	onError   func(error)
	onMessage func(protocol.IMessage)

	// Metrics
	messagesSent     uint64
	messagesReceived uint64
	bytesSent        uint64
	bytesReceived    uint64

	// Write mutex to ensure thread-safe writes
	writeMu sync.Mutex
}

// NewWebSocketConnection creates a new WebSocket connection
func NewWebSocketConnection(conn *websocket.Conn, config protocol.Config) *Connection {
	now := time.Now()
	return &Connection{
		id:           uuid.New().String(),
		conn:         conn,
		config:       config,
		metadata:     make(map[string]interface{}),
		lastActivity: now.Unix(),
		connectedAt:  now,
	}
}

// ID returns the connection ID
func (c *Connection) ID() string {
	return c.id
}

// RemoteAddr returns the remote network address
func (c *Connection) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

// LocalAddr returns the local network address
func (c *Connection) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

// Send sends raw data over the connection
func (c *Connection) Send(data []byte) error {
	if c.IsClosed() {
		return errors.New("connection is closed")
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	// Set write deadline
	if c.config.WriteTimeout > 0 {
		_ = c.conn.SetWriteDeadline(time.Now().Add(c.config.WriteTimeout))
	}

	err := c.conn.WriteMessage(websocket.BinaryMessage, data)
	if err != nil {
		return errors.Wrap(err, "failed to write message")
	}

	// Update metrics
	atomic.AddUint64(&c.bytesSent, uint64(len(data)))
	atomic.StoreInt64(&c.lastActivity, time.Now().Unix())

	return nil
}

// Receive receives raw data from the connection
func (c *Connection) Receive() ([]byte, error) {
	if c.IsClosed() {
		return nil, errors.New("connection is closed")
	}

	// Set read deadline
	if c.config.ReadTimeout > 0 {
		_ = c.conn.SetReadDeadline(time.Now().Add(c.config.ReadTimeout))
	}

	messageType, data, err := c.conn.ReadMessage()
	if err != nil {
		return nil, errors.Wrap(err, "failed to read message")
	}

	// Only handle text and binary messages
	if messageType != websocket.TextMessage && messageType != websocket.BinaryMessage {
		return nil, errors.New("unsupported message type")
	}

	// Update metrics
	atomic.AddUint64(&c.bytesReceived, uint64(len(data)))
	atomic.StoreInt64(&c.lastActivity, time.Now().Unix())

	return data, nil
}

// SendMessage sends a message over the connection
func (c *Connection) SendMessage(message protocol.IMessage) error {
	if c.IsClosed() {
		return errors.New("connection is closed")
	}

	// Validate message
	if validator, ok := message.(interface{ Validate() error }); ok {
		if err := validator.Validate(); err != nil {
			return errors.Wrap(err, "message validation failed")
		}
	}

	// Compress if enabled and message is large enough
	if c.config.EnableCompression && message.Size() > 1024 {
		if err := message.Compress(); err != nil {
			// Log but don't fail - compression is optional
		}
	}

	data, err := message.Marshal()
	if err != nil {
		return errors.Wrap(err, "failed to marshal message")
	}

	// Check message size limit
	if c.config.MaxMessageSize > 0 && uint32(len(data)) > c.config.MaxMessageSize {
		return errors.Errorf("message size %d exceeds limit %d", len(data), c.config.MaxMessageSize)
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	// Set write deadline
	if c.config.WriteTimeout > 0 {
		_ = c.conn.SetWriteDeadline(time.Now().Add(c.config.WriteTimeout))
	}

	err = c.conn.WriteMessage(websocket.TextMessage, data)
	if err != nil {
		return errors.Wrap(err, "failed to write message")
	}

	// Update metrics
	atomic.AddUint64(&c.messagesSent, 1)
	atomic.AddUint64(&c.bytesSent, uint64(len(data)))
	atomic.StoreInt64(&c.lastActivity, time.Now().Unix())

	return nil
}

// ReceiveMessage receives a message from the connection
func (c *Connection) ReceiveMessage() (protocol.IMessage, error) {
	if c.IsClosed() {
		return nil, errors.New("connection is closed")
	}

	// Set read deadline
	if c.config.ReadTimeout > 0 {
		_ = c.conn.SetReadDeadline(time.Now().Add(c.config.ReadTimeout))
	}

	messageType, data, err := c.conn.ReadMessage()
	if err != nil {
		return nil, errors.Wrap(err, "failed to read message")
	}

	// Only handle text messages for protocol messages
	if messageType != websocket.TextMessage {
		return nil, errors.New("expected text message for protocol message")
	}

	// Check message size limit
	if c.config.MaxMessageSize > 0 && uint32(len(data)) > c.config.MaxMessageSize {
		return nil, errors.Errorf("message size %d exceeds limit %d", len(data), c.config.MaxMessageSize)
	}

	// Create and unmarshal message
	message := protocol.NewMessage("", nil)
	if err = message.Unmarshal(data); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal message")
	}

	// Decompress if needed
	if message.IsCompressed() {
		if err = message.Decompress(); err != nil {
			return nil, errors.Wrap(err, "failed to decompress message")
		}
	}

	// Update metrics
	atomic.AddUint64(&c.messagesReceived, 1)
	atomic.AddUint64(&c.bytesReceived, uint64(len(data)))
	atomic.StoreInt64(&c.lastActivity, time.Now().Unix())

	// Trigger callback if set
	if c.onMessage != nil {
		c.onMessage(message)
	}

	return message, nil
}

// IsAlive checks if the connection is still active
func (c *Connection) IsAlive() bool {
	return atomic.LoadInt32(&c.closed) == 0
}

// IsClosed checks if the connection is closed
func (c *Connection) IsClosed() bool {
	return atomic.LoadInt32(&c.closed) == 1
}

// LastActivity returns the time of the last activity
func (c *Connection) LastActivity() time.Time {
	timestamp := atomic.LoadInt64(&c.lastActivity)
	return time.Unix(timestamp, 0)
}

// Close closes the connection
func (c *Connection) Close() error {
	return c.CloseWithReason("connection closed")
}

// CloseWithReason closes the connection with a specific reason
func (c *Connection) CloseWithReason(reason string) error {
	if !atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		return nil // Already closed
	}

	// Send close message
	c.writeMu.Lock()
	closeMessage := websocket.FormatCloseMessage(websocket.CloseNormalClosure, reason)
	_ = c.conn.WriteControl(websocket.CloseMessage, closeMessage, time.Now().Add(time.Second))
	c.writeMu.Unlock()

	// Close underlying connection
	err := c.conn.Close()

	// Trigger callback if set
	if c.onClose != nil {
		c.onClose(reason)
	}

	return err
}

// SetTimeout sets the connection timeout
func (c *Connection) SetTimeout(timeout time.Duration) error {
	// WebSocket timeouts are handled per operation
	// This could be used to update the config
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config.ReadTimeout = timeout
	c.config.WriteTimeout = timeout
	return nil
}

// SetKeepAlive sets the keep-alive for the connection
func (c *Connection) SetKeepAlive(keepAlive bool) error {
	// WebSocket has built-in keep-alive via ping/pong
	// This is handled at the protocol level
	return nil
}

// SetBufferSize sets the buffer size for the connection
func (c *Connection) SetBufferSize(size int) error {
	// Buffer sizes are set during connection creation
	// This could be used for future connections
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config.BufferSize = uint32(size)
	return nil
}

// Metadata returns a copy of the connection metadata
func (c *Connection) Metadata() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	metadata := make(map[string]interface{}, len(c.metadata))
	for k, v := range c.metadata {
		metadata[k] = v
	}
	return metadata
}

// SetMetadata sets a metadata key-value pair
func (c *Connection) SetMetadata(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metadata[key] = value
}

// GetMetadata gets a metadata value by key
func (c *Connection) GetMetadata(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	value, exists := c.metadata[key]
	return value, exists
}

// OnClose sets a callback for when the connection is closed
func (c *Connection) OnClose(callback func(string)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onClose = callback
}

// OnError sets a callback for connection errors
func (c *Connection) OnError(callback func(error)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onError = callback
}

// OnMessage sets a callback for received messages
func (c *Connection) OnMessage(callback func(protocol.IMessage)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onMessage = callback
}

// ClientInfo returns client information
func (c *Connection) ClientInfo() protocol.ClientInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Extract groups from metadata
	var groups []string
	if groupsInterface, exists := c.metadata["groups"]; exists {
		if groupsSlice, ok := groupsInterface.([]string); ok {
			groups = groupsSlice
		}
	}

	// Extract user info from metadata
	userID, _ := c.metadata["user_id"].(string)
	isAuthenticated, _ := c.metadata["authenticated"].(bool)
	userAgent, _ := c.metadata["user_agent"].(string)
	version, _ := c.metadata["version"].(string)

	var permissions []string
	if permsInterface, exists := c.metadata["permissions"]; exists {
		if permsSlice, ok := permsInterface.([]string); ok {
			permissions = permsSlice
		}
	}

	return protocol.ClientInfo{
		ID:              c.id,
		RemoteAddress:   c.RemoteAddr().String(),
		ConnectedAt:     c.connectedAt,
		LastActivity:    c.LastActivity(),
		UserAgent:       userAgent,
		Version:         version,
		Groups:          groups,
		Metadata:        c.Metadata(),
		IsAuthenticated: isAuthenticated,
		UserID:          userID,
		Permissions:     permissions,
	}
}

// GetMetrics returns connection metrics
func (c *Connection) GetMetrics() protocol.ClientMetrics {
	return protocol.ClientMetrics{
		ActiveConnections:    1, // This connection
		TotalConnections:     1,
		FailedConnections:    0,
		ConnectionsPerSecond: 0,
		MessagesSent:         atomic.LoadUint64(&c.messagesSent),
		MessagesReceived:     atomic.LoadUint64(&c.messagesReceived),
		MessagesPerSecond:    0, // Would need time tracking
		AverageMessageSize:   c.calculateAverageMessageSize(),
	}
}

// calculateAverageMessageSize calculates the average message size
func (c *Connection) calculateAverageMessageSize() float64 {
	totalMessages := atomic.LoadUint64(&c.messagesSent) + atomic.LoadUint64(&c.messagesReceived)
	if totalMessages == 0 {
		return 0
	}
	totalBytes := atomic.LoadUint64(&c.bytesSent) + atomic.LoadUint64(&c.bytesReceived)
	return float64(totalBytes) / float64(totalMessages)
}

// SetPongHandler sets the pong handler for the WebSocket connection
func (c *Connection) SetPongHandler(handler func(string) error) {
	c.conn.SetPongHandler(handler)
}

// WriteControl writes a control message with the given deadline
func (c *Connection) WriteControl(messageType int, data []byte, deadline time.Time) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.conn.WriteControl(messageType, data, deadline)
}
