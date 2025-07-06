package protocol

import (
	"context"
	"fmt"
	"sync"
)

// --- Engine Implementation ---

// DefaultEngine is the default implementation of the Engine interface.
type DefaultEngine struct {
	clients      *SyncMap[string, Client]
	groups       *SyncMap[string, Group]
	onConnect    func(client Client)
	onDisconnect func(client Client, reason error)
	stopOnce     sync.Once
	codec        Codec
}

// NewEngine creates a new DefaultEngine.
func NewEngine(codec Codec) Engine {
	return &DefaultEngine{
		clients: NewSyncMap[string, Client](),
		groups:  NewSyncMap[string, Group](),
		codec:   codec,
	}
}

// Start begins listening for connections on the specified transport.
func (e *DefaultEngine) Start(ctx context.Context, transport Transport) error {
	go func() {
		for {
			conn, err := transport.Accept(ctx)
			if err != nil {
				// If the context is cancelled, it's a graceful shutdown.
				if ctx.Err() != nil {
					return
				}
				// Handle other errors (e.g., log them)
				continue
			}

			// Create and manage the new client.
			go e.handleConnection(conn)
		}
	}()
	return nil
}

// Stop gracefully shuts down the engine.
func (e *DefaultEngine) Stop(ctx context.Context) error {
	// Implementation for stopping the engine would go here.
	// This would involve closing all transports and clients.
	return nil
}

// OnConnect sets a handler to be called when a new client connects.
func (e *DefaultEngine) OnConnect(handler func(client Client)) {
	e.onConnect = handler
}

// OnDisconnect sets a handler to be called when a client disconnects.
func (e *DefaultEngine) OnDisconnect(handler func(client Client, reason error)) {
	e.onDisconnect = handler
}

// Group returns a group by its ID. If the group doesn't exist, it's created.
func (e *DefaultEngine) Group(id string) Group {
	group, ok := e.groups.Get(id)
	if !ok {
		group = NewDefaultGroup(id)
		e.groups.Set(id, group)
	}
	return group
}

// Client returns a client by its ID.
func (e *DefaultEngine) Client(id string) (Client, bool) {
	return e.clients.Get(id)
}

// handleConnection manages the lifecycle of a new client connection.
func (e *DefaultEngine) handleConnection(conn Connection) {
	// In a real-world scenario, the ID would likely come from an authentication handshake.
	clientID := fmt.Sprintf("client-%s", conn.RemoteAddr().String())
	client := NewDefaultClient(clientID, conn, e.codec)
	e.clients.Set(clientID, client)

	var disconnectReason error
	defer func() {
		e.clients.Delete(clientID)
		if e.onDisconnect != nil {
			e.onDisconnect(client, disconnectReason)
		}
	}()

	if e.onConnect != nil {
		e.onConnect(client)
	}

	// Keep the connection alive by waiting for the client to disconnect.
	<-client.Context().Done()
	disconnectReason = client.Context().Err()
}
