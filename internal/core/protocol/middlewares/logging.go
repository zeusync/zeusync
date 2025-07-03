package middlewares

import (
	"context"
	"github.com/zeusync/zeusync/internal/core/observability/log"
	"github.com/zeusync/zeusync/internal/core/protocol"
	"time"
)

// LoggingMiddleware logs all protocol events
type LoggingMiddleware struct {
	logger log.Log
}

// Name returns the middleware name
func (m *LoggingMiddleware) Name() string {
	return "logging"
}

// Priority returns the middleware priority
func (m *LoggingMiddleware) Priority() uint16 {
	return 1000 // High priority
}

// BeforeHandle logs before message handling
func (m *LoggingMiddleware) BeforeHandle(_ context.Context, client protocol.ClientInfo, message *protocol.Message) error {
	m.logger.Debug("Processing message",
		log.String("client_id", client.ID),
		log.String("message_type", message.Type()),
		log.String("message_id", message.ID()),
		log.String("remote_addr", client.RemoteAddress),
	)
	return nil
}

// AfterHandle logs after message handling
func (m *LoggingMiddleware) AfterHandle(_ context.Context, client protocol.ClientInfo, message *protocol.Message, _ *protocol.Message, err error) error {
	fields := []log.Field{
		log.String("client_id", client.ID),
		log.String("message_type", message.Type()),
		log.String("message_id", message.ID()),
		log.String("remote_addr", client.RemoteAddress),
	}
	if err != nil {
		m.logger.Error("IMessage handling failed", fields...)
	} else {
		m.logger.Debug("IMessage handled successfully", fields...)
	}
	return nil
}

// OnConnect logs client connections
func (m *LoggingMiddleware) OnConnect(_ context.Context, client protocol.ClientInfo) error {
	m.logger.Debug("Client connected",
		log.String("client_id", client.ID),
		log.String("remote_addr", client.RemoteAddress),
		log.String("user_agent", client.UserAgent),
	)
	return nil
}

// OnDisconnect logs client disconnections
func (m *LoggingMiddleware) OnDisconnect(_ context.Context, client protocol.ClientInfo, reason string) error {
	m.logger.Debug("Client disconnected",
		log.String("client_id", client.ID),
		log.String("remote_addr", client.RemoteAddress),
		log.String("reason", reason),
		log.Duration("duration", time.Since(client.ConnectedAt)),
	)
	return nil
}
