package protocol

import (
	"context"
	"net"
	"sync/atomic"
	"time"

	"github.com/zeusync/zeusync/internal/core/observability/log"
)

// Connection represents an abstract network connection
type Connection interface {
	// Basic connection operations

	ID() ConnectionID
	RemoteAddr() net.Addr
	LocalAddr() net.Addr
	Transport() TransportType

	// State management

	State() ConnectionState
	Info() ConnectionInfo

	// Message operations

	SendMessage(msg *Message) error
	SendMessageAsync(msg *Message) <-chan error
	ReceiveMessage(ctx context.Context) (*Message, error)

	// Stream operations

	OpenStream(ctx context.Context) (Stream, error)
	AcceptStream(ctx context.Context) (Stream, error)

	// Health and lifecycle

	Ping(ctx context.Context) (time.Duration, error)
	Close() error
	Done() <-chan struct{}

	// Statistics

	BytesSent() uint64
	BytesReceived() uint64
	LastActivity() time.Time
}

// BaseConnection provides common connection functionality
type BaseConnection struct {
	id            ConnectionID
	state         int32 // atomic ConnectionState
	remoteAddr    net.Addr
	localAddr     net.Addr
	transport     TransportType
	connectedAt   time.Time
	lastActivity  int64 // atomic unix timestamp
	bytesSent     uint64
	bytesReceived uint64
	done          chan struct{}
	config        *GlobalConfig
	logger        log.Log
}

// NewBaseConnection creates a new base connection
func NewBaseConnection(id ConnectionID, remoteAddr, localAddr net.Addr, transport TransportType, config *GlobalConfig, logger log.Log) *BaseConnection {
	if config == nil {
		config = DefaultGlobalConfig()
	}
	if logger == nil {
		logger = log.Provide()
	}

	conn := &BaseConnection{
		id:           id,
		state:        int32(ConnectionStateConnecting),
		remoteAddr:   remoteAddr,
		localAddr:    localAddr,
		transport:    transport,
		connectedAt:  time.Now(),
		lastActivity: time.Now().Unix(),
		done:         make(chan struct{}),
		config:       config,
		logger:       logger.With(log.String("connection_id", string(id)), log.String("transport", string(transport))),
	}

	conn.logger.Debug("Connection created",
		log.String("remote_addr", remoteAddr.String()),
		log.String("local_addr", localAddr.String()))

	return conn
}

// ID returns the connection ID
func (c *BaseConnection) ID() ConnectionID {
	return c.id
}

// RemoteAddr returns the remote address
func (c *BaseConnection) RemoteAddr() net.Addr {
	return c.remoteAddr
}

// LocalAddr returns the local address
func (c *BaseConnection) LocalAddr() net.Addr {
	return c.localAddr
}

// Transport returns the transport type
func (c *BaseConnection) Transport() TransportType {
	return c.transport
}

// State returns the current connection state
func (c *BaseConnection) State() ConnectionState {
	return ConnectionState(atomic.LoadInt32(&c.state))
}

// SetState atomically sets the connection state
func (c *BaseConnection) SetState(state ConnectionState) {
	oldState := ConnectionState(atomic.SwapInt32(&c.state, int32(state)))
	if oldState != state {
		c.logger.Debug("Connection state changed",
			log.String("old_state", oldState.String()),
			log.String("new_state", state.String()))
	}
}

// Info returns connection information
func (c *BaseConnection) Info() ConnectionInfo {
	return ConnectionInfo{
		ID:            c.id,
		RemoteAddr:    c.remoteAddr.String(),
		LocalAddr:     c.localAddr.String(),
		Transport:     c.transport,
		ConnectedAt:   c.connectedAt,
		LastActivity:  time.Unix(atomic.LoadInt64(&c.lastActivity), 0),
		BytesSent:     atomic.LoadUint64(&c.bytesSent),
		BytesReceived: atomic.LoadUint64(&c.bytesReceived),
		State:         c.State(),
	}
}

// BytesSent returns the number of bytes sent
func (c *BaseConnection) BytesSent() uint64 {
	return atomic.LoadUint64(&c.bytesSent)
}

// BytesReceived returns the number of bytes received
func (c *BaseConnection) BytesReceived() uint64 {
	return atomic.LoadUint64(&c.bytesReceived)
}

// LastActivity returns the last activity time
func (c *BaseConnection) LastActivity() time.Time {
	return time.Unix(atomic.LoadInt64(&c.lastActivity), 0)
}

// UpdateActivity updates the last activity timestamp
func (c *BaseConnection) UpdateActivity() {
	atomic.StoreInt64(&c.lastActivity, time.Now().Unix())
}

// AddBytesSent atomically adds to bytes sent counter
func (c *BaseConnection) AddBytesSent(bytes uint64) {
	atomic.AddUint64(&c.bytesSent, bytes)
	c.UpdateActivity()
}

// AddBytesReceived atomically adds to bytes received counter
func (c *BaseConnection) AddBytesReceived(bytes uint64) {
	atomic.AddUint64(&c.bytesReceived, bytes)
	c.UpdateActivity()
}

// Done returns a channel that's closed when the connection is closed
func (c *BaseConnection) Done() <-chan struct{} {
	return c.done
}

// MarkClosed marks the connection as closed
func (c *BaseConnection) MarkClosed() {
	c.SetState(ConnectionStateDisconnected)
	select {
	case <-c.done:
	default:
		close(c.done)
		c.logger.Info("Connection closed",
			log.Duration("duration", time.Since(c.connectedAt)),
			log.Uint64("bytes_sent", atomic.LoadUint64(&c.bytesSent)),
			log.Uint64("bytes_received", atomic.LoadUint64(&c.bytesReceived)))
	}
}

// Logger returns the connection logger
func (c *BaseConnection) Logger() log.Log {
	return c.logger
}

// Config returns the connection configuration
func (c *BaseConnection) Config() *GlobalConfig {
	return c.config
}

// ConnectionManager manages multiple connections
type ConnectionManager interface {
	// Connection lifecycle

	AddConnection(conn Connection) error
	RemoveConnection(id ConnectionID) error
	GetConnection(id ConnectionID) (Connection, bool)

	// Connection queries

	ListConnections() []Connection
	ConnectionCount() int

	// Health monitoring

	HealthCheck(ctx context.Context) []HealthCheck

	// Cleanup

	Close() error
}

// ConnectionListener listens for incoming connections
type ConnectionListener interface {
	// Listen for connections

	Listen(ctx context.Context) error
	Accept(ctx context.Context) (Connection, error)

	// Configuration

	Addr() net.Addr
	Transport() TransportType

	// Lifecycle

	Close() error
}

// ConnectionDialer creates outbound connections
type ConnectionDialer interface {
	// Dial a connection

	Dial(ctx context.Context, addr string) (Connection, error)
	DialWithConfig(ctx context.Context, addr string, config *GlobalConfig) (Connection, error)

	// Configuration

	Transport() TransportType

	// Lifecycle

	Close() error
}

// ConnectionState string representation
func (cs ConnectionState) String() string {
	switch cs {
	case ConnectionStateConnecting:
		return "connecting"
	case ConnectionStateConnected:
		return "connected"
	case ConnectionStateDisconnecting:
		return "disconnecting"
	case ConnectionStateDisconnected:
		return "disconnected"
	case ConnectionStateError:
		return "error"
	default:
		return "unknown"
	}
}
