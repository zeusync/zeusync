package protocol

import (
	"context"
	"github.com/quic-go/quic-go"
	"github.com/zeusync/zeusync/internal/core/protocol/intrefaces"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
)

var _ intrefaces.Connection = (*QuicConnection)(nil)

// QuicConnection represents a QUIC client connection
type QuicConnection struct {
	id           string
	session      *quic.Conn
	config       intrefaces.ProtocolConfig
	mu           sync.RWMutex
	metadata     map[string]interface{}
	lastActivity int64 // Unix timestamp
	connectedAt  time.Time
	closed       int32

	// Callbacks
	onClose   func(string)
	onError   func(error)
	onMessage func(intrefaces.Message)

	// Metrics
	messagesSent     uint64
	messagesReceived uint64
	bytesSent        uint64
	bytesReceived    uint64

	// Stream management
	currentStream *quic.Stream
	streamMu      sync.Mutex
}

// NewQuicConnection creates a new QUIC connection
func NewQuicConnection(session *quic.Conn, config intrefaces.ProtocolConfig) *QuicConnection {
	now := time.Now()
	return &QuicConnection{
		id:           uuid.New().String(),
		session:      session,
		config:       config,
		metadata:     make(map[string]interface{}),
		lastActivity: now.Unix(),
		connectedAt:  now,
	}
}

// ID returns the connection ID
func (c *QuicConnection) ID() string {
	return c.id
}

// RemoteAddr returns the remote network address
func (c *QuicConnection) RemoteAddr() net.Addr {
	return c.session.RemoteAddr()
}

// LocalAddr returns the local network address
func (c *QuicConnection) LocalAddr() net.Addr {
	return c.session.LocalAddr()
}

// Send sends raw data over the connection
func (c *QuicConnection) Send(data []byte) error {
	if c.IsClosed() {
		return errors.New("connection is closed")
	}

	stream, err := c.getOrCreateStream()
	if err != nil {
		return errors.Wrap(err, "failed to get stream")
	}

	// Set write deadline
	if c.config.WriteTimeout > 0 {
		_ = stream.SetWriteDeadline(time.Now().Add(c.config.WriteTimeout))
	}

	_, err = stream.Write(data)
	if err != nil {
		return errors.Wrap(err, "failed to write data")
	}

	// Update metrics
	atomic.AddUint64(&c.bytesSent, uint64(len(data)))
	atomic.StoreInt64(&c.lastActivity, time.Now().Unix())

	return nil
}

// Receive receives raw data from the connection
func (c *QuicConnection) Receive() ([]byte, error) {
	if c.IsClosed() {
		return nil, errors.New("connection is closed")
	}

	stream, err := c.acceptStream()
	if err != nil {
		return nil, errors.Wrap(err, "failed to accept stream")
	}

	// Set read deadline
	if c.config.ReadTimeout > 0 {
		_ = stream.SetReadDeadline(time.Now().Add(c.config.ReadTimeout))
	}

	// Read data with buffer
	buffer := make([]byte, c.config.BufferSize)
	n, err := stream.Read(buffer)
	if err != nil && err != io.EOF {
		return nil, errors.Wrap(err, "failed to read data")
	}

	data := buffer[:n]

	// Update metrics
	atomic.AddUint64(&c.bytesReceived, uint64(len(data)))
	atomic.StoreInt64(&c.lastActivity, time.Now().Unix())

	return data, nil
}

// SendMessage sends a message over the connection
func (c *QuicConnection) SendMessage(message intrefaces.Message) error {
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

	stream, err := c.getOrCreateStream()
	if err != nil {
		return errors.Wrap(err, "failed to get stream")
	}

	// Set write deadline
	if c.config.WriteTimeout > 0 {
		_ = stream.SetWriteDeadline(time.Now().Add(c.config.WriteTimeout))
	}

	// Write message length first (4 bytes)
	lengthBytes := make([]byte, 4)
	lengthBytes[0] = byte(len(data) >> 24)
	lengthBytes[1] = byte(len(data) >> 16)
	lengthBytes[2] = byte(len(data) >> 8)
	lengthBytes[3] = byte(len(data))

	if _, err := stream.Write(lengthBytes); err != nil {
		return errors.Wrap(err, "failed to write message length")
	}

	// Write message data
	if _, err := stream.Write(data); err != nil {
		return errors.Wrap(err, "failed to write message data")
	}

	// Update metrics
	atomic.AddUint64(&c.messagesSent, 1)
	atomic.AddUint64(&c.bytesSent, uint64(len(data)+4))
	atomic.StoreInt64(&c.lastActivity, time.Now().Unix())

	return nil
}

// ReceiveMessage receives a message from the connection
func (c *QuicConnection) ReceiveMessage() (intrefaces.Message, error) {
	if c.IsClosed() {
		return nil, errors.New("connection is closed")
	}

	stream, err := c.acceptStream()
	if err != nil {
		return nil, errors.Wrap(err, "failed to accept stream")
	}

	// Set read deadline
	if c.config.ReadTimeout > 0 {
		_ = stream.SetReadDeadline(time.Now().Add(c.config.ReadTimeout))
	}

	// Read message length first (4 bytes)
	lengthBytes := make([]byte, 4)
	if _, err = io.ReadFull(stream, lengthBytes); err != nil {
		return nil, errors.Wrap(err, "failed to read message length")
	}

	messageLength := int(lengthBytes[0])<<24 | int(lengthBytes[1])<<16 | int(lengthBytes[2])<<8 | int(lengthBytes[3])

	// Check message size limit
	if c.config.MaxMessageSize > 0 && uint32(messageLength) > c.config.MaxMessageSize {
		return nil, errors.Errorf("message size %d exceeds limit %d", messageLength, c.config.MaxMessageSize)
	}

	// Read message data
	data := make([]byte, messageLength)
	if _, err := io.ReadFull(stream, data); err != nil {
		return nil, errors.Wrap(err, "failed to read message data")
	}

	// Create and unmarshal message
	message := NewMessage("", nil)
	if err = message.Unmarshal(data); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal message")
	}

	// Decompress if needed
	if message.compressed {
		if err = message.Decompress(); err != nil {
			return nil, errors.Wrap(err, "failed to decompress message")
		}
	}

	// Update metrics
	atomic.AddUint64(&c.messagesReceived, 1)
	atomic.AddUint64(&c.bytesReceived, uint64(messageLength+4))
	atomic.StoreInt64(&c.lastActivity, time.Now().Unix())

	// Trigger callback if set
	if c.onMessage != nil {
		c.onMessage(message)
	}

	return message, nil
}

// getOrCreateStream gets or creates a stream for sending data
func (c *QuicConnection) getOrCreateStream() (*quic.Stream, error) {
	c.streamMu.Lock()
	defer c.streamMu.Unlock()

	if c.currentStream == nil {
		stream, err := c.session.OpenStreamSync(context.Background())
		if err != nil {
			return nil, errors.Wrap(err, "failed to open stream")
		}
		c.currentStream = stream
	}

	return c.currentStream, nil
}

// acceptStream accepts an incoming stream
func (c *QuicConnection) acceptStream() (*quic.Stream, error) {
	stream, err := c.session.AcceptStream(context.Background())
	if err != nil {
		return nil, errors.Wrap(err, "failed to accept stream")
	}
	return stream, nil
}

// IsAlive checks if the connection is still active
func (c *QuicConnection) IsAlive() bool {
	if atomic.LoadInt32(&c.closed) == 1 {
		return false
	}

	// Check if session context is done
	select {
	case <-c.session.Context().Done():
		return false
	default:
		return true
	}
}

// IsClosed checks if the connection is closed
func (c *QuicConnection) IsClosed() bool {
	return atomic.LoadInt32(&c.closed) == 1
}

// LastActivity returns the time of the last activity
func (c *QuicConnection) LastActivity() time.Time {
	timestamp := atomic.LoadInt64(&c.lastActivity)
	return time.Unix(timestamp, 0)
}

// Close closes the connection
func (c *QuicConnection) Close() error {
	return c.CloseWithReason("connection closed")
}

// CloseWithReason closes the connection with a specific reason
func (c *QuicConnection) CloseWithReason(reason string) error {
	if !atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		return nil // Already closed
	}

	// Close current stream if exists
	c.streamMu.Lock()
	if c.currentStream != nil {
		_ = c.currentStream.Close()
		c.currentStream = nil
	}
	c.streamMu.Unlock()

	// Close session with application error
	err := c.session.CloseWithError(0, reason)

	// Trigger callback if set
	if c.onClose != nil {
		c.onClose(reason)
	}

	return err
}

// SetTimeout sets the connection timeout
func (c *QuicConnection) SetTimeout(timeout time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config.ReadTimeout = timeout
	c.config.WriteTimeout = timeout
	return nil
}

// SetKeepAlive sets the keep-alive for the connection
func (c *QuicConnection) SetKeepAlive(keepAlive bool) error {
	// QUIC has built-in keep-alive mechanisms
	// This could be used to configure idle timeout
	return nil
}

// SetBufferSize sets the buffer size for the connection
func (c *QuicConnection) SetBufferSize(size int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config.BufferSize = uint32(size)
	return nil
}

// Metadata returns a copy of the connection metadata
func (c *QuicConnection) Metadata() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	metadata := make(map[string]interface{}, len(c.metadata))
	for k, v := range c.metadata {
		metadata[k] = v
	}
	return metadata
}

// SetMetadata sets a metadata key-value pair
func (c *QuicConnection) SetMetadata(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metadata[key] = value
}

// GetMetadata gets a metadata value by key
func (c *QuicConnection) GetMetadata(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	value, exists := c.metadata[key]
	return value, exists
}

// OnClose sets a callback for when the connection is closed
func (c *QuicConnection) OnClose(callback func(string)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onClose = callback
}

// OnError sets a callback for connection errors
func (c *QuicConnection) OnError(callback func(error)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onError = callback
}

// OnMessage sets a callback for received messages
func (c *QuicConnection) OnMessage(callback func(intrefaces.Message)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onMessage = callback
}

// ClientInfo returns client information
func (c *QuicConnection) ClientInfo() intrefaces.ClientInfo {
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

	return intrefaces.ClientInfo{
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
func (c *QuicConnection) GetMetrics() intrefaces.ClientMetrics {
	return intrefaces.ClientMetrics{
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
func (c *QuicConnection) calculateAverageMessageSize() float64 {
	totalMessages := atomic.LoadUint64(&c.messagesSent) + atomic.LoadUint64(&c.messagesReceived)
	if totalMessages == 0 {
		return 0
	}
	totalBytes := atomic.LoadUint64(&c.bytesSent) + atomic.LoadUint64(&c.bytesReceived)
	return float64(totalBytes) / float64(totalMessages)
}
