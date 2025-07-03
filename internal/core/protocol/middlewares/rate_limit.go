package middlewares

import (
	"context"
	"github.com/zeusync/zeusync/internal/core/observability/log"
	"github.com/zeusync/zeusync/internal/core/protocol"
	"sync"
	"time"
)

// RateLimitMiddleware implements rate limiting
type RateLimitMiddleware struct {
	logger    log.Log
	rateLimit int           // Messages per window
	window    time.Duration // Time window
	clients   sync.Map      // client ID -> *clientRateLimit
}

type clientRateLimit struct {
	count  int
	window time.Time
	mu     sync.Mutex
}

// Name returns the middleware name
func (m *RateLimitMiddleware) Name() string {
	return "rate_limit"
}

// Priority returns the middleware priority
func (m *RateLimitMiddleware) Priority() uint16 {
	return 800 // After auth
}

// BeforeHandle checks rate limits before message handling
func (m *RateLimitMiddleware) BeforeHandle(_ context.Context, client protocol.ClientInfo, message *protocol.Message) error {
	// Skip rate limiting for certain message types
	if message.Type() == "ping" || message.Type() == "heartbeat" {
		return nil
	}

	now := time.Now()
	clientLimit := m.getClientRateLimit(client.ID)

	clientLimit.mu.Lock()
	defer clientLimit.mu.Unlock()

	// Reset window if expired
	if now.Sub(clientLimit.window) > m.window {
		clientLimit.count = 0
		clientLimit.window = now
	}

	// Check rate limit
	if clientLimit.count >= m.rateLimit {
		m.logger.Warn("Rate limit exceeded",
			log.String("client_id", client.ID),
			log.String("message_type", message.Type()),
			log.Int("count", clientLimit.count),
			log.Int("limit", m.rateLimit),
		)
		return nil // Don't fail, just log
	}

	clientLimit.count++
	return nil
}

// AfterHandle is called after message handling
func (m *RateLimitMiddleware) AfterHandle(_ context.Context, _ protocol.ClientInfo, _ *protocol.Message, response *protocol.Message, err error) error {
	return nil
}

// OnConnect handles client connection
func (m *RateLimitMiddleware) OnConnect(_ context.Context, client protocol.ClientInfo) error {
	// Initialize rate limit for new client
	m.clients.Store(client.ID, &clientRateLimit{
		count:  0,
		window: time.Now(),
	})
	return nil
}

// OnDisconnect handles client disconnection
func (m *RateLimitMiddleware) OnDisconnect(_ context.Context, client protocol.ClientInfo, _ string) error {
	// Clean up rate limit data
	m.clients.Delete(client.ID)
	return nil
}

// getClientRateLimit gets or creates rate limit data for a client
func (m *RateLimitMiddleware) getClientRateLimit(clientID string) *clientRateLimit {
	if limit, exists := m.clients.Load(clientID); exists {
		return limit.(*clientRateLimit)
	}

	limit := &clientRateLimit{
		count:  0,
		window: time.Now(),
	}
	m.clients.Store(clientID, limit)
	return limit
}
