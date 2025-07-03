package middlewares

import (
	"context"
	"github.com/zeusync/zeusync/internal/core/observability/log"
	"github.com/zeusync/zeusync/internal/core/protocol"
)

// AuthMiddleware handles authentication
type AuthMiddleware struct {
	skipMap map[string]struct{}
	logger  log.Log
}

// Name returns the middleware name
func (m *AuthMiddleware) Name() string {
	return "auth"
}

// Priority returns the middleware priority
func (m *AuthMiddleware) Priority() uint16 {
	return 900 // High priority, but after logging
}

// BeforeHandle checks authentication before message handling
func (m *AuthMiddleware) BeforeHandle(_ context.Context, client protocol.ClientInfo, message *protocol.Message) error {
	// Skip auth for certain message types
	if _, exists := m.skipMap[message.Type()]; exists {
		return nil
	}

	// Skipping types
	if message.Type() == "auth" || message.Type() == "ping" || message.Type() == "heartbeat" {
		return nil
	}

	if !client.IsAuthenticated {
		m.logger.Warn("Unauthenticated client attempted to send message",
			log.String("client_id", client.ID),
			log.String("message_type", message.Type()),
		)
		return nil // Don't fail, just log
	}

	return nil
}

// AfterHandle is called after message handling
func (m *AuthMiddleware) AfterHandle(_ context.Context, _ protocol.ClientInfo, _ *protocol.Message, _ *protocol.Message, _ error) error {
	return nil
}

// OnConnect handles client connection authentication
func (m *AuthMiddleware) OnConnect(_ context.Context, client protocol.ClientInfo) error {
	// In a real implementation, you might check tokens, certificates, etc.
	m.logger.Debug("Client authentication check",
		log.String("client_id", client.ID),
		log.String("remote_addr", client.RemoteAddress),
	)
	return nil
}

// OnDisconnect handles client disconnection
func (m *AuthMiddleware) OnDisconnect(_ context.Context, _ protocol.ClientInfo, _ string) error {
	return nil
}
