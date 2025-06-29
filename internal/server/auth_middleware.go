package server

import (
	"context"
	"fmt"

	"github.com/zeusync/zeusync/internal/core/protocol/intrefaces"
)

// SimpleAuthMiddleware is a simple authentication middleware for demonstration purposes.
// In a real application, you would use a more robust authentication mechanism.
type SimpleAuthMiddleware struct{}

func (m *SimpleAuthMiddleware) Name() string {
	return "SimpleAuthMiddleware"
}

func (m *SimpleAuthMiddleware) Priority() uint16 {
	return 100
}

func (m *SimpleAuthMiddleware) BeforeHandle(ctx context.Context, client intrefaces.ClientInfo, message intrefaces.Message) error {
	// For WebSocket, authentication happens at connection time.
	return nil
}

func (m *SimpleAuthMiddleware) AfterHandle(ctx context.Context, client intrefaces.ClientInfo, message intrefaces.Message, response intrefaces.Message, err error) error {
	return nil
}

func (m *SimpleAuthMiddleware) OnConnect(ctx context.Context, client intrefaces.ClientInfo) error {
	// In a real app, you'd get the token from the request (e.g., headers or query params).
	// For now, we'll assume a token is passed in the client's metadata.
	token, ok := client.Metadata["token"].(string)
	if !ok || token != "supersecrettoken" {
		return fmt.Errorf("unauthorized")
	}

	client.IsAuthenticated = true
	client.UserID = "user_" + client.ID // Assign a simple user ID
	return nil
}

func (m *SimpleAuthMiddleware) OnDisconnect(ctx context.Context, client intrefaces.ClientInfo, reason string) error {
	return nil
}
