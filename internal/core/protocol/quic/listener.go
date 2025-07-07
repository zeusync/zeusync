package quic

import (
	"context"
	"net"
	"sync/atomic"

	"github.com/zeusync/zeusync/internal/core/observability/log"
	"github.com/zeusync/zeusync/internal/core/protocol"

	"github.com/quic-go/quic-go"
)

// Listener implements the ConnectionListener interface for QUIC
type Listener struct {
	listener  *quic.Listener
	transport *Transport
	config    *protocol.GlobalConfig
	closed    int32 // atomic bool
	logger    log.Log
}

// NewQUICListener creates a new QUIC listener
func NewQUICListener(listener *quic.Listener, transport *Transport, config *protocol.GlobalConfig, logger log.Log) *Listener {
	if logger == nil {
		logger = log.Provide()
	}

	qlistener := &Listener{
		listener:  listener,
		transport: transport,
		config:    config,
		logger:    logger.With(log.String("listener_addr", listener.Addr().String())),
	}

	qlistener.logger.Info("QUIC listener created",
		log.String("addr", listener.Addr().String()))

	return qlistener
}

// Listen starts listening for connections (no-op for QUIC as it's already listening)
func (l *Listener) Listen(ctx context.Context) error {
	// QUIC listener is already listening when created
	l.logger.Debug("QUIC listener is already active")
	return nil
}

// Accept accepts a new connection
func (l *Listener) Accept(ctx context.Context) (protocol.Connection, error) {
	if atomic.LoadInt32(&l.closed) == 1 {
		l.logger.Debug("Attempted to accept on closed listener")
		return nil, protocol.ErrConnectionClosed
	}

	l.logger.Debug("Waiting for incoming QUIC connection")

	conn, err := l.listener.Accept(ctx)
	if err != nil {
		l.transport.UpdateStats("errors_encountered", 1)
		l.transport.UpdateStats("connections_failed", 1)
		l.logger.Error("Failed to accept QUIC connection", log.Error(err))
		return nil, protocol.WrapError(err, "failed to accept QUIC connection")
	}

	l.transport.UpdateStats("connections_accepted", 1)
	l.transport.UpdateStats("connections_active", 1)

	l.logger.Info("QUIC connection accepted",
		log.String("remote_addr", conn.RemoteAddr().String()),
		log.String("local_addr", conn.LocalAddr().String()))

	return NewQUICConnection(conn, l.transport, l.config, l.logger), nil
}

// Addr returns the listener address
func (l *Listener) Addr() net.Addr {
	return l.listener.Addr()
}

// Transport returns the transport type
func (l *Listener) Transport() protocol.TransportType {
	return protocol.TransportQUIC
}

// Close closes the listener
func (l *Listener) Close() error {
	if !atomic.CompareAndSwapInt32(&l.closed, 0, 1) {
		return nil // Already closed
	}

	l.logger.Info("Closing QUIC listener")
	return l.listener.Close()
}
