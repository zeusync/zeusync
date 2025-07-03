# ZeuSync Protocol Implementation

This package provides a complete, production-ready implementation of network protocols for real-time communication, supporting both WebSocket and QUIC protocols with advanced features like middleware, groups, metrics, and more.

## Features

### Core Features
- **Multi-Protocol Support**: WebSocket and QUIC implementations
- **Thread-Safe Operations**: All operations are thread-safe with proper synchronization
- **Message Compression**: Automatic compression for large messages
- **Quality of Service**: Support for different delivery guarantees
- **Priority Handling**: Message priority levels for better performance
- **Group/Room Management**: Built-in support for client groups and rooms
- **Middleware System**: Extensible middleware for cross-cutting concerns
- **Metrics Collection**: Comprehensive metrics and monitoring
- **Connection Management**: Advanced connection lifecycle management

### Security Features
- **TLS Support**: Full TLS 1.3 support for both protocols
- **Authentication Middleware**: Built-in authentication support
- **Rate Limiting**: Configurable rate limiting per client
- **Input Validation**: Message validation and size limits
- **Origin Checking**: WebSocket origin validation

### Performance Features
- **Worker Pools**: Configurable worker pools for message processing
- **Connection Pooling**: Efficient connection management
- **Message Queuing**: Asynchronous message processing with queues
- **Compression**: Automatic message compression for bandwidth optimization
- **Keep-Alive**: Intelligent keep-alive mechanisms

## Quick Start

### Basic WebSocket Server

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/sirupsen/logrus"
    "zeusync/internal/core/protocol"
    "zeusync/internal/core/protocol/intrefaces"
)

func main() {
    // Configure the protocol
    config := intrefaces.ProtocolConfig{
        Host:              "0.0.0.0",
        Port:              8080,
        MaxConnections:    1000,
        ReadTimeout:       30 * time.Second,
        WriteTimeout:      30 * time.Second,
        MaxMessageSize:    1024 * 1024, // 1MB
        BufferSize:        4096,
        WorkerCount:       10,
        QueueSize:         10000,
        EnableCompression: true,
        EnableGroups:      true,
        EnableMetrics:     true,
    }

    logger := logrus.New()
    wsProtocol := protocol.NewWebSocketProtocol(config, logger)

    // Register a message handler
    wsProtocol.RegisterHandler("chat", func(ctx context.Context, client intrefaces.ClientInfo, message intrefaces.Message) error {
        log.Printf("Received chat message from %s: %v", client.ID, message.Payload())
        
        // Broadcast to all clients
        return wsProtocol.Broadcast(message)
    })

    // Start the server
    ctx := context.Background()
    if err := wsProtocol.Start(ctx, config); err != nil {
        log.Fatal(err)
    }

    log.Println("WebSocket server started on ws://localhost:8080/ws")
    
    // Keep running
    select {}
}
```

### Basic QUIC Server

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/sirupsen/logrus"
    "zeusync/internal/core/protocol"
    "zeusync/internal/core/protocol/intrefaces"
)

func main() {
    config := intrefaces.ProtocolConfig{
        Host:           "0.0.0.0",
        Port:           9090,
        MaxConnections: 1000,
        ReadTimeout:    30 * time.Second,
        WriteTimeout:   30 * time.Second,
        BufferSize:     4096,
        WorkerCount:    10,
        QueueSize:      10000,
        TLSEnabled:     false, // Uses self-signed cert for development
    }

    logger := logrus.New()
    quicProtocol := protocol.NewQuicProtocol(config, logger)

    // Register handlers
    quicProtocol.RegisterHandler("echo", func(ctx context.Context, client intrefaces.ClientInfo, message intrefaces.Message) error {
        // Echo the message back
        response := message.CreateResponse(message.Payload())
        return quicProtocol.Send(client.ID, response)
    })

    ctx := context.Background()
    if err := quicProtocol.Start(ctx, config); err != nil {
        log.Fatal(err)
    }

    log.Println("QUIC server started on quic://localhost:9090")
    select {}
}
```

## Advanced Usage

### Message Handling

```go
// Create a message
message := protocol.NewMessage("chat", map[string]interface{}{
    "text": "Hello, World!",
    "user": "alice",
}, protocol.MessageOptions{
    Priority: intrefaces.PriorityHigh,
    QoS:      intrefaces.QoSExactlyOnce,
    Headers: map[string]string{
        "source": "client1",
        "target": "client2",
    },
})

// Send to specific client
err := protocol.Send("client-id", message)

// Broadcast to all clients
err := protocol.Broadcast(message)

// Send to multiple clients
err := protocol.SendToMultiple([]string{"client1", "client2"}, message)
```

### Group Management

```go
// Create a group
err := protocol.CreateGroup("chat-room-1")

// Add clients to group
err := protocol.JoinGroup("client1", "chat-room-1")
err := protocol.JoinGroup("client2", "chat-room-1")

// Send message to group
message := protocol.NewMessage("group-chat", "Hello everyone!")
err := protocol.SendToGroup("chat-room-1", message)

// Get group members
members, err := protocol.GetGroupMembers("chat-room-1")

// Remove client from group
err := protocol.LeaveGroup("client1", "chat-room-1")
```

### Middleware

```go
// Logging middleware
loggingMiddleware := &protocol.LoggingMiddleware{Logger: logger}
protocol.AddMiddleware(loggingMiddleware)

// Authentication middleware
authMiddleware := &protocol.AuthMiddleware{Logger: logger}
protocol.AddMiddleware(authMiddleware)

// Rate limiting middleware
rateLimitMiddleware := &protocol.RateLimitMiddleware{
    Logger:    logger,
    RateLimit: 100, // 100 messages per minute
    Window:    time.Minute,
}
protocol.AddMiddleware(rateLimitMiddleware)

// Custom middleware
type CustomMiddleware struct{}

func (m *CustomMiddleware) Name() string { return "custom" }
func (m *CustomMiddleware) Priority() uint16 { return 500 }

func (m *CustomMiddleware) BeforeHandle(ctx context.Context, client intrefaces.ClientInfo, message intrefaces.Message) error {
    // Custom logic before message handling
    return nil
}

func (m *CustomMiddleware) AfterHandle(ctx context.Context, client intrefaces.ClientInfo, message intrefaces.Message, response intrefaces.Message, err error) error {
    // Custom logic after message handling
    return nil
}

func (m *CustomMiddleware) OnConnect(ctx context.Context, client intrefaces.ClientInfo) error {
    // Custom logic on client connect
    return nil
}

func (m *CustomMiddleware) OnDisconnect(ctx context.Context, client intrefaces.ClientInfo, reason string) error {
    // Custom logic on client disconnect
    return nil
}

protocol.AddMiddleware(&CustomMiddleware{})
```

### Metrics and Monitoring

```go
// Get protocol metrics
metrics := protocol.GetMetrics()
log.Printf("Active connections: %d", metrics.ActiveConnections)
log.Printf("Messages per second: %.2f", metrics.MessagesPerSecond)

// Get client-specific metrics
clientMetrics, exists := protocol.GetClientMetrics("client-id")
if exists {
    log.Printf("Client messages sent: %d", clientMetrics.MessagesSent)
}

// Health check endpoint (for WebSocket)
// GET /health returns JSON with server status

// Metrics endpoint (for WebSocket)
// GET /metrics returns JSON with detailed metrics
```

## Configuration

### Protocol Configuration

```go
config := intrefaces.ProtocolConfig{
    // Network settings
    Host:             "0.0.0.0",
    Port:             8080,
    MaxConnections:   1000,
    ReadTimeout:      30 * time.Second,
    WriteTimeout:     30 * time.Second,
    KeepAliveTimeout: 60 * time.Second,

    // Message settings
    MaxMessageSize:    1024 * 1024, // 1MB
    CompressionLevel:  6,           // gzip compression level
    EnableCompression: true,

    // Security settings
    TLSEnabled:  true,
    CertFile:    "/path/to/cert.pem",
    KeyFile:     "/path/to/key.pem",
    RequireAuth: true,

    // Performance tuning
    BufferSize:       4096,
    WorkerCount:      10,
    QueueSize:        10000,
    EnablePipelining: true,

    // Features
    EnableGroups:     true,
    EnableMiddleware: true,
    EnableMetrics:    true,
    EnableTracing:    true,

    // Custom options
    Options: map[string]any{
        "custom_option": "value",
    },
}
```

## Architecture

### Protocol Interface

The core `Protocol` interface provides a unified API for different transport protocols:

```go
type Protocol interface {
    // Identity
    Name() string
    Version() string
    Type() ProtocolType

    // Lifecycle
    Start(ctx context.Context, config ProtocolConfig) error
    Stop(ctx context.Context) error
    Restart(ctx context.Context) error
    IsRunning() bool

    // Message handling
    RegisterHandler(messageType string, handler MessageHandler) error
    UnregisterHandler(messageType string) error
    GetHandler(messageType string) (MessageHandler, bool)
    SetDefaultHandler(handler MessageHandler)

    // Client management
    Send(clientID string, message Message) error
    SendToMultiple(clientIDs []string, message Message) error
    Broadcast(message Message) error
    BroadcastExcept(excludeClientIDs []string, message Message) error

    // Connection management
    GetClient(clientID string) (ClientInfo, bool)
    GetAllClients() []ClientInfo
    DisconnectClient(clientID string, reason string) error
    GetConnectionCount() int

    // Groups/Rooms support
    CreateGroup(groupID string) error
    DeleteGroup(groupID string) error
    JoinGroup(clientID, groupID string) error
    LeaveGroup(clientID, groupID string) error
    SendToGroup(groupID string, message Message) error
    GetGroupMembers(groupID string) ([]string, error)

    // Middleware support
    AddMiddleware(middleware ProtocolMiddleware) error
    RemoveMiddleware(name string) error

    // Metrics and monitoring
    GetMetrics() ProtocolMetrics
    GetClientMetrics(clientID string) (ClientMetrics, bool)

    // Configuration
    GetConfig() ProtocolConfig
    UpdateConfig(config ProtocolConfig) error
}
```

### Message System

Messages are the core communication unit with support for:

- **Serialization**: JSON-based serialization with compression
- **Headers**: Key-value metadata
- **Routing**: Message routing information
- **Priority**: Message priority levels
- **QoS**: Quality of service guarantees
- **Responses**: Request-response pattern support

### Connection Management

Each connection is managed through the `Connection` interface:

- **Thread-safe operations**: All connection operations are thread-safe
- **Metadata**: Arbitrary metadata storage per connection
- **Callbacks**: Event callbacks for connection lifecycle
- **Metrics**: Per-connection metrics tracking

## Performance Considerations

### WebSocket Performance
- Uses Gorilla WebSocket library for optimal performance
- Configurable read/write buffer sizes
- Automatic ping/pong handling for connection health
- Efficient message queuing and worker pools

### QUIC Performance
- Uses lucas-clemente/quic-go for QUIC implementation
- Stream multiplexing for better throughput
- Automatic congestion control
- Built-in packet loss recovery

### Memory Management
- Connection pooling to reduce GC pressure
- Efficient message serialization
- Configurable buffer sizes
- Automatic cleanup of disconnected clients

### Scalability
- Horizontal scaling through load balancing
- Configurable worker pools
- Efficient group management
- Metrics for monitoring and optimization

## Testing

Run the test suite:

```bash
go test ./internal/core/protocol/...
```

Run benchmarks:

```bash
go test -bench=. ./internal/core/protocol/...
```

## Examples

See the `example_usage.go` file for a complete example server implementation that demonstrates:

- Multi-protocol support (WebSocket + QUIC)
- Message handling and routing
- Group/room management
- Middleware integration
- Metrics collection
- Graceful shutdown

## Production Deployment

### Security Checklist
- [ ] Use proper TLS certificates (not self-signed)
- [ ] Implement proper authentication
- [ ] Configure rate limiting
- [ ] Validate all input messages
- [ ] Set appropriate message size limits
- [ ] Configure proper CORS policies for WebSocket

### Performance Checklist
- [ ] Tune worker pool sizes based on load
- [ ] Configure appropriate buffer sizes
- [ ] Enable compression for large messages
- [ ] Set up monitoring and alerting
- [ ] Configure proper timeouts
- [ ] Implement connection limits

### Monitoring
- Monitor connection counts and message rates
- Track error rates and response times
- Set up alerts for unusual patterns
- Use the built-in metrics endpoints
- Implement custom metrics as needed

## Contributing

When contributing to this protocol implementation:

1. Ensure all tests pass
2. Add tests for new features
3. Follow Go best practices
4. Update documentation
5. Consider performance implications
6. Maintain backward compatibility

## License

This implementation is part of the ZeuSync project and follows the project's licensing terms.