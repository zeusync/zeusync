package protocol

import (
	"context"
	"fmt"
	"sync"
)

// --- Client and Scope Implementations ---

// DefaultClient is the default implementation of the Client interface.
type DefaultClient struct {
	id        string
	conn      Connection
	codec     Codec
	scopes    *SyncMap[string, Scope]
	ctx       context.Context
	cancel    context.CancelFunc
	closeOnce sync.Once
}

// NewDefaultClient creates a new DefaultClient.
func NewDefaultClient(id string, conn Connection, codec Codec) *DefaultClient {
	ctx, cancel := context.WithCancel(context.Background())
	return &DefaultClient{
		id:     id,
		conn:   conn,
		codec:  codec,
		scopes: NewSyncMap[string, Scope](),
		ctx:    ctx,
		cancel: cancel,
	}
}

// ID returns the unique identifier for the client.
func (c *DefaultClient) ID() string {
	return c.id
}

// Send sends a message directly to the client.
func (c *DefaultClient) Send(msg Message) error {
	data, err := c.codec.Encode(msg)
	if err != nil {
		return fmt.Errorf("failed to encode message: %w", err)
	}
	return c.conn.Write(c.ctx, data)
}

// Receive waits for and returns the next message from the client.
func (c *DefaultClient) Receive(ctx context.Context) (Message, error) {
	data, err := c.conn.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read from connection: %w", err)
	}
	return c.codec.Decode(data)
}

// Scope returns a new or existing data scope for this client.
func (c *DefaultClient) Scope(id string) Scope {
	scope, ok := c.scopes.Get(id)
	if !ok {
		scope = NewDefaultScope(id, c)
		c.scopes.Set(id, scope)
	}
	return scope
}

// Scopes returns all scopes associated with this client.
func (c *DefaultClient) Scopes() []Scope {
	var scopes []Scope
	c.scopes.Range(func(_ string, scope Scope) bool {
		scopes = append(scopes, scope)
		return true
	})
	return scopes
}

// Close terminates the client's connection.
func (c *DefaultClient) Close() error {
	c.closeOnce.Do(func() {
		c.cancel()
	})
	return c.conn.Close()
}

// Context returns a context that is cancelled when the client disconnects.
func (c *DefaultClient) Context() context.Context {
	return c.ctx
}

// DefaultScope is the default implementation of the Scope interface.
type DefaultScope struct {
	id     string
	client Client
}

// NewDefaultScope creates a new DefaultScope.
func NewDefaultScope(id string, client Client) *DefaultScope {
	return &DefaultScope{id: id, client: client}
}

// ID returns the unique identifier for the scope.
func (s *DefaultScope) ID() string {
	return s.id
}

// Update sends a message to the client, indicating a change within this scope.
func (s *DefaultScope) Update(msg Message) error {
	// The message could be wrapped here to include scope information if needed.
	return s.client.Send(msg)
}

// Client returns the parent client of this scope.
func (s *DefaultScope) Client() Client {
	return s.client
}

// Send implements the Subscriber interface for Scope.
func (s *DefaultScope) Send(msg Message) error {
	return s.Update(msg)
}
