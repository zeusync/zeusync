package protocol

import (
	"context"
	"fmt"
	"github.com/zeusync/zeusync/internal/core/protocol/intrefaces"
	"net"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var _ intrefaces.Protocol = (*WebSocketProtocol)(nil)

// WebSocketProtocol implements the Protocol interface for WebSocket
type WebSocketProtocol struct {
	name    string
	version string
	config  intrefaces.ProtocolConfig
	server  *http.Server
	running int32
	mu      sync.RWMutex

	// Message handling
	handlers       map[string]intrefaces.MessageHandler
	defaultHandler intrefaces.MessageHandler
	middlewares    []intrefaces.ProtocolMiddleware

	// Client management
	clients   map[string]*WebSocketConnection
	clientsMu sync.RWMutex
	groups    map[string]map[string]struct{}
	groupsMu  sync.RWMutex

	// Metrics
	metrics *WebSocketMetrics
	logger  *logrus.Entry

	// WebSocket upgrader
	upgrader websocket.Upgrader

	// Worker pools
	messageWorkers chan *messageWork
	workerWg       sync.WaitGroup
	ctx            context.Context
	cancel         context.CancelFunc
}

// WebSocketMetrics extends the interface metrics with WebSocket-specific data
type WebSocketMetrics struct {
	intrefaces.ProtocolMetrics
	mu                   sync.RWMutex
	upgradeErrors        int64
	pingsSent            int64
	pongsReceived        int64
	connectionTimeouts   int64
	messageQueueOverflow int64
}

// messageWork represents work for message processing workers
type messageWork struct {
	client  *WebSocketConnection
	message intrefaces.Message
	ctx     context.Context
}

// NewWebSocketProtocol creates a new WebSocket protocol instance
func NewWebSocketProtocol(config intrefaces.ProtocolConfig, logger *logrus.Logger) *WebSocketProtocol {
	ctx, cancel := context.WithCancel(context.Background())

	return &WebSocketProtocol{
		name:     "WebSocket",
		version:  "1.0.0",
		config:   config,
		handlers: make(map[string]intrefaces.MessageHandler),
		clients:  make(map[string]*WebSocketConnection),
		groups:   make(map[string]map[string]struct{}),
		metrics:  &WebSocketMetrics{},
		logger:   logger.WithField("protocol", "websocket"),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  int(config.BufferSize),
			WriteBufferSize: int(config.BufferSize),
			CheckOrigin: func(r *http.Request) bool {
				// In production, implement proper origin checking
				return true
			},
			EnableCompression: config.EnableCompression,
		},
		messageWorkers: make(chan *messageWork, config.QueueSize),
		ctx:            ctx,
		cancel:         cancel,
	}
}

// Name returns the protocol name
func (p *WebSocketProtocol) Name() string {
	return p.name
}

// Version returns the protocol version
func (p *WebSocketProtocol) Version() string {
	return p.version
}

// Type returns the protocol type
func (p *WebSocketProtocol) Type() intrefaces.ProtocolType {
	if p.config.TLSEnabled {
		return intrefaces.ProtocolWebSocketSecure
	}
	return intrefaces.ProtocolWebSocket
}

// Start starts the WebSocket protocol server
func (p *WebSocketProtocol) Start(ctx context.Context, config intrefaces.ProtocolConfig) error {
	if !atomic.CompareAndSwapInt32(&p.running, 0, 1) {
		return errors.New("protocol is already running")
	}

	p.mu.Lock()
	p.config = config
	p.mu.Unlock()

	// Start worker pool
	p.startWorkers()

	// Setup HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", p.handleWebSocket)
	mux.HandleFunc("/health", p.handleHealth)
	mux.HandleFunc("/metrics", p.handleMetrics)

	addr := net.JoinHostPort(p.config.Host, fmt.Sprintf("%d", p.config.Port))
	p.server = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  p.config.ReadTimeout,
		WriteTimeout: p.config.WriteTimeout,
		IdleTimeout:  p.config.KeepAliveTimeout,
	}

	// Start server
	go func() {
		var err error
		if p.config.TLSEnabled {
			err = p.server.ListenAndServeTLS(p.config.CertFile, p.config.KeyFile)
		} else {
			err = p.server.ListenAndServe()
		}

		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			p.logger.WithError(err).Error("WebSocket server error")
		}
	}()

	// Start metrics collection
	go p.collectMetrics()

	p.logger.Infof("WebSocket protocol started on %s", addr)
	return nil
}

// Stop stops the WebSocket protocol server
func (p *WebSocketProtocol) Stop(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&p.running, 1, 0) {
		return errors.New("protocol is not running")
	}

	// Cancel context to stop workers
	p.cancel()

	// Close all client connections
	p.clientsMu.Lock()
	for _, client := range p.clients {
		_ = client.Close()
	}
	p.clients = make(map[string]*WebSocketConnection)
	p.clientsMu.Unlock()

	// Stop HTTP server
	if p.server != nil {
		if err := p.server.Shutdown(ctx); err != nil {
			return errors.Wrap(err, "failed to shutdown HTTP server")
		}
	}

	// Wait for workers to finish
	p.workerWg.Wait()

	p.logger.Info("WebSocket protocol stopped")
	return nil
}

// Restart restarts the protocol
func (p *WebSocketProtocol) Restart(ctx context.Context) error {
	if err := p.Stop(ctx); err != nil {
		return err
	}
	return p.Start(ctx, p.config)
}

// IsRunning returns true if the protocol is running
func (p *WebSocketProtocol) IsRunning() bool {
	return atomic.LoadInt32(&p.running) == 1
}

// startWorkers starts the message processing worker pool
func (p *WebSocketProtocol) startWorkers() {
	workerCount := p.config.WorkerCount
	if workerCount == 0 {
		workerCount = 10 // Default worker count
	}

	for i := uint32(0); i < workerCount; i++ {
		p.workerWg.Add(1)
		go p.messageWorker()
	}
}

// messageWorker processes messages from the work queue
func (p *WebSocketProtocol) messageWorker() {
	defer p.workerWg.Done()

	for {
		select {
		case work := <-p.messageWorkers:
			p.processMessage(work)
		case <-p.ctx.Done():
			return
		}
	}
}

// processMessage processes a single message
func (p *WebSocketProtocol) processMessage(work *messageWork) {
	defer func() {
		if r := recover(); r != nil {
			p.logger.WithField("panic", r).Error("Message processing panic")
		}
	}()

	// Apply middleware before handling
	for _, mw := range p.middlewares {
		if err := mw.BeforeHandle(work.ctx, work.client.ClientInfo(), work.message); err != nil {
			p.logger.WithError(err).Error("Middleware BeforeHandle error")
			return
		}
	}

	// Find handler
	handler, ok := p.handlers[work.message.Type()]
	if !ok {
		handler = p.defaultHandler
	}

	var response intrefaces.Message
	var err error

	if handler != nil {
		err = handler(work.ctx, work.client.ClientInfo(), work.message)
	} else {
		err = errors.New("no handler found for message type")
	}

	// Apply middleware after handling
	for i := len(p.middlewares) - 1; i >= 0; i-- {
		mw := p.middlewares[i]
		if mwErr := mw.AfterHandle(work.ctx, work.client.ClientInfo(), work.message, response, err); mwErr != nil {
			p.logger.WithError(mwErr).Error("Middleware AfterHandle error")
		}
	}

	// Update metrics
	atomic.AddUint64(&p.metrics.MessagesReceived, 1)
}

// handleWebSocket handles WebSocket upgrade requests
func (p *WebSocketProtocol) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Upgrade connection
	conn, err := p.upgrader.Upgrade(w, r, nil)
	if err != nil {
		p.logger.WithError(err).Error("WebSocket upgrade failed")
		atomic.AddInt64(&p.metrics.upgradeErrors, 1)
		return
	}

	// Create client connection
	client := NewWebSocketConnection(conn, p.config)

	// Add to clients map
	p.clientsMu.Lock()
	p.clients[client.ID()] = client
	p.clientsMu.Unlock()

	// Update metrics
	atomic.AddUint64(&p.metrics.ActiveConnections, 1)
	atomic.AddInt64(&p.metrics.TotalConnections, 1)

	// Apply connection middleware
	for _, mw := range p.middlewares {
		if err := mw.OnConnect(p.ctx, client.ClientInfo()); err != nil {
			p.logger.WithError(err).Error("Middleware OnConnect error")
		}
	}

	p.logger.WithField("client_id", client.ID()).Info("Client connected")

	// Handle client messages
	go p.handleClient(client)
}

// handleClient handles messages from a specific client
func (p *WebSocketProtocol) handleClient(client *WebSocketConnection) {
	defer func() {
		// Remove from clients map
		p.clientsMu.Lock()
		delete(p.clients, client.ID())
		p.clientsMu.Unlock()

		// Remove from all groups
		p.groupsMu.Lock()
		for groupID, members := range p.groups {
			delete(members, client.ID())
			if len(members) == 0 {
				delete(p.groups, groupID)
			}
		}
		p.groupsMu.Unlock()

		// Apply disconnect middleware
		for _, mw := range p.middlewares {
			if err := mw.OnDisconnect(p.ctx, client.ClientInfo(), "connection closed"); err != nil {
				p.logger.WithError(err).Error("Middleware OnDisconnect error")
			}
		}

		// Update metrics
		atomic.AddUint64(&p.metrics.ActiveConnections, ^uint64(0)) // Decrement

		_ = client.Close()
		p.logger.WithField("client_id", client.ID()).Info("Client disconnected")
	}()

	// Set up ping/pong handling
	client.SetPongHandler(func(string) error {
		atomic.AddInt64(&p.metrics.pongsReceived, 1)
		return nil
	})

	// Start ping ticker
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	go func() {
		for {
			select {
			case <-pingTicker.C:
				if err := client.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(10*time.Second)); err != nil {
					p.logger.WithError(err).Error("Failed to send ping")
					return
				}
				atomic.AddInt64(&p.metrics.pingsSent, 1)
			case <-p.ctx.Done():
				return
			}
		}
	}()

	// Read messages
	for {
		message, err := client.ReceiveMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				p.logger.WithError(err).Error("WebSocket error")
			}
			return
		}

		// Queue message for processing
		select {
		case p.messageWorkers <- &messageWork{
			client:  client,
			message: message,
			ctx:     p.ctx,
		}:
		default:
			// Queue is full
			atomic.AddInt64(&p.metrics.messageQueueOverflow, 1)
			p.logger.Warn("Message queue overflow, dropping message")
		}
	}
}

// handleHealth handles health check requests
func (p *WebSocketProtocol) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, `{"status":"healthy","connections":%d}`, p.GetConnectionCount())
}

// handleMetrics handles metrics requests
func (p *WebSocketProtocol) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	metrics := p.GetMetrics()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Simple JSON response with metrics
	_, _ = fmt.Fprintf(w, `{
		"active_connections": %d,
		"total_connections": %d,
		"failed_connections": %d,
		"messages_sent": %d,
		"messages_received": %d,
		"upgrade_errors": %d,
		"pings_sent": %d,
		"pongs_received": %d
	}`,
		metrics.ActiveConnections,
		metrics.TotalConnections,
		metrics.FailedConnections,
		metrics.MessagesSent,
		metrics.MessagesReceived,
		p.metrics.upgradeErrors,
		p.metrics.pingsSent,
		p.metrics.pongsReceived,
	)
}

// collectMetrics periodically collects and updates metrics
func (p *WebSocketProtocol) collectMetrics() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var lastMessagesSent, lastMessagesReceived uint64
	var lastTime = time.Now()

	for {
		select {
		case <-ticker.C:
			now := time.Now()
			duration := now.Sub(lastTime).Seconds()

			currentSent := atomic.LoadUint64(&p.metrics.MessagesSent)
			currentReceived := atomic.LoadUint64(&p.metrics.MessagesReceived)

			if duration > 0 {
				p.metrics.mu.Lock()
				p.metrics.MessagesPerSecond = float64(currentSent+currentReceived-lastMessagesSent-lastMessagesReceived) / duration
				p.metrics.mu.Unlock()
			}

			lastMessagesSent = currentSent
			lastMessagesReceived = currentReceived
			lastTime = now

		case <-p.ctx.Done():
			return
		}
	}
}

// RegisterHandler registers a message handler for a specific message type
func (p *WebSocketProtocol) RegisterHandler(messageType string, handler intrefaces.MessageHandler) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.handlers[messageType]; exists {
		return errors.Errorf("handler for message type '%s' already registered", messageType)
	}

	p.handlers[messageType] = handler
	return nil
}

// UnregisterHandler unregisters a message handler
func (p *WebSocketProtocol) UnregisterHandler(messageType string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.handlers[messageType]; !exists {
		return errors.Errorf("no handler registered for message type '%s'", messageType)
	}

	delete(p.handlers, messageType)
	return nil
}

// GetHandler returns the handler for a specific message type
func (p *WebSocketProtocol) GetHandler(messageType string) (intrefaces.MessageHandler, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	handler, exists := p.handlers[messageType]
	return handler, exists
}

// SetDefaultHandler sets the default message handler
func (p *WebSocketProtocol) SetDefaultHandler(handler intrefaces.MessageHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.defaultHandler = handler
}

// Send sends a message to a specific client
func (p *WebSocketProtocol) Send(clientID string, message intrefaces.Message) error {
	p.clientsMu.RLock()
	client, exists := p.clients[clientID]
	p.clientsMu.RUnlock()

	if !exists {
		return errors.Errorf("client '%s' not found", clientID)
	}

	if err := client.SendMessage(message); err != nil {
		return errors.Wrap(err, "failed to send message to client")
	}

	atomic.AddUint64(&p.metrics.MessagesSent, 1)
	return nil
}

// SendToMultiple sends a message to multiple clients
func (p *WebSocketProtocol) SendToMultiple(clientIDs []string, message intrefaces.Message) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(clientIDs))

	for _, clientID := range clientIDs {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			if err := p.Send(id, message); err != nil {
				errChan <- err
			}
		}(clientID)
	}

	wg.Wait()
	close(errChan)

	// Collect errors
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return errors.Errorf("failed to send to %d clients", len(errs))
	}

	return nil
}

// Broadcast sends a message to all connected clients
func (p *WebSocketProtocol) Broadcast(message intrefaces.Message) error {
	p.clientsMu.RLock()
	clientIDs := make([]string, 0, len(p.clients))
	for id := range p.clients {
		clientIDs = append(clientIDs, id)
	}
	p.clientsMu.RUnlock()

	return p.SendToMultiple(clientIDs, message)
}

// BroadcastExcept sends a message to all clients except the specified ones
func (p *WebSocketProtocol) BroadcastExcept(excludeClientIDs []string, message intrefaces.Message) error {
	excludeMap := make(map[string]struct{}, len(excludeClientIDs))
	for _, id := range excludeClientIDs {
		excludeMap[id] = struct{}{}
	}

	p.clientsMu.RLock()
	var clientIDs []string
	for id := range p.clients {
		if _, excluded := excludeMap[id]; !excluded {
			clientIDs = append(clientIDs, id)
		}
	}
	p.clientsMu.RUnlock()

	return p.SendToMultiple(clientIDs, message)
}

// GetClient returns information about a specific client
func (p *WebSocketProtocol) GetClient(clientID string) (intrefaces.ClientInfo, bool) {
	p.clientsMu.RLock()
	client, exists := p.clients[clientID]
	p.clientsMu.RUnlock()

	if !exists {
		return intrefaces.ClientInfo{}, false
	}

	return client.ClientInfo(), true
}

// GetAllClients returns a list of all connected clients
func (p *WebSocketProtocol) GetAllClients() []intrefaces.ClientInfo {
	p.clientsMu.RLock()
	defer p.clientsMu.RUnlock()

	clients := make([]intrefaces.ClientInfo, 0, len(p.clients))
	for _, client := range p.clients {
		clients = append(clients, client.ClientInfo())
	}

	return clients
}

// DisconnectClient disconnects a client
func (p *WebSocketProtocol) DisconnectClient(clientID string, reason string) error {
	p.clientsMu.RLock()
	client, exists := p.clients[clientID]
	p.clientsMu.RUnlock()

	if !exists {
		return errors.Errorf("client '%s' not found", clientID)
	}

	return client.CloseWithReason(reason)
}

// GetConnectionCount returns the number of connected clients
func (p *WebSocketProtocol) GetConnectionCount() int {
	p.clientsMu.RLock()
	defer p.clientsMu.RUnlock()
	return len(p.clients)
}

// CreateGroup creates a new client group
func (p *WebSocketProtocol) CreateGroup(groupID string) error {
	p.groupsMu.Lock()
	defer p.groupsMu.Unlock()

	if _, exists := p.groups[groupID]; exists {
		return errors.Errorf("group '%s' already exists", groupID)
	}

	p.groups[groupID] = make(map[string]struct{})
	return nil
}

// DeleteGroup deletes a client group
func (p *WebSocketProtocol) DeleteGroup(groupID string) error {
	p.groupsMu.Lock()
	defer p.groupsMu.Unlock()

	if _, exists := p.groups[groupID]; !exists {
		return errors.Errorf("group '%s' not found", groupID)
	}

	delete(p.groups, groupID)
	return nil
}

// JoinGroup adds a client to a group
func (p *WebSocketProtocol) JoinGroup(clientID, groupID string) error {
	p.groupsMu.Lock()
	defer p.groupsMu.Unlock()

	group, exists := p.groups[groupID]
	if !exists {
		return errors.Errorf("group '%s' not found", groupID)
	}

	// Verify client exists
	p.clientsMu.RLock()
	_, clientExists := p.clients[clientID]
	p.clientsMu.RUnlock()

	if !clientExists {
		return errors.Errorf("client '%s' not found", clientID)
	}

	group[clientID] = struct{}{}
	return nil
}

// LeaveGroup removes a client from a group
func (p *WebSocketProtocol) LeaveGroup(clientID, groupID string) error {
	p.groupsMu.Lock()
	defer p.groupsMu.Unlock()

	group, exists := p.groups[groupID]
	if !exists {
		return errors.Errorf("group '%s' not found", groupID)
	}

	delete(group, clientID)
	return nil
}

// SendToGroup sends a message to all clients in a group
func (p *WebSocketProtocol) SendToGroup(groupID string, message intrefaces.Message) error {
	p.groupsMu.RLock()
	group, exists := p.groups[groupID]
	if !exists {
		p.groupsMu.RUnlock()
		return errors.Errorf("group '%s' not found", groupID)
	}

	clientIDs := make([]string, 0, len(group))
	for id := range group {
		clientIDs = append(clientIDs, id)
	}
	p.groupsMu.RUnlock()

	return p.SendToMultiple(clientIDs, message)
}

// GetGroupMembers returns a list of client IDs in a group
func (p *WebSocketProtocol) GetGroupMembers(groupID string) ([]string, error) {
	p.groupsMu.RLock()
	defer p.groupsMu.RUnlock()

	group, exists := p.groups[groupID]
	if !exists {
		return nil, errors.Errorf("group '%s' not found", groupID)
	}

	members := make([]string, 0, len(group))
	for id := range group {
		members = append(members, id)
	}

	return members, nil
}

// AddMiddleware adds a protocol middleware
func (p *WebSocketProtocol) AddMiddleware(middleware intrefaces.ProtocolMiddleware) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check for duplicate middleware
	for _, mw := range p.middlewares {
		if mw.Name() == middleware.Name() {
			return errors.Errorf("middleware '%s' already exists", middleware.Name())
		}
	}

	p.middlewares = append(p.middlewares, middleware)

	// Sort by priority (higher priority first)
	sort.Slice(p.middlewares, func(i, j int) bool {
		return p.middlewares[i].Priority() > p.middlewares[j].Priority()
	})

	return nil
}

// RemoveMiddleware removes a protocol middleware
func (p *WebSocketProtocol) RemoveMiddleware(name string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i, mw := range p.middlewares {
		if mw.Name() == name {
			p.middlewares = append(p.middlewares[:i], p.middlewares[i+1:]...)
			return nil
		}
	}

	return errors.Errorf("middleware '%s' not found", name)
}

// GetMetrics returns protocol metrics
func (p *WebSocketProtocol) GetMetrics() intrefaces.ProtocolMetrics {
	p.metrics.mu.RLock()
	defer p.metrics.mu.RUnlock()

	return intrefaces.ProtocolMetrics{
		ActiveConnections:    atomic.LoadUint64(&p.metrics.ActiveConnections),
		TotalConnections:     atomic.LoadInt64(&p.metrics.TotalConnections),
		FailedConnections:    atomic.LoadInt64(&p.metrics.FailedConnections),
		MessagesSent:         atomic.LoadUint64(&p.metrics.MessagesSent),
		MessagesReceived:     atomic.LoadUint64(&p.metrics.MessagesReceived),
		MessagesPerSecond:    p.metrics.MessagesPerSecond,
		ConnectionsPerSecond: 0, // TODO: Implement
		AverageMessageSize:   0, // TODO: Implement
	}
}

// GetClientMetrics returns metrics for a specific client
func (p *WebSocketProtocol) GetClientMetrics(clientID string) (intrefaces.ClientMetrics, bool) {
	p.clientsMu.RLock()
	client, exists := p.clients[clientID]
	p.clientsMu.RUnlock()

	if !exists {
		return intrefaces.ClientMetrics{}, false
	}

	return client.GetMetrics(), true
}

// GetConfig returns the protocol configuration
func (p *WebSocketProtocol) GetConfig() intrefaces.ProtocolConfig {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.config
}

// UpdateConfig updates the protocol configuration
func (p *WebSocketProtocol) UpdateConfig(config intrefaces.ProtocolConfig) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// In a production system, you'd want to validate the config
	// and potentially restart certain components
	p.config = config
	return nil
}
