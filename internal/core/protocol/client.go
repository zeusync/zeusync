package protocol

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// Client represents a connected client with associated connection and state
type Client interface {
	// Identity

	ID() ClientID
	ConnectionID() ConnectionID
	Connection() Connection

	// State management

	Info() ClientInfo
	Metadata() map[string]interface{}
	SetMetadata(key string, value interface{})

	// Messaging

	SendMessage(msg *Message) error
	SendMessageAsync(msg *Message) <-chan error
	Broadcast(msg *Message, filter MessageFilter) error

	// Groups

	JoinGroup(groupID GroupID) error
	LeaveGroup(groupID GroupID) error
	Groups() []GroupID
	IsInGroup(groupID GroupID) bool

	// Scopes

	JoinScope(scopeID ScopeID) error
	LeaveScope(scopeID ScopeID) error
	Scopes() []ScopeID
	IsInScope(scopeID ScopeID) bool

	// Streams

	OpenStream(ctx context.Context) (Stream, error)
	HandleStream(handler StreamHandler) error

	// Health and activity

	Ping(ctx context.Context) (time.Duration, error)
	LastSeen() time.Time
	IsActive() bool

	// Lifecycle

	Disconnect() error
	Done() <-chan struct{}
}

// ClientConfig holds configuration for clients
type ClientConfig struct {
	MaxGroups         int
	MaxScopes         int
	MaxStreams        int
	HeartbeatInterval time.Duration
	InactivityTimeout time.Duration
	MessageQueueSize  int
	EnableCompression bool
	CompressionLevel  int
}

// DefaultClientConfig returns default client configuration
func DefaultClientConfig() ClientConfig {
	return ClientConfig{
		MaxGroups:         100,
		MaxScopes:         50,
		MaxStreams:        10,
		HeartbeatInterval: 30 * time.Second,
		InactivityTimeout: 5 * time.Minute,
		MessageQueueSize:  1000,
		EnableCompression: true,
		CompressionLevel:  1,
	}
}

// BaseClient provides common client functionality
type BaseClient struct {
	id           ClientID
	connection   Connection
	connectedAt  time.Time
	lastSeen     int64    // atomic unix timestamp
	groups       sync.Map // map[GroupID]bool
	scopes       sync.Map // map[ScopeID]bool
	metadata     sync.Map // map[string]interface{}
	messageQueue chan *Message
	done         chan struct{}
	config       ClientConfig
	active       int32 // atomic bool
}

// NewBaseClient creates a new base client
func NewBaseClient(id ClientID, conn Connection, config ClientConfig) *BaseClient {
	client := &BaseClient{
		id:           id,
		connection:   conn,
		connectedAt:  time.Now(),
		lastSeen:     time.Now().Unix(),
		messageQueue: make(chan *Message, config.MessageQueueSize),
		done:         make(chan struct{}),
		config:       config,
		active:       1,
	}

	// Start message processing goroutine
	go client.processMessages()

	return client
}

// ID returns the client ID
func (c *BaseClient) ID() ClientID {
	return c.id
}

// ConnectionID returns the connection ID
func (c *BaseClient) ConnectionID() ConnectionID {
	return c.connection.ID()
}

// Connection returns the underlying connection
func (c *BaseClient) Connection() Connection {
	return c.connection
}

// Info returns client information
func (c *BaseClient) Info() ClientInfo {
	groups := make([]GroupID, 0)
	c.groups.Range(func(key, value interface{}) bool {
		groups = append(groups, key.(GroupID))
		return true
	})

	scopes := make([]ScopeID, 0)
	c.scopes.Range(func(key, value interface{}) bool {
		scopes = append(scopes, key.(ScopeID))
		return true
	})

	metadata := make(map[string]interface{})
	c.metadata.Range(func(key, value interface{}) bool {
		metadata[key.(string)] = value
		return true
	})

	return ClientInfo{
		ID:           c.id,
		ConnectionID: c.connection.ID(),
		Groups:       groups,
		Scopes:       scopes,
		ConnectedAt:  c.connectedAt,
		LastSeen:     time.Unix(atomic.LoadInt64(&c.lastSeen), 0),
		Metadata:     metadata,
	}
}

// Metadata returns a copy of client metadata
func (c *BaseClient) Metadata() map[string]interface{} {
	metadata := make(map[string]interface{})
	c.metadata.Range(func(key, value interface{}) bool {
		metadata[key.(string)] = value
		return true
	})
	return metadata
}

// SetMetadata sets a metadata key-value pair
func (c *BaseClient) SetMetadata(key string, value interface{}) {
	c.metadata.Store(key, value)
}

// SendMessage sends a message to the client
func (c *BaseClient) SendMessage(msg *Message) error {
	if !c.IsActive() {
		return ErrClientInactive
	}

	msg.TargetClientID = c.id
	return c.connection.SendMessage(msg)
}

// SendMessageAsync sends a message asynchronously
func (c *BaseClient) SendMessageAsync(msg *Message) <-chan error {
	errChan := make(chan error, 1)

	if !c.IsActive() {
		errChan <- ErrClientInactive
		return errChan
	}

	go func() {
		defer close(errChan)
		msg.TargetClientID = c.id
		errChan <- c.connection.SendMessage(msg)
	}()

	return errChan
}

// JoinGroup adds the client to a group
func (c *BaseClient) JoinGroup(groupID GroupID) error {
	if !c.IsActive() {
		return ErrClientInactive
	}

	// Check group limit
	groupCount := 0
	c.groups.Range(func(key, value interface{}) bool {
		groupCount++
		return true
	})

	if groupCount >= c.config.MaxGroups {
		return ErrMaxGroupsExceeded
	}

	c.groups.Store(groupID, true)
	return nil
}

// LeaveGroup removes the client from a group
func (c *BaseClient) LeaveGroup(groupID GroupID) error {
	c.groups.Delete(groupID)
	return nil
}

// Groups returns all groups the client belongs to
func (c *BaseClient) Groups() []GroupID {
	groups := make([]GroupID, 0)
	c.groups.Range(func(key, value interface{}) bool {
		groups = append(groups, key.(GroupID))
		return true
	})
	return groups
}

// IsInGroup checks if the client is in a specific group
func (c *BaseClient) IsInGroup(groupID GroupID) bool {
	_, exists := c.groups.Load(groupID)
	return exists
}

// JoinScope adds the client to a scope
func (c *BaseClient) JoinScope(scopeID ScopeID) error {
	if !c.IsActive() {
		return ErrClientInactive
	}

	// Check scope limit
	scopeCount := 0
	c.scopes.Range(func(key, value interface{}) bool {
		scopeCount++
		return true
	})

	if scopeCount >= c.config.MaxScopes {
		return ErrMaxScopesExceeded
	}

	c.scopes.Store(scopeID, true)
	return nil
}

// LeaveScope removes the client from a scope
func (c *BaseClient) LeaveScope(scopeID ScopeID) error {
	c.scopes.Delete(scopeID)
	return nil
}

// Scopes returns all scopes the client belongs to
func (c *BaseClient) Scopes() []ScopeID {
	scopes := make([]ScopeID, 0)
	c.scopes.Range(func(key, value interface{}) bool {
		scopes = append(scopes, key.(ScopeID))
		return true
	})
	return scopes
}

// IsInScope checks if the client is in a specific scope
func (c *BaseClient) IsInScope(scopeID ScopeID) bool {
	_, exists := c.scopes.Load(scopeID)
	return exists
}

// OpenStream opens a new stream with the client
func (c *BaseClient) OpenStream(ctx context.Context) (Stream, error) {
	if !c.IsActive() {
		return nil, ErrClientInactive
	}
	return c.connection.OpenStream(ctx)
}

// Ping sends a ping to the client and measures latency
func (c *BaseClient) Ping(ctx context.Context) (time.Duration, error) {
	if !c.IsActive() {
		return 0, ErrClientInactive
	}
	return c.connection.Ping(ctx)
}

// LastSeen returns the last activity time
func (c *BaseClient) LastSeen() time.Time {
	return time.Unix(atomic.LoadInt64(&c.lastSeen), 0)
}

// UpdateLastSeen updates the last seen timestamp
func (c *BaseClient) UpdateLastSeen() {
	atomic.StoreInt64(&c.lastSeen, time.Now().Unix())
}

// IsActive checks if the client is active
func (c *BaseClient) IsActive() bool {
	return atomic.LoadInt32(&c.active) == 1
}

// Disconnect disconnects the client
func (c *BaseClient) Disconnect() error {
	atomic.StoreInt32(&c.active, 0)

	select {
	case <-c.done:
	default:
		close(c.done)
	}

	return c.connection.Close()
}

// Done returns a channel that's closed when the client is disconnected
func (c *BaseClient) Done() <-chan struct{} {
	return c.done
}

// processMessages processes queued messages
func (c *BaseClient) processMessages() {
	for {
		select {
		case msg := <-c.messageQueue:
			if msg != nil {
				_ = c.connection.SendMessage(msg)
				msg.Release()
			}
		case <-c.done:
			return
		}
	}
}

// ClientManager manages multiple clients
type ClientManager interface {
	// Client lifecycle

	AddClient(client Client) error
	RemoveClient(id ClientID) error
	GetClient(id ClientID) (Client, bool)

	// Client queries

	ListClients() []Client
	ClientCount() int
	ActiveClients() []Client

	// Group operations

	GetClientsInGroup(groupID GroupID) []Client
	BroadcastToGroup(groupID GroupID, msg *Message, filter MessageFilter) error

	// Scope operations

	GetClientsInScope(scopeID ScopeID) []Client
	BroadcastToScope(scopeID ScopeID, msg *Message, filter MessageFilter) error

	// Health monitoring

	HealthCheck(ctx context.Context) []HealthCheck

	// Cleanup

	Close() error
}
