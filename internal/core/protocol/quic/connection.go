package quic

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zeusync/zeusync/internal/core/observability/log"
	"github.com/zeusync/zeusync/internal/core/protocol"

	"github.com/quic-go/quic-go"
)

// Connection implements the Connection interface for QUIC
type Connection struct {
	*protocol.BaseConnection
	conn      *quic.Conn
	transport *Transport
	streams   sync.Map // map[protocol.StreamID]*QUICStream
	closed    int32    // atomic bool
	logger    log.Log

	// Message handling
	messageQueue chan *protocol.Message
	sendQueue    chan *protocol.Message

	// Stream management
	streamCounter uint64 // atomic
	maxStreams    int
}

// NewQUICConnection creates a new QUIC connection
func NewQUICConnection(conn *quic.Conn, transport *Transport, config *protocol.GlobalConfig, logger log.Log) *Connection {
	id := protocol.GenerateConnectionID()

	if logger == nil {
		logger = log.Provide()
	}

	baseConn := protocol.NewBaseConnection(
		id,
		conn.RemoteAddr(),
		conn.LocalAddr(),
		protocol.TransportQUIC,
		config,
		logger,
	)

	qconn := &Connection{
		BaseConnection: baseConn,
		conn:           conn,
		transport:      transport,
		messageQueue:   make(chan *protocol.Message, config.MessageBufferSize),
		sendQueue:      make(chan *protocol.Message, config.MessageBufferSize),
		maxStreams:     config.MaxStreams,
		logger:         logger.With(log.String("connection_id", string(id))),
	}

	// Set connection state to connected
	qconn.SetState(protocol.ConnectionStateConnected)

	qconn.logger.Info("QUIC connection established",
		log.String("remote_addr", conn.RemoteAddr().String()),
		log.String("local_addr", conn.LocalAddr().String()))

	// Start background goroutines
	go qconn.handleMessages()
	go qconn.handleSending()
	go qconn.monitorConnection()

	return qconn
}

// SendMessage sends a message over the connection
func (c *Connection) SendMessage(msg *protocol.Message) error {
	if atomic.LoadInt32(&c.closed) == 1 {
		c.logger.Debug("Attempted to send message on closed connection")
		return protocol.ErrConnectionClosed
	}

	if c.State() != protocol.ConnectionStateConnected {
		c.logger.Debug("Attempted to send message on non-connected connection",
			log.String("state", c.State().String()))
		return protocol.ErrConnectionClosed
	}

	// Add message to send queue
	select {
	case c.sendQueue <- msg:
		msg.Retain() // Retain reference for async processing
		c.logger.Debug("Message queued for sending", log.String("message_id", string(msg.ID)))
		return nil
	default:
		c.logger.Warn("Send queue full, dropping message", log.String("message_id", string(msg.ID)))
		return protocol.ErrMessageQueueFull
	}
}

// SendMessageAsync sends a message asynchronously
func (c *Connection) SendMessageAsync(msg *protocol.Message) <-chan error {
	errChan := make(chan error, 1)

	go func() {
		defer close(errChan)
		errChan <- c.SendMessage(msg)
	}()

	return errChan
}

// ReceiveMessage receives a message from the connection
func (c *Connection) ReceiveMessage(ctx context.Context) (*protocol.Message, error) {
	if atomic.LoadInt32(&c.closed) == 1 {
		return nil, protocol.ErrConnectionClosed
	}

	select {
	case msg := <-c.messageQueue:
		if msg != nil {
			c.UpdateActivity()
			c.AddBytesReceived(uint64(msg.Size()))
			c.logger.Debug("Message received", log.String("message_id", string(msg.ID)))
			return msg, nil
		}
		return nil, protocol.ErrConnectionClosed
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.Done():
		return nil, protocol.ErrConnectionClosed
	}
}

// OpenStream opens a new bidirectional stream
func (c *Connection) OpenStream(ctx context.Context) (protocol.Stream, error) {
	if atomic.LoadInt32(&c.closed) == 1 {
		return nil, protocol.ErrConnectionClosed
	}

	if c.State() != protocol.ConnectionStateConnected {
		return nil, protocol.ErrConnectionClosed
	}

	// Check stream limit
	if c.maxStreams > 0 && int(atomic.LoadUint64(&c.streamCounter)) >= c.maxStreams {
		c.logger.Warn("Maximum streams reached", log.Int("max_streams", c.maxStreams))
		return nil, protocol.ErrMaxStreamsReached
	}

	stream, err := c.conn.OpenStreamSync(ctx)
	if err != nil {
		c.logger.Error("Failed to open QUIC stream", log.Error(err))
		return nil, protocol.WrapError(err, "failed to open QUIC stream")
	}

	atomic.AddUint64(&c.streamCounter, 1)

	qstream := NewQUICStream(stream, c.ID(), protocol.StreamTypeBidirectional, protocol.DefaultStreamConfig(), c.logger)
	c.streams.Store(qstream.ID(), qstream)

	c.logger.Debug("Stream opened",
		log.String("stream_id", string(qstream.ID())),
		log.Uint64("total_streams", atomic.LoadUint64(&c.streamCounter)))

	return qstream, nil
}

// AcceptStream accepts an incoming stream
func (c *Connection) AcceptStream(ctx context.Context) (protocol.Stream, error) {
	if atomic.LoadInt32(&c.closed) == 1 {
		return nil, protocol.ErrConnectionClosed
	}

	if c.State() != protocol.ConnectionStateConnected {
		return nil, protocol.ErrConnectionClosed
	}

	stream, err := c.conn.AcceptStream(ctx)
	if err != nil {
		c.logger.Error("Failed to accept QUIC stream", log.Error(err))
		return nil, protocol.WrapError(err, "failed to accept QUIC stream")
	}

	atomic.AddUint64(&c.streamCounter, 1)

	qstream := NewQUICStream(stream, c.ID(), protocol.StreamTypeBidirectional, protocol.DefaultStreamConfig(), c.logger)
	c.streams.Store(qstream.ID(), qstream)

	c.logger.Debug("Stream accepted",
		log.String("stream_id", string(qstream.ID())),
		log.Uint64("total_streams", atomic.LoadUint64(&c.streamCounter)))

	return qstream, nil
}

// Ping sends a ping and measures round-trip time
func (c *Connection) Ping(ctx context.Context) (time.Duration, error) {
	if atomic.LoadInt32(&c.closed) == 1 {
		return 0, protocol.ErrConnectionClosed
	}

	start := time.Now()

	// Create a ping message
	pingMsg := protocol.GetMessage()
	pingMsg.Type = protocol.MessageTypeHeartbeat
	pingMsg.Payload = []byte("ping")
	defer pingMsg.Release()

	// Send ping via a temporary stream
	stream, err := c.OpenStream(ctx)
	if err != nil {
		c.logger.Error("Failed to open stream for ping", log.Error(err))
		return 0, err
	}
	defer func() {
		_ = stream.Close()
	}()

	err = stream.SendMessage(pingMsg)
	if err != nil {
		c.logger.Error("Failed to send ping message", log.Error(err))
		return 0, err
	}

	// Wait for response (simplified ping implementation)
	_, err = stream.ReceiveMessage(ctx)
	if err != nil {
		c.logger.Error("Failed to receive ping response", log.Error(err))
		return 0, err
	}

	latency := time.Since(start)
	c.logger.Debug("Ping completed", log.Duration("latency", latency))

	return latency, nil
}

// Close closes the connection
func (c *Connection) Close() error {
	if !atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		return nil // Already closed
	}

	c.logger.Info("Closing QUIC connection")
	c.SetState(protocol.ConnectionStateDisconnecting)

	// Close all streams
	c.streams.Range(func(key, value interface{}) bool {
		if stream, ok := value.(*Stream); ok {
			_ = stream.Close()
		}
		return true
	})

	// Close the QUIC connection
	err := c.conn.CloseWithError(0, "connection closed")

	c.SetState(protocol.ConnectionStateDisconnected)
	c.MarkClosed()

	// Update transport stats
	c.transport.UpdateStats("connections_active", ^uint64(0)) // Decrement

	return err
}

// handleMessages handles incoming messages from streams
func (c *Connection) handleMessages() {
	defer close(c.messageQueue)
	c.logger.Debug("Message handler started")

	for {
		select {
		case <-c.Done():
			c.logger.Debug("Message handler stopping")
			return
		default:
			// Accept incoming streams and handle messages
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			stream, err := c.AcceptStream(ctx)
			cancel()

			if err != nil {
				if atomic.LoadInt32(&c.closed) == 1 {
					return
				}
				continue
			}

			// Handle stream messages in a separate goroutine
			go c.handleStreamMessages(stream)
		}
	}
}

// handleStreamMessages handles messages from a specific stream
func (c *Connection) handleStreamMessages(stream protocol.Stream) {
	defer func() {
		_ = stream.Close()
	}()

	streamLogger := c.logger.With(log.String("stream_id", string(stream.ID())))
	streamLogger.Debug("Stream message handler started")

	for {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		msg, err := stream.ReceiveMessage(ctx)
		cancel()

		if err != nil {
			streamLogger.Debug("Stream message handler stopping", log.Error(err))
			return
		}

		if msg != nil {
			select {
			case c.messageQueue <- msg:
				streamLogger.Debug("Message forwarded to queue", log.String("message_id", string(msg.ID)))
			case <-c.Done():
				msg.Release()
				return
			default:
				// Queue full, drop message
				streamLogger.Warn("Message queue full, dropping message", log.String("message_id", string(msg.ID)))
				msg.Release()
			}
		}
	}
}

// handleSending handles outgoing messages
func (c *Connection) handleSending() {
	c.logger.Debug("Send handler started")

	for {
		select {
		case msg := <-c.sendQueue:
			if msg != nil {
				_ = c.sendMessageInternal(msg)
				msg.Release()
			}
		case <-c.Done():
			c.logger.Debug("Send handler stopping")
			return
		}
	}
}

// sendMessageInternal sends a message internally
func (c *Connection) sendMessageInternal(msg *protocol.Message) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Open a stream for the message
	stream, err := c.OpenStream(ctx)
	if err != nil {
		c.logger.Error("Failed to open stream for message",
			log.String("message_id", string(msg.ID)),
			log.Error(err))
		return err
	}
	defer func() {
		_ = stream.Close()
	}()

	// Send the message
	err = stream.SendMessage(msg)
	if err != nil {
		c.logger.Error("Failed to send message",
			log.String("message_id", string(msg.ID)),
			log.Error(err))
		return err
	}

	c.UpdateActivity()
	c.AddBytesSent(uint64(msg.Size()))

	c.logger.Debug("Message sent successfully",
		log.String("message_id", string(msg.ID)),
		log.Int("payload_size", len(msg.Payload)))

	return nil
}

// monitorConnection monitors the connection health
func (c *Connection) monitorConnection() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	c.logger.Debug("Connection monitor started")

	for {
		select {
		case <-ticker.C:
			// Check if connection is still alive
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			latency, err := c.Ping(ctx)
			cancel()

			if err != nil {
				c.logger.Error("Connection health check failed", log.Error(err))
				c.SetState(protocol.ConnectionStateError)
				_ = c.Close()
				return
			}

			c.logger.Debug("Connection health check passed", log.Duration("latency", latency))

		case <-c.Done():
			c.logger.Debug("Connection monitor stopping")
			return
		}
	}
}

// GetStream returns a stream by ID
func (c *Connection) GetStream(id protocol.StreamID) (protocol.Stream, bool) {
	if value, exists := c.streams.Load(id); exists {
		return value.(*Stream), true
	}
	return nil, false
}

// RemoveStream removes a stream from the connection
func (c *Connection) RemoveStream(id protocol.StreamID) {
	c.streams.Delete(id)
	atomic.AddUint64(&c.streamCounter, ^uint64(0)) // Decrement
	c.logger.Debug("Stream removed",
		log.String("stream_id", string(id)),
		log.Uint64("remaining_streams", atomic.LoadUint64(&c.streamCounter)))
}
