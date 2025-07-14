package server

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zeusync/zeusync/internal/core/protocol"
	"github.com/zeusync/zeusync/internal/core/protocol/quic"

	"github.com/zeusync/zeusync/internal/core/observability/log"
)

// Server represents a ZeuSync game server
type Server struct {
	// Core components
	transport protocol.Transport
	listener  protocol.ConnectionListener

	// Client management
	clients     sync.Map // map[protocol.ClientID]*ClientSession
	clientCount int64    // atomic

	// Group management
	groups     sync.Map // map[string]*Group
	groupCount int64    // atomic

	// Scope management
	scopes     sync.Map // map[string]*Scope
	scopeCount int64    // atomic

	// Server state
	running int32 // atomic bool
	closed  int32 // atomic bool

	// Configuration and logging
	config Config
	logger log.Log

	// Background workers
	workerGroup sync.WaitGroup
	stopChan    chan struct{}
}

// Config holds server configuration
type Config struct {
	// Network settings
	ListenAddr string
	MaxClients int

	// Message settings
	MessageTimeout    time.Duration
	MaxMessageSize    int
	MessageBufferSize int

	// Group settings
	MaxGroups       int
	MaxGroupMembers int

	// Scope settings
	MaxScopes        int
	MaxScopeDataSize uint64

	// Health monitoring
	HealthCheckInterval time.Duration
	ClientTimeout       time.Duration

	// Logging
	LogLevel log.Level

	// Custom options
	CustomOptions map[string]interface{}
}

// DefaultServerConfig returns default server configuration
func DefaultServerConfig() Config {
	return Config{
		ListenAddr:          "127.0.0.1:8080",
		MaxClients:          10_000,
		MessageTimeout:      10 * time.Second,
		MaxMessageSize:      1024 * 1024, // 1MB
		MessageBufferSize:   1000,
		MaxGroups:           1000,
		MaxGroupMembers:     10000,
		MaxScopes:           500,
		MaxScopeDataSize:    100 * 1024 * 1024, // 100MB
		HealthCheckInterval: 30 * time.Second,
		ClientTimeout:       5 * time.Minute,
		LogLevel:            log.LevelInfo,
		CustomOptions:       make(map[string]interface{}),
	}
}

// ClientSession represents a connected client session
type ClientSession struct {
	ID          protocol.ClientID
	Connection  protocol.Connection
	Groups      sync.Map // map[string]bool
	Scopes      sync.Map // map[string]bool
	Metadata    sync.Map // map[string]interface{}
	ConnectedAt time.Time
	LastSeen    int64 // atomic unix timestamp
	Active      int32 // atomic bool
}

// Group represents a client group
type Group struct {
	Name        string
	Clients     sync.Map // map[protocol.ClientID]bool
	ClientCount int64    // atomic
	CreatedAt   time.Time
	Metadata    sync.Map // map[string]interface{}
}

// Scope represents a data scope
type Scope struct {
	Name        string
	Clients     sync.Map // map[protocol.ClientID]bool
	ClientCount int64    // atomic
	Data        sync.Map // map[string][]byte
	DataSize    uint64   // atomic
	CreatedAt   time.Time
	Metadata    sync.Map // map[string]interface{}
}

// ControlMessage represents a control message structure
type ControlMessage struct {
	Action string                 `json:"action"`
	Group  string                 `json:"group,omitempty"`
	Scope  string                 `json:"scope,omitempty"`
	Data   map[string]interface{} `json:"data,omitempty"`
}

// NewServer creates a new ZeuSync server
func NewServer(config Config) *Server {
	if config.CustomOptions == nil {
		config.CustomOptions = make(map[string]interface{})
	}

	logger := log.New(config.LogLevel)

	server := &Server{
		config:   config,
		logger:   logger.With(log.String("component", "server")),
		stopChan: make(chan struct{}),
	}

	server.logger.Info("Server created",
		log.String("listen_addr", config.ListenAddr),
		log.Int("max_clients", config.MaxClients))

	return server
}

// Start starts the server
func (s *Server) Start(ctx context.Context) error {
	if atomic.LoadInt32(&s.closed) == 1 {
		return ErrServerClosed
	}

	if !atomic.CompareAndSwapInt32(&s.running, 0, 1) {
		return ErrServerAlreadyRunning
	}

	s.logger.Info("Starting server")

	// Create QUIC transport
	globalConfig := protocol.DefaultGlobalConfig()
	globalConfig.MaxConnections = s.config.MaxClients
	globalConfig.MaxMessageSize = s.config.MaxMessageSize
	globalConfig.MessageBufferSize = s.config.MessageBufferSize
	globalConfig.MaxGroups = s.config.MaxGroups
	globalConfig.MaxScopes = s.config.MaxScopes

	s.transport = quic.NewQUICTransport(nil, globalConfig, s.logger)

	// Create listener
	listener, err := s.transport.Listen(ctx, s.config.ListenAddr)
	if err != nil {
		atomic.StoreInt32(&s.running, 0)
		s.logger.Error("Failed to create listener", log.Error(err))
		return err
	}

	s.listener = listener

	s.logger.Info("Server listening",
		log.String("addr", listener.Addr().String()))

	// Start background workers
	s.startWorkers()

	// Start accepting connections
	go s.acceptConnections()

	s.logger.Info("Server started successfully")

	return nil
}

// Stop stops the server
func (s *Server) Stop(_ context.Context) error {
	if !atomic.CompareAndSwapInt32(&s.running, 1, 0) {
		return ErrServerNotRunning
	}

	s.logger.Info("Stopping server")

	// Signal stop
	close(s.stopChan)

	// Close listener
	if s.listener != nil {
		_ = s.listener.Close()
	}

	// Disconnect all clients
	s.clients.Range(func(key, value interface{}) bool {
		if session, ok := value.(*ClientSession); ok {
			_ = session.Connection.Close()
		}
		return true
	})

	// Wait for workers to stop
	s.stopWorkers()

	// Close transport
	if s.transport != nil {
		_ = s.transport.Close()
	}

	s.logger.Info("Server stopped")

	return nil
}

// Close closes the server and releases all resources
func (s *Server) Close() error {
	if !atomic.CompareAndSwapInt32(&s.closed, 0, 1) {
		return nil // Already closed
	}

	s.logger.Info("Closing server")

	// Stop if running
	if atomic.LoadInt32(&s.running) == 1 {
		_ = s.Stop(context.Background())
	}

	s.logger.Info("Server closed")

	return nil
}

// acceptConnections accepts incoming client connections
func (s *Server) acceptConnections() {
	s.logger.Debug("Connection acceptor started")
	defer s.logger.Debug("Connection acceptor stopped")

	for atomic.LoadInt32(&s.running) == 1 {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

		conn, err := s.listener.Accept(ctx)
		if err != nil {
			cancel()

			if atomic.LoadInt32(&s.running) == 0 {
				return
			}

			if !errors.Is(err, context.DeadlineExceeded) {
				s.logger.Error("Failed to accept connection", log.Error(err))
			}

			// Добавляем небольшую паузу перед повторной попыткой
			time.Sleep(100 * time.Millisecond)
			continue
		}
		cancel()

		if int(atomic.LoadInt64(&s.clientCount)) >= s.config.MaxClients {
			s.logger.Warn("Maximum clients reached, rejecting connection",
				log.String("remote_addr", conn.RemoteAddr().String()))
			_ = conn.Close()
			continue
		}

		session := &ClientSession{
			ID:          protocol.GenerateClientID(),
			Connection:  conn,
			ConnectedAt: time.Now(),
			LastSeen:    time.Now().Unix(),
			Active:      1,
		}

		s.clients.Store(session.ID, session)
		atomic.AddInt64(&s.clientCount, 1)

		s.logger.Info("Client connected",
			log.String("client_id", string(session.ID)),
			log.String("remote_addr", conn.RemoteAddr().String()),
			log.Int64("total_clients", atomic.LoadInt64(&s.clientCount)))

		go s.handleClient(session)
	}
}

// handleClient handles a client session
func (s *Server) handleClient(session *ClientSession) {
	defer func() {
		// Remove client
		s.clients.Delete(session.ID)
		atomic.AddInt64(&s.clientCount, -1)

		// Remove from groups and scopes
		s.removeClientFromAllGroups(session.ID)
		s.removeClientFromAllScopes(session.ID)

		// Close connection
		_ = session.Connection.Close()

		s.logger.Info("Client disconnected",
			log.String("client_id", string(session.ID)),
			log.Int64("total_clients", atomic.LoadInt64(&s.clientCount)))
	}()

	clientLogger := s.logger.With(log.String("client_id", string(session.ID)))
	clientLogger.Debug("Client handler started")

	for atomic.LoadInt32(&session.Active) == 1 {
		ctx, cancel := context.WithTimeout(context.Background(), s.config.MessageTimeout)
		msg, err := session.Connection.ReceiveMessage(ctx)
		cancel()

		if err != nil {
			if atomic.LoadInt32(&session.Active) == 1 {
				clientLogger.Error("Failed to receive message", log.Error(err))
			}
			break
		}

		if msg != nil {
			atomic.StoreInt64(&session.LastSeen, time.Now().Unix())
			s.handleMessage(session, msg)
		}
	}

	clientLogger.Debug("Client handler stopped")
}

// handleMessage processes a message from a client
func (s *Server) handleMessage(session *ClientSession, msg *protocol.Message) {
	clientLogger := s.logger.With(log.String("client_id", string(session.ID)))

	clientLogger.Debug("Handling message",
		log.String("message_id", string(msg.ID)),
		log.String("type", msg.Type.String()))

	switch msg.Type {
	case protocol.MessageTypeControl:
		s.handleControlMessage(session, msg)
	case protocol.MessageTypeData:
		s.handleDataMessage(session, msg)
	case protocol.MessageTypeHeartbeat:
		s.handleHeartbeatMessage(session, msg)
	default:
		clientLogger.Warn("Unknown message type", log.String("type", msg.Type.String()))
	}

	msg.Release()
}

// handleControlMessage processes control messages
func (s *Server) handleControlMessage(session *ClientSession, msg *protocol.Message) {
	var controlMsg ControlMessage
	err := json.Unmarshal(msg.Payload, &controlMsg)
	if err != nil {
		s.logger.Error("Failed to parse control message", log.Error(err))
		return
	}

	clientLogger := s.logger.With(log.String("client_id", string(session.ID)))

	switch controlMsg.Action {
	case "join_group":
		s.handleJoinGroup(session, controlMsg.Group)
	case "leave_group":
		s.handleLeaveGroup(session, controlMsg.Group)
	case "subscribe_scope":
		s.handleSubscribeScope(session, controlMsg.Scope)
	case "unsubscribe_scope":
		s.handleUnsubscribeScope(session, controlMsg.Scope)
	default:
		clientLogger.Warn("Unknown control action", log.String("action", controlMsg.Action))
	}
}

// handleDataMessage processes data messages
func (s *Server) handleDataMessage(session *ClientSession, msg *protocol.Message) {
	// Echo the message back to the client
	builder := protocol.NewMessageBuilder(s.logger)
	response := builder.
		WithType(protocol.MessageTypeData).
		WithString("Echo: " + msg.AsString()).
		WithTarget(session.ID).
		Build()

	err := session.Connection.SendMessage(response)
	if err != nil {
		s.logger.Error("Failed to send echo response",
			log.String("client_id", string(session.ID)),
			log.Error(err))
	}
}

// handleHeartbeatMessage processes heartbeat messages
func (s *Server) handleHeartbeatMessage(session *ClientSession, msg *protocol.Message) {
	// Send heartbeat response
	builder := protocol.NewMessageBuilder(s.logger)
	response := builder.
		WithType(protocol.MessageTypeHeartbeat).
		WithString("pong").
		WithTarget(session.ID).
		Build()

	err := session.Connection.SendMessage(response)
	if err != nil {
		s.logger.Error("Failed to send heartbeat response",
			log.String("client_id", string(session.ID)),
			log.Error(err))
	}
}

// handleJoinGroup handles group join requests
func (s *Server) handleJoinGroup(session *ClientSession, groupName string) {
	if groupName == "" {
		return
	}

	clientLogger := s.logger.With(log.String("client_id", string(session.ID)))
	clientLogger.Info("Client joining group", log.String("group", groupName))

	// Get or create group
	groupInterface, _ := s.groups.LoadOrStore(groupName, &Group{
		Name:      groupName,
		CreatedAt: time.Now(),
	})
	group := groupInterface.(*Group)

	// Add client to group
	if _, loaded := group.Clients.LoadOrStore(session.ID, true); !loaded {
		atomic.AddInt64(&group.ClientCount, 1)
	}

	// Add group to client
	session.Groups.Store(groupName, true)

	clientLogger.Info("Client joined group successfully",
		log.String("group", groupName),
		log.Int64("group_members", atomic.LoadInt64(&group.ClientCount)))
}

// handleLeaveGroup handles group leave requests
func (s *Server) handleLeaveGroup(session *ClientSession, groupName string) {
	if groupName == "" {
		return
	}

	clientLogger := s.logger.With(log.String("client_id", string(session.ID)))
	clientLogger.Info("Client leaving group", log.String("group", groupName))

	// Remove from group
	if groupInterface, exists := s.groups.Load(groupName); exists {
		group := groupInterface.(*Group)
		if _, loaded := group.Clients.LoadAndDelete(session.ID); loaded {
			atomic.AddInt64(&group.ClientCount, -1)
		}
	}

	// Remove group from client
	session.Groups.Delete(groupName)

	clientLogger.Info("Client left group successfully", log.String("group", groupName))
}

// handleSubscribeScope handles scope subscription requests
func (s *Server) handleSubscribeScope(session *ClientSession, scopeName string) {
	if scopeName == "" {
		return
	}

	clientLogger := s.logger.With(log.String("client_id", string(session.ID)))
	clientLogger.Info("Client subscribing to scope", log.String("scope", scopeName))

	// Get or create scope
	scopeInterface, _ := s.scopes.LoadOrStore(scopeName, &Scope{
		Name:      scopeName,
		CreatedAt: time.Now(),
	})
	scope := scopeInterface.(*Scope)

	// Add client to scope
	if _, loaded := scope.Clients.LoadOrStore(session.ID, true); !loaded {
		atomic.AddInt64(&scope.ClientCount, 1)
	}

	// Add scope to client
	session.Scopes.Store(scopeName, true)

	clientLogger.Info("Client subscribed to scope successfully",
		log.String("scope", scopeName),
		log.Int64("scope_subscribers", atomic.LoadInt64(&scope.ClientCount)))
}

// handleUnsubscribeScope handles scope unsubscription requests
func (s *Server) handleUnsubscribeScope(session *ClientSession, scopeName string) {
	if scopeName == "" {
		return
	}

	clientLogger := s.logger.With(log.String("client_id", string(session.ID)))
	clientLogger.Info("Client unsubscribing from scope", log.String("scope", scopeName))

	// Remove from scope
	if scopeInterface, exists := s.scopes.Load(scopeName); exists {
		scope := scopeInterface.(*Scope)
		if _, loaded := scope.Clients.LoadAndDelete(session.ID); loaded {
			atomic.AddInt64(&scope.ClientCount, -1)
		}
	}

	// Remove scope from client
	session.Scopes.Delete(scopeName)

	clientLogger.Info("Client unsubscribed from scope successfully", log.String("scope", scopeName))
}

// removeClientFromAllGroups removes a client from all groups
func (s *Server) removeClientFromAllGroups(clientID protocol.ClientID) {
	s.groups.Range(func(key, value interface{}) bool {
		group := value.(*Group)
		if _, loaded := group.Clients.LoadAndDelete(clientID); loaded {
			atomic.AddInt64(&group.ClientCount, -1)
		}
		return true
	})
}

// removeClientFromAllScopes removes a client from all scopes
func (s *Server) removeClientFromAllScopes(clientID protocol.ClientID) {
	s.scopes.Range(func(key, value interface{}) bool {
		scope := value.(*Scope)
		if _, loaded := scope.Clients.LoadAndDelete(clientID); loaded {
			atomic.AddInt64(&scope.ClientCount, -1)
		}
		return true
	})
}

// BroadcastToGroup broadcasts a message to all clients in a group
func (s *Server) BroadcastToGroup(groupName string, msg *protocol.Message) error {
	groupInterface, exists := s.groups.Load(groupName)
	if !exists {
		return ErrGroupNotFound
	}

	group := groupInterface.(*Group)

	s.logger.Debug("Broadcasting to group",
		log.String("group", groupName),
		log.Int64("members", atomic.LoadInt64(&group.ClientCount)))

	group.Clients.Range(func(key, value interface{}) bool {
		clientID := key.(protocol.ClientID)
		if sessionInterface, exists := s.clients.Load(clientID); exists {
			session := sessionInterface.(*ClientSession)
			go func() {
				err := session.Connection.SendMessage(msg)
				if err != nil {
					s.logger.Error("Failed to send group message",
						log.String("client_id", string(clientID)),
						log.Error(err))
				}
			}()
		}
		return true
	})

	return nil
}

// GetStats returns server statistics
func (s *Server) GetStats() Stats {
	return Stats{
		ClientCount: atomic.LoadInt64(&s.clientCount),
		GroupCount:  atomic.LoadInt64(&s.groupCount),
		ScopeCount:  atomic.LoadInt64(&s.scopeCount),
		Running:     atomic.LoadInt32(&s.running) == 1,
	}
}

// Stats contains server statistics
type Stats struct {
	ClientCount int64
	GroupCount  int64
	ScopeCount  int64
	Running     bool
}

// startWorkers starts background worker goroutines
func (s *Server) startWorkers() {
	s.workerGroup.Add(1)

	// Health monitor
	go func() {
		defer s.workerGroup.Done()
		s.healthMonitor()
	}()
}

// stopWorkers stops background worker goroutines
func (s *Server) stopWorkers() {
	s.workerGroup.Wait()
}

// healthMonitor monitors server and client health
func (s *Server) healthMonitor() {
	s.logger.Debug("Health monitor started")

	ticker := time.NewTicker(s.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.performHealthChecks()
		case <-s.stopChan:
			s.logger.Debug("Health monitor stopped")
			return
		}
	}
}

// performHealthChecks performs health checks on clients
func (s *Server) performHealthChecks() {
	now := time.Now().Unix()
	timeoutSeconds := int64(s.config.ClientTimeout.Seconds())

	var disconnectedClients []protocol.ClientID

	s.clients.Range(func(key, value interface{}) bool {
		clientID := key.(protocol.ClientID)
		session := value.(*ClientSession)

		lastSeen := atomic.LoadInt64(&session.LastSeen)
		if now-lastSeen > timeoutSeconds {
			disconnectedClients = append(disconnectedClients, clientID)
		}

		return true
	})

	// Disconnect inactive clients
	for _, clientID := range disconnectedClients {
		if sessionInterface, exists := s.clients.Load(clientID); exists {
			session := sessionInterface.(*ClientSession)
			atomic.StoreInt32(&session.Active, 0)
			s.logger.Info("Disconnecting inactive client",
				log.String("client_id", string(clientID)))
		}
	}

	if len(disconnectedClients) > 0 {
		s.logger.Info("Health check completed",
			log.Int("disconnected_clients", len(disconnectedClients)),
			log.Int64("active_clients", atomic.LoadInt64(&s.clientCount)))
	}
}
