// Package client provides a high-level QUIC client SDK for ZeuSync
package client

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zeusync/zeusync/internal/core/observability/log"
	"github.com/zeusync/zeusync/internal/core/protocol"
	"github.com/zeusync/zeusync/internal/core/protocol/quic"
)

// Client represents a ZeuSync client connection
type Client struct {
	// Connection management
	conn      protocol.Connection
	transport *quic.Transport

	// Client state
	id       protocol.ClientID
	groups   sync.Map // map[protocol.GroupID]bool
	scopes   sync.Map // map[protocol.ScopeID]bool
	metadata sync.Map // map[string]interface{}

	// Event handlers
	messageHandlers map[protocol.MessageType][]MessageHandler
	eventHandlers   map[EventType][]EventHandler
	handlerMutex    sync.RWMutex

	// Lifecycle
	connected int32 // atomic bool
	closed    int32 // atomic bool
	done      chan struct{}

	// Configuration and logging
	config Config
	logger log.Log

	// Background workers
	workerGroup sync.WaitGroup
}

// Config holds configuration for the client
type Config struct {
	// Connection settings
	ServerAddr           string
	ConnectTimeout       time.Duration
	ReconnectInterval    time.Duration
	MaxReconnectAttempts int

	// Message settings
	MessageTimeout    time.Duration
	MaxMessageSize    int
	MessageBufferSize int

	// Health monitoring
	HeartbeatInterval  time.Duration
	HealthCheckTimeout time.Duration

	// Logging
	LogLevel log.Level

	// Custom options
	CustomOptions map[string]interface{}
}

// DefaultClientConfig returns default client configuration
func DefaultClientConfig() Config {
	return Config{
		ServerAddr:           "localhost:8080",
		ConnectTimeout:       30 * time.Second,
		ReconnectInterval:    5 * time.Second,
		MaxReconnectAttempts: 10,
		MessageTimeout:       10 * time.Second,
		MaxMessageSize:       1024 * 1024, // 1MB
		MessageBufferSize:    1000,
		HeartbeatInterval:    30 * time.Second,
		HealthCheckTimeout:   5 * time.Second,
		LogLevel:             log.LevelInfo,
		CustomOptions:        make(map[string]interface{}),
	}
}

// MessageHandler defines a function type for handling incoming messages
type MessageHandler func(msg *protocol.Message) error

// EventHandler defines a function type for handling client events
type EventHandler func(event Event) error

// EventType represents different types of client events
type EventType string

const (
	EventTypeConnected    EventType = "connected"
	EventTypeDisconnected EventType = "disconnected"
	EventTypeReconnecting EventType = "reconnecting"
	EventTypeError        EventType = "error"
	EventTypeJoinedGroup  EventType = "joined_group"
	EventTypeLeftGroup    EventType = "left_group"
	EventTypeJoinedScope  EventType = "joined_scope"
	EventTypeLeftScope    EventType = "left_scope"
)

// Event represents a client event
type Event struct {
	Type      EventType
	Timestamp time.Time
	Data      map[string]interface{}
	Error     error
}

// NewClient creates a new ZeuSync client
func NewClient(config Config) *Client {
	if config.CustomOptions == nil {
		config.CustomOptions = make(map[string]interface{})
	}

	logger := log.New(config.LogLevel)

	client := &Client{
		id:              protocol.GenerateClientID(),
		messageHandlers: make(map[protocol.MessageType][]MessageHandler),
		eventHandlers:   make(map[EventType][]EventHandler),
		done:            make(chan struct{}),
		config:          config,
		logger:          logger.With(log.String("component", "client")),
	}

	client.logger.Info("Client created", log.String("client_id", string(client.id)))

	return client
}

// Connect establishes connection to the ZeuSync server
func (c *Client) Connect(ctx context.Context) error {
	if atomic.LoadInt32(&c.closed) == 1 {
		return ErrClientClosed
	}

	if atomic.LoadInt32(&c.connected) == 1 {
		return ErrAlreadyConnected
	}

	c.logger.Info("Connecting to server", log.String("addr", c.config.ServerAddr))

	// Create QUIC transport
	globalConfig := protocol.DefaultGlobalConfig()
	globalConfig.MaxMessageSize = c.config.MaxMessageSize
	globalConfig.MessageBufferSize = c.config.MessageBufferSize

	c.transport = quic.NewQUICTransport(nil, globalConfig, c.logger)

	// Connect with timeout
	connectCtx, cancel := context.WithTimeout(ctx, c.config.ConnectTimeout)
	defer cancel()

	conn, err := c.transport.Dial(connectCtx, c.config.ServerAddr)
	if err != nil {
		c.logger.Error("Failed to connect to server",
			log.String("addr", c.config.ServerAddr),
			log.Error(err))
		return err
	}

	c.conn = conn
	atomic.StoreInt32(&c.connected, 1)

	c.logger.Info("Connected to server",
		log.String("local_addr", conn.LocalAddr().String()),
		log.String("remote_addr", conn.RemoteAddr().String()))

	// Start background workers
	c.startWorkers()

	// Emit connected event
	c.emitEvent(Event{
		Type:      EventTypeConnected,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"server_addr": c.config.ServerAddr,
			"local_addr":  conn.LocalAddr().String(),
			"remote_addr": conn.RemoteAddr().String(),
		},
	})

	return nil
}

// Disconnect closes the connection to the server
func (c *Client) Disconnect() error {
	if !atomic.CompareAndSwapInt32(&c.connected, 1, 0) {
		return ErrNotConnected
	}

	c.logger.Info("Disconnecting from server")

	// Close connection
	if c.conn != nil {
		_ = c.conn.Close()
	}

	// Stop workers
	c.stopWorkers()

	// Emit disconnected event
	c.emitEvent(Event{
		Type:      EventTypeDisconnected,
		Timestamp: time.Now(),
	})

	c.logger.Info("Disconnected from server")

	return nil
}

// Close closes the client and releases all resources
func (c *Client) Close() error {
	if !atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		return nil // Already closed
	}

	c.logger.Info("Closing client")

	// Disconnect if connected
	if atomic.LoadInt32(&c.connected) == 1 {
		_ = c.Disconnect()
	}

	// Close transport
	if c.transport != nil {
		_ = c.transport.Close()
	}

	// Signal done
	select {
	case <-c.done:
	default:
		close(c.done)
	}

	c.logger.Info("Client closed")

	return nil
}

// SendMessage sends a message to the server
func (c *Client) SendMessage(msg *protocol.Message) error {
	if atomic.LoadInt32(&c.connected) == 0 {
		return ErrNotConnected
	}

	if atomic.LoadInt32(&c.closed) == 1 {
		return ErrClientClosed
	}

	// Set source client ID
	msg.SourceClientID = c.id

	c.logger.Debug("Sending message",
		log.String("message_id", string(msg.ID)),
		log.String("type", msg.Type.String()))

	return c.conn.SendMessage(msg)
}

// SendMessageWithTimeout sends a message with a timeout
func (c *Client) SendMessageWithTimeout(msg *protocol.Message, timeout time.Duration) error {
	if atomic.LoadInt32(&c.connected) == 0 {
		return ErrNotConnected
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	errChan := c.conn.SendMessageAsync(msg)

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// SendString sends a string message
func (c *Client) SendString(msgType protocol.MessageType, text string) error {
	msg := protocol.NewMessageFromString(msgType, text)
	return c.SendMessage(msg)
}

// SendJSON sends a JSON message
func (c *Client) SendJSON(msgType protocol.MessageType, data interface{}) error {
	msg, err := protocol.NewMessageFromJSON(msgType, data)
	if err != nil {
		return err
	}
	return c.SendMessage(msg)
}

// SendBytes sends a binary message
func (c *Client) SendBytes(msgType protocol.MessageType, data []byte) error {
	msg := protocol.NewMessageFromBytes(msgType, data)
	return c.SendMessage(msg)
}

// JoinGroup joins a group by name
func (c *Client) JoinGroup(groupName string) error {
	if atomic.LoadInt32(&c.connected) == 0 {
		return ErrNotConnected
	}

	c.logger.Info("Joining group", log.String("group", groupName))

	// Send join group message
	builder := protocol.NewMessageBuilder(c.logger)
	msg := builder.
		WithType(protocol.MessageTypeControl).
		WithJSON(map[string]interface{}{
			"action": "join_group",
			"group":  groupName,
		}).
		Build()

	err := c.SendMessage(msg)
	if err != nil {
		c.logger.Error("Failed to send join group message", log.Error(err))
		return err
	}

	// Store group membership (will be confirmed by server response)
	groupID := protocol.GroupID(groupName) // Simplified for example
	c.groups.Store(groupID, true)

	// Emit event
	c.emitEvent(Event{
		Type:      EventTypeJoinedGroup,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"group": groupName,
		},
	})

	return nil
}

// LeaveGroup leaves a group by name
func (c *Client) LeaveGroup(groupName string) error {
	if atomic.LoadInt32(&c.connected) == 0 {
		return ErrNotConnected
	}

	c.logger.Info("Leaving group", log.String("group", groupName))

	// Send leave group message
	builder := protocol.NewMessageBuilder(c.logger)
	msg := builder.
		WithType(protocol.MessageTypeControl).
		WithJSON(map[string]interface{}{
			"action": "leave_group",
			"group":  groupName,
		}).
		Build()

	err := c.SendMessage(msg)
	if err != nil {
		c.logger.Error("Failed to send leave group message", log.Error(err))
		return err
	}

	// Remove group membership
	groupID := protocol.GroupID(groupName)
	c.groups.Delete(groupID)

	// Emit event
	c.emitEvent(Event{
		Type:      EventTypeLeftGroup,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"group": groupName,
		},
	})

	return nil
}

// SubscribeToScope subscribes to a data scope
func (c *Client) SubscribeToScope(scopeName string) error {
	if atomic.LoadInt32(&c.connected) == 0 {
		return ErrNotConnected
	}

	c.logger.Info("Subscribing to scope", log.String("scope", scopeName))

	// Send subscribe message
	builder := protocol.NewMessageBuilder(c.logger)
	msg := builder.
		WithType(protocol.MessageTypeControl).
		WithJSON(map[string]interface{}{
			"action": "subscribe_scope",
			"scope":  scopeName,
		}).
		Build()

	err := c.SendMessage(msg)
	if err != nil {
		c.logger.Error("Failed to send subscribe scope message", log.Error(err))
		return err
	}

	// Store scope subscription
	scopeID := protocol.ScopeID(scopeName)
	c.scopes.Store(scopeID, true)

	// Emit event
	c.emitEvent(Event{
		Type:      EventTypeJoinedScope,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"scope": scopeName,
		},
	})

	return nil
}

// UnsubscribeFromScope unsubscribes from a data scope
func (c *Client) UnsubscribeFromScope(scopeName string) error {
	if atomic.LoadInt32(&c.connected) == 0 {
		return ErrNotConnected
	}

	c.logger.Info("Unsubscribing from scope", log.String("scope", scopeName))

	// Send unsubscribe message
	builder := protocol.NewMessageBuilder(c.logger)
	msg := builder.
		WithType(protocol.MessageTypeControl).
		WithJSON(map[string]interface{}{
			"action": "unsubscribe_scope",
			"scope":  scopeName,
		}).
		Build()

	err := c.SendMessage(msg)
	if err != nil {
		c.logger.Error("Failed to send unsubscribe scope message", log.Error(err))
		return err
	}

	// Remove scope subscription
	scopeID := protocol.ScopeID(scopeName)
	c.scopes.Delete(scopeID)

	// Emit event
	c.emitEvent(Event{
		Type:      EventTypeLeftScope,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"scope": scopeName,
		},
	})

	return nil
}

// OnMessage registers a message handler for a specific message type
func (c *Client) OnMessage(msgType protocol.MessageType, handler MessageHandler) {
	c.handlerMutex.Lock()
	defer c.handlerMutex.Unlock()

	c.messageHandlers[msgType] = append(c.messageHandlers[msgType], handler)
	c.logger.Debug("Message handler registered", log.String("type", string(msgType)))
}

// OnEvent registers an event handler for a specific event type
func (c *Client) OnEvent(eventType EventType, handler EventHandler) {
	c.handlerMutex.Lock()
	defer c.handlerMutex.Unlock()

	c.eventHandlers[eventType] = append(c.eventHandlers[eventType], handler)
	c.logger.Debug("Event handler registered", log.String("type", string(eventType)))
}

// SetMetadata sets client metadata
func (c *Client) SetMetadata(key string, value interface{}) {
	c.metadata.Store(key, value)
	c.logger.Debug("Metadata set", log.String("key", key))
}

// GetMetadata gets client metadata
func (c *Client) GetMetadata(key string) (interface{}, bool) {
	return c.metadata.Load(key)
}

// ID returns the client ID
func (c *Client) ID() protocol.ClientID {
	return c.id
}

// IsConnected returns true if the client is connected
func (c *Client) IsConnected() bool {
	return atomic.LoadInt32(&c.connected) == 1
}

// IsClosed returns true if the client is closed
func (c *Client) IsClosed() bool {
	return atomic.LoadInt32(&c.closed) == 1
}

// ConnectionInfo returns information about the current connection
func (c *Client) ConnectionInfo() (protocol.ConnectionInfo, error) {
	if c.conn == nil {
		return protocol.ConnectionInfo{}, ErrNotConnected
	}
	return c.conn.Info(), nil
}

// Ping sends a ping to the server and returns the latency
func (c *Client) Ping() (time.Duration, error) {
	if atomic.LoadInt32(&c.connected) == 0 {
		return 0, ErrNotConnected
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.config.HealthCheckTimeout)
	defer cancel()

	return c.conn.Ping(ctx)
}

// startWorkers starts background worker goroutines
func (c *Client) startWorkers() {
	c.workerGroup.Add(3)

	// Message receiver
	go func() {
		defer c.workerGroup.Done()
		c.messageReceiver()
	}()

	// Health monitor
	go func() {
		defer c.workerGroup.Done()
		c.healthMonitor()
	}()

	// Reconnection handler
	go func() {
		defer c.workerGroup.Done()
		c.reconnectionHandler()
	}()
}

// stopWorkers stops background worker goroutines
func (c *Client) stopWorkers() {
	// Workers will stop when connection is closed or client is done
	c.workerGroup.Wait()
}

// messageReceiver handles incoming messages
func (c *Client) messageReceiver() {
	c.logger.Debug("Message receiver started")

	for atomic.LoadInt32(&c.connected) == 1 && atomic.LoadInt32(&c.closed) == 0 {
		ctx, cancel := context.WithTimeout(context.Background(), c.config.MessageTimeout)
		msg, err := c.conn.ReceiveMessage(ctx)
		cancel()

		if err != nil {
			if atomic.LoadInt32(&c.connected) == 1 {
				c.logger.Error("Failed to receive message", log.Error(err))
			}
			continue
		}

		if msg != nil {
			c.handleMessage(msg)
		}
	}

	c.logger.Debug("Message receiver stopped")
}

// handleMessage processes an incoming message
func (c *Client) handleMessage(msg *protocol.Message) {
	c.logger.Debug("Handling message",
		log.String("message_id", string(msg.ID)),
		log.String("type", string(msg.Type)))

	c.handlerMutex.RLock()
	handlers := c.messageHandlers[msg.Type]
	c.handlerMutex.RUnlock()

	for _, handler := range handlers {
		go func(h MessageHandler) {
			if err := h(msg); err != nil {
				c.logger.Error("Message handler error", log.Error(err))
			}
		}(handler)
	}

	msg.Release()
}

// healthMonitor monitors connection health
func (c *Client) healthMonitor() {
	c.logger.Debug("Health monitor started")

	ticker := time.NewTicker(c.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if atomic.LoadInt32(&c.connected) == 1 {
				_, err := c.Ping()
				if err != nil {
					c.logger.Warn("Health check failed", log.Error(err))
					// Connection will be handled by reconnection handler
				}
			}
		case <-c.done:
			c.logger.Debug("Health monitor stopped")
			return
		}
	}
}

// reconnectionHandler handles automatic reconnection
func (c *Client) reconnectionHandler() {
	c.logger.Debug("Reconnection handler started")

	for atomic.LoadInt32(&c.closed) == 0 {
		select {
		case <-c.conn.Done():
			if atomic.LoadInt32(&c.closed) == 1 {
				return
			}

			c.logger.Warn("Connection lost, attempting to reconnect")
			atomic.StoreInt32(&c.connected, 0)

			// Emit reconnecting event
			c.emitEvent(Event{
				Type:      EventTypeReconnecting,
				Timestamp: time.Now(),
			})

			// Attempt reconnection
			for attempt := 1; attempt <= c.config.MaxReconnectAttempts; attempt++ {
				c.logger.Info("Reconnection attempt", log.Int("attempt", attempt))

				ctx, cancel := context.WithTimeout(context.Background(), c.config.ConnectTimeout)
				err := c.Connect(ctx)
				cancel()

				if err == nil {
					c.logger.Info("Reconnected successfully")
					break
				}

				c.logger.Error("Reconnection failed",
					log.Int("attempt", attempt),
					log.Error(err))

				if attempt < c.config.MaxReconnectAttempts {
					time.Sleep(c.config.ReconnectInterval)
				}
			}

		case <-c.done:
			c.logger.Debug("Reconnection handler stopped")
			return
		}
	}
}

// emitEvent emits an event to registered handlers
func (c *Client) emitEvent(event Event) {
	c.handlerMutex.RLock()
	handlers := c.eventHandlers[event.Type]
	c.handlerMutex.RUnlock()

	for _, handler := range handlers {
		go func(h EventHandler) {
			if err := h(event); err != nil {
				c.logger.Error("Event handler error", log.Error(err))
			}
		}(handler)
	}
}
