package protocol

import (
	"sync"
	"sync/atomic"
	"time"
)

// Group represents a collection of clients that can receive broadcast messages
type Group interface {
	// Identity

	ID() GroupID
	Name() string
	Info() GroupInfo

	// Client management

	AddClient(clientID ClientID) error
	RemoveClient(clientID ClientID) error
	HasClient(clientID ClientID) bool
	Clients() []ClientID
	ClientCount() int

	// Messaging

	Broadcast(msg *Message, filter MessageFilter) error
	BroadcastAsync(msg *Message, filter MessageFilter) <-chan error

	// Metadata

	Metadata() map[string]interface{}
	SetMetadata(key string, value interface{})

	// Lifecycle

	Created() time.Time
	Close() error
}

// GroupConfig holds configuration for groups
type GroupConfig struct {
	MaxClients       int
	BroadcastTimeout time.Duration
	EnableMetrics    bool
	Persistent       bool
}

// DefaultGroupConfig returns default group configuration
func DefaultGroupConfig() GroupConfig {
	return GroupConfig{
		MaxClients:       10000,
		BroadcastTimeout: 5 * time.Second,
		EnableMetrics:    true,
		Persistent:       false,
	}
}

// BaseGroup provides common group functionality
type BaseGroup struct {
	id          GroupID
	name        string
	clients     sync.Map // map[ClientID]bool
	clientCount int64    // atomic
	metadata    sync.Map // map[string]interface{}
	createdAt   time.Time
	config      GroupConfig
	closed      int32 // atomic bool
}

// NewBaseGroup creates a new base group
func NewBaseGroup(id GroupID, name string, config GroupConfig) *BaseGroup {
	return &BaseGroup{
		id:        id,
		name:      name,
		createdAt: time.Now(),
		config:    config,
	}
}

// ID returns the group ID
func (g *BaseGroup) ID() GroupID {
	return g.id
}

// Name returns the group name
func (g *BaseGroup) Name() string {
	return g.name
}

// Info returns group information
func (g *BaseGroup) Info() GroupInfo {
	metadata := make(map[string]interface{})
	g.metadata.Range(func(key, value interface{}) bool {
		metadata[key.(string)] = value
		return true
	})

	return GroupInfo{
		ID:          g.id,
		Name:        g.name,
		ClientCount: int(atomic.LoadInt64(&g.clientCount)),
		CreatedAt:   g.createdAt,
		Metadata:    metadata,
	}
}

// AddClient adds a client to the group
func (g *BaseGroup) AddClient(clientID ClientID) error {
	if atomic.LoadInt32(&g.closed) == 1 {
		return ErrGroupClosed
	}

	if g.config.MaxClients > 0 && int(atomic.LoadInt64(&g.clientCount)) >= g.config.MaxClients {
		return ErrMaxClientsExceeded
	}

	if _, loaded := g.clients.LoadOrStore(clientID, true); !loaded {
		atomic.AddInt64(&g.clientCount, 1)
	}

	return nil
}

// RemoveClient removes a client from the group
func (g *BaseGroup) RemoveClient(clientID ClientID) error {
	if _, loaded := g.clients.LoadAndDelete(clientID); loaded {
		atomic.AddInt64(&g.clientCount, -1)
	}
	return nil
}

// HasClient checks if a client is in the group
func (g *BaseGroup) HasClient(clientID ClientID) bool {
	_, exists := g.clients.Load(clientID)
	return exists
}

// Clients returns all client IDs in the group
func (g *BaseGroup) Clients() []ClientID {
	clients := make([]ClientID, 0, atomic.LoadInt64(&g.clientCount))
	g.clients.Range(func(key, value interface{}) bool {
		clients = append(clients, key.(ClientID))
		return true
	})
	return clients
}

// ClientCount returns the number of clients in the group
func (g *BaseGroup) ClientCount() int {
	return int(atomic.LoadInt64(&g.clientCount))
}

// Metadata returns a copy of group metadata
func (g *BaseGroup) Metadata() map[string]interface{} {
	metadata := make(map[string]interface{})
	g.metadata.Range(func(key, value interface{}) bool {
		metadata[key.(string)] = value
		return true
	})
	return metadata
}

// SetMetadata sets a metadata key-value pair
func (g *BaseGroup) SetMetadata(key string, value interface{}) {
	g.metadata.Store(key, value)
}

// Created returns the group creation time
func (g *BaseGroup) Created() time.Time {
	return g.createdAt
}

// Close closes the group
func (g *BaseGroup) Close() error {
	atomic.StoreInt32(&g.closed, 1)
	return nil
}

// IsClosed checks if the group is closed
func (g *BaseGroup) IsClosed() bool {
	return atomic.LoadInt32(&g.closed) == 1
}

// GroupManager manages multiple groups
type GroupManager interface {
	// Group lifecycle

	CreateGroup(name string, config GroupConfig) (Group, error)
	GetGroup(id GroupID) (Group, bool)
	GetGroupByName(name string) (Group, bool)
	DeleteGroup(id GroupID) error

	// Group queries

	ListGroups() []Group
	GroupCount() int

	// Client operations

	AddClientToGroup(clientID ClientID, groupID GroupID) error
	RemoveClientFromGroup(clientID ClientID, groupID GroupID) error
	GetClientGroups(clientID ClientID) []GroupID

	// Broadcasting

	BroadcastToGroup(groupID GroupID, msg *Message, filter MessageFilter) error
	BroadcastToGroups(groupIDs []GroupID, msg *Message, filter MessageFilter) error

	// Cleanup

	Close() error
}

// Scope represents a data context that clients can subscribe to
type Scope interface {
	// Identity

	ID() ScopeID
	Name() string
	Info() ScopeInfo

	// Client management

	AddClient(clientID ClientID) error
	RemoveClient(clientID ClientID) error
	HasClient(clientID ClientID) bool
	Clients() []ClientID
	ClientCount() int

	// Data management

	SetData(key string, data []byte) error
	GetData(key string) ([]byte, bool)
	DeleteData(key string) error
	ListKeys() []string
	DataSize() uint64

	// Data streaming

	StreamData(clientID ClientID, key string) error
	StreamAllData(clientID ClientID) error

	// Metadata

	Metadata() map[string]interface{}
	SetMetadata(key string, value interface{})

	// Lifecycle

	Created() time.Time
	Close() error
}

// ScopeConfig holds configuration for scopes
type ScopeConfig struct {
	MaxClients      int
	MaxDataSize     uint64
	MaxKeys         int
	DataTTL         time.Duration
	EnableStreaming bool
	Persistent      bool
}

// DefaultScopeConfig returns default scope configuration
func DefaultScopeConfig() ScopeConfig {
	return ScopeConfig{
		MaxClients:      1000,
		MaxDataSize:     100 * 1024 * 1024, // 100MB
		MaxKeys:         10000,
		DataTTL:         0, // No TTL by default
		EnableStreaming: true,
		Persistent:      false,
	}
}

// BaseScope provides common scope functionality
type BaseScope struct {
	id          ScopeID
	name        string
	clients     sync.Map // map[ClientID]bool
	clientCount int64    // atomic
	data        sync.Map // map[string][]byte
	dataSize    uint64   // atomic
	metadata    sync.Map // map[string]interface{}
	createdAt   time.Time
	config      ScopeConfig
	closed      int32 // atomic bool
}

// NewBaseScope creates a new base scope
func NewBaseScope(id ScopeID, name string, config ScopeConfig) *BaseScope {
	return &BaseScope{
		id:        id,
		name:      name,
		createdAt: time.Now(),
		config:    config,
	}
}

// ID returns the scope ID
func (s *BaseScope) ID() ScopeID {
	return s.id
}

// Name returns the scope name
func (s *BaseScope) Name() string {
	return s.name
}

// Info returns scope information
func (s *BaseScope) Info() ScopeInfo {
	metadata := make(map[string]interface{})
	s.metadata.Range(func(key, value interface{}) bool {
		metadata[key.(string)] = value
		return true
	})

	return ScopeInfo{
		ID:          s.id,
		Name:        s.name,
		ClientCount: int(atomic.LoadInt64(&s.clientCount)),
		DataSize:    atomic.LoadUint64(&s.dataSize),
		CreatedAt:   s.createdAt,
		Metadata:    metadata,
	}
}

// AddClient adds a client to the scope
func (s *BaseScope) AddClient(clientID ClientID) error {
	if atomic.LoadInt32(&s.closed) == 1 {
		return ErrScopeClosed
	}

	if s.config.MaxClients > 0 && int(atomic.LoadInt64(&s.clientCount)) >= s.config.MaxClients {
		return ErrMaxClientsExceeded
	}

	if _, loaded := s.clients.LoadOrStore(clientID, true); !loaded {
		atomic.AddInt64(&s.clientCount, 1)
	}

	return nil
}

// RemoveClient removes a client from the scope
func (s *BaseScope) RemoveClient(clientID ClientID) error {
	if _, loaded := s.clients.LoadAndDelete(clientID); loaded {
		atomic.AddInt64(&s.clientCount, -1)
	}
	return nil
}

// HasClient checks if a client is in the scope
func (s *BaseScope) HasClient(clientID ClientID) bool {
	_, exists := s.clients.Load(clientID)
	return exists
}

// Clients returns all client IDs in the scope
func (s *BaseScope) Clients() []ClientID {
	clients := make([]ClientID, 0, atomic.LoadInt64(&s.clientCount))
	s.clients.Range(func(key, value interface{}) bool {
		clients = append(clients, key.(ClientID))
		return true
	})
	return clients
}

// ClientCount returns the number of clients in the scope
func (s *BaseScope) ClientCount() int {
	return int(atomic.LoadInt64(&s.clientCount))
}

// SetData sets data in the scope
func (s *BaseScope) SetData(key string, data []byte) error {
	if atomic.LoadInt32(&s.closed) == 1 {
		return ErrScopeClosed
	}

	// Check data size limits
	newSize := atomic.LoadUint64(&s.dataSize) + uint64(len(data))
	if s.config.MaxDataSize > 0 && newSize > s.config.MaxDataSize {
		return ErrMaxDataSizeExceeded
	}

	// Check key limits
	keyCount := 0
	s.data.Range(func(key, value interface{}) bool {
		keyCount++
		return true
	})

	if s.config.MaxKeys > 0 && keyCount >= s.config.MaxKeys {
		if _, exists := s.data.Load(key); !exists {
			return ErrMaxKeysExceeded
		}
	}

	// Remove old data size if key exists
	if oldData, exists := s.data.Load(key); exists {
		atomic.AddUint64(&s.dataSize, ^uint64(len(oldData.([]byte))-1))
	}

	s.data.Store(key, data)
	atomic.AddUint64(&s.dataSize, uint64(len(data)))

	return nil
}

// GetData gets data from the scope
func (s *BaseScope) GetData(key string) ([]byte, bool) {
	if value, exists := s.data.Load(key); exists {
		return value.([]byte), true
	}
	return nil, false
}

// DeleteData deletes data from the scope
func (s *BaseScope) DeleteData(key string) error {
	if value, loaded := s.data.LoadAndDelete(key); loaded {
		atomic.AddUint64(&s.dataSize, ^uint64(len(value.([]byte))-1))
	}
	return nil
}

// ListKeys returns all data keys in the scope
func (s *BaseScope) ListKeys() []string {
	keys := make([]string, 0)
	s.data.Range(func(key, value interface{}) bool {
		keys = append(keys, key.(string))
		return true
	})
	return keys
}

// DataSize returns the total size of data in the scope
func (s *BaseScope) DataSize() uint64 {
	return atomic.LoadUint64(&s.dataSize)
}

// Metadata returns a copy of scope metadata
func (s *BaseScope) Metadata() map[string]interface{} {
	metadata := make(map[string]interface{})
	s.metadata.Range(func(key, value interface{}) bool {
		metadata[key.(string)] = value
		return true
	})
	return metadata
}

// SetMetadata sets a metadata key-value pair
func (s *BaseScope) SetMetadata(key string, value interface{}) {
	s.metadata.Store(key, value)
}

// Created returns the scope creation time
func (s *BaseScope) Created() time.Time {
	return s.createdAt
}

// Close closes the scope
func (s *BaseScope) Close() error {
	atomic.StoreInt32(&s.closed, 1)
	return nil
}

// IsClosed checks if the scope is closed
func (s *BaseScope) IsClosed() bool {
	return atomic.LoadInt32(&s.closed) == 1
}

// ScopeManager manages multiple scopes
type ScopeManager interface {
	// Scope lifecycle

	CreateScope(name string, config ScopeConfig) (Scope, error)
	GetScope(id ScopeID) (Scope, bool)
	GetScopeByName(name string) (Scope, bool)
	DeleteScope(id ScopeID) error

	// Scope queries

	ListScopes() []Scope
	ScopeCount() int

	// Client operations

	AddClientToScope(clientID ClientID, scopeID ScopeID) error
	RemoveClientFromScope(clientID ClientID, scopeID ScopeID) error
	GetClientScopes(clientID ClientID) []ScopeID

	// Data operations

	SetScopeData(scopeID ScopeID, key string, data []byte) error
	GetScopeData(scopeID ScopeID, key string) ([]byte, bool)
	StreamScopeData(scopeID ScopeID, clientID ClientID, key string) error

	// Cleanup

	Close() error
}
