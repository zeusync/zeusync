package protocol

import (
	"context"
	"net"
	"sync"
)

// --- Core Abstractions ---

// Engine is the heart of the server, managing transports, clients, and groups.
type Engine interface {
	// Start begins listening for connections on the specified transport.
	Start(ctx context.Context, transport Transport) error

	// Stop gracefully shuts down the engine.
	Stop(ctx context.Context) error

	// OnConnect sets a handler to be called when a new client connects.
	OnConnect(handler func(client Client))

	// OnDisconnect sets a handler to be called when a client disconnects.
	OnDisconnect(handler func(client Client, reason error))

	// Group returns a group by its ID. If the group doesn't exist, it's created.
	Group(id string) Group

	// Client returns a client by its ID.
	Client(id string) (Client, bool)
}

// Transport defines the interface for a network protocol (e.g., QUIC, WebSocket).
type Transport interface {
	// Listen starts the transport on the given address.
	Listen(ctx context.Context, address string) error

	// Accept waits for and returns the next incoming connection.
	Accept(ctx context.Context) (Connection, error)

	// Dial connects to a remote address.
	Dial(ctx context.Context, address string) (Connection, error)

	// Close terminates the transport.
	Close() error
}

// Connection represents a single, raw network connection.
type Connection interface {
	// Read reads raw data from the connection.
	Read(ctx context.Context) ([]byte, error)

	// Write writes raw data to the connection.
	Write(ctx context.Context, data []byte) error

	// Close closes the connection.
	Close() error

	// LocalAddr returns the local network address.
	LocalAddr() net.Addr

	// RemoteAddr returns the remote network address.
	RemoteAddr() net.Addr
}

// --- Client and Data Scoping ---

// Client represents a connected user.
// It's a high-level abstraction over a connection.
type Client interface {
	// ID returns the unique identifier for the client.
	ID() string

	// Send sends a message directly to the client.
	Send(msg Message) error

	// Receive waits for and returns the next message from the client.
	Receive(ctx context.Context) (Message, error)

	// Scope returns a new or existing data scope for this client.
	// Scopes are used to manage and synchronize specific subsets of data.
	Scope(id string) Scope

	// Scopes returns all scopes associated with this client.
	Scopes() []Scope

	// Close terminates the client's connection.
	Close() error

	// Context returns a context that is cancelled when the client disconnects.
	Context() context.Context
}

// Scope represents a specific data context for a client.
// It allows for fine-grained data synchronization.
type Scope interface {
	// ID returns the unique identifier for the scope.
	ID() string

	// Update sends a message to the client, indicating a change within this scope.
	// This is the primary way to synchronize state.
	Update(msg Message) error

	// Client returns the parent client of this scope.
	Client() Client
}

// --- Grouping and Broadcasting ---

// Subscriber is an interface that can be added to a group.
// Both Client and Scope implement this interface.
type Subscriber interface {
	// Send sends a message to the subscriber.
	Send(msg Message) error
}

// Group is a collection of subscribers for broadcasting messages.
type Group interface {
	// ID returns the unique identifier for the group.
	ID() string

	// Add adds a subscriber to the group.
	Add(sub Subscriber) error

	// Remove removes a subscriber from the group.
	Remove(sub Subscriber) error

	// Broadcast sends a message to all subscribers in the group.
	Broadcast(msg Message)

	// Len returns the number of subscribers in the group.
	Len() int
}

// --- Messaging and Serialization ---

// MessageType is a byte identifier for the message type.
type MessageType byte

// Message is the fundamental data unit exchanged over the network.
type Message interface {
	// Type returns the message type identifier.
	Type() MessageType

	// Payload returns the raw data of the message.
	Payload() []byte
}

// Codec defines the contract for message serialization and deserialization.
type Codec interface {
	// Encode converts a Message into a byte slice.
	Encode(msg Message) ([]byte, error)

	// Decode converts a byte slice back into a Message.
	Decode(data []byte) (Message, error)
}

// --- Concurrency Primitives ---

type SyncMap[K comparable, V any] struct {
	mu   sync.RWMutex
	data map[K]V
}

func NewSyncMap[K comparable, V any]() *SyncMap[K, V] {
	return &SyncMap[K, V]{data: make(map[K]V)}
}

func (m *SyncMap[K, V]) Get(key K) (V, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	val, ok := m.data[key]
	return val, ok
}

func (m *SyncMap[K, V]) Set(key K, value V) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value
}

func (m *SyncMap[K, V]) Delete(key K) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
}

func (m *SyncMap[K, V]) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.data)
}

func (m *SyncMap[K, V]) Range(f func(key K, value V) bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for k, v := range m.data {
		if !f(k, v) {
			break
		}
	}
}
