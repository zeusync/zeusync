package quic

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"github.com/quic-go/quic-go"
	"github.com/zeusync/zeusync/internal/core/observability/log"
	"github.com/zeusync/zeusync/internal/core/protocol"
	"math/big"
	"net"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
)

var _ protocol.Protocol = (*QUIC)(nil)

// QUIC implements the QUIC interface for QUIC
type QUIC struct {
	name     string
	version  string
	config   protocol.Config
	listener *quic.Listener
	running  int32
	mu       sync.RWMutex

	// IMessage handling
	handlers       map[string]protocol.MessageHandler
	defaultHandler protocol.MessageHandler
	middlewares    []protocol.Middleware

	// Client management
	clients   map[string]*Connection
	clientsMu sync.RWMutex
	groups    map[string]map[string]struct{}
	groupsMu  sync.RWMutex

	// Metrics
	metrics *Metrics
	logger  log.Log

	// Worker pools
	messageWorkers chan *quicMessageWork
	workerWg       sync.WaitGroup
	ctx            context.Context
	cancel         context.CancelFunc
}

// Metrics extends the interface metrics with QUIC-specific data
type Metrics struct {
	protocol.Metrics
	mu                   sync.RWMutex
	handshakeErrors      int64
	streamErrors         int64
	connectionTimeouts   int64
	messageQueueOverflow int64
	packetsLost          int64
	rttAverage           time.Duration
}

// quicMessageWork represents work for message processing workers
type quicMessageWork struct {
	client  *Connection
	message protocol.IMessage
	ctx     context.Context
}

// NewQuicProtocol creates a new QUIC protocol instance
func NewQuicProtocol(config protocol.Config, logger log.Log) *QUIC {
	ctx, cancel := context.WithCancel(context.Background())

	return &QUIC{
		name:           "QUIC",
		version:        "1.0.0",
		config:         config,
		handlers:       make(map[string]protocol.MessageHandler),
		clients:        make(map[string]*Connection),
		groups:         make(map[string]map[string]struct{}),
		metrics:        &Metrics{},
		logger:         logger.With(log.String("protocol", "quic")),
		messageWorkers: make(chan *quicMessageWork, config.QueueSize),
		ctx:            ctx,
		cancel:         cancel,
	}
}

// Name returns the protocol name
func (q *QUIC) Name() string {
	return q.name
}

// Version returns the protocol version
func (q *QUIC) Version() string {
	return q.version
}

// Type returns the protocol type
func (q *QUIC) Type() protocol.Type {
	return protocol.Custom
}

// Start starts the QUIC protocol listener
func (q *QUIC) Start(ctx context.Context, config protocol.Config) error {
	if !atomic.CompareAndSwapInt32(&q.running, 0, 1) {
		return errors.New("protocol is already running")
	}

	q.mu.Lock()
	q.config = config
	q.mu.Unlock()

	// Start worker pool
	q.startWorkers()

	tlsConfig, err := q.createTLSConfig()
	if err != nil {
		return errors.Wrap(err, "failed to create TLS config")
	}

	// Configure QUIC
	quicConfig := &quic.Config{
		MaxIdleTimeout:        q.config.KeepAliveTimeout,
		MaxIncomingStreams:    int64(q.config.MaxConnections),
		MaxIncomingUniStreams: int64(q.config.MaxConnections / 2),
	}

	addr := net.JoinHostPort(q.config.Host, fmt.Sprintf("%d", q.config.Port))
	listener, err := quic.ListenAddr(addr, tlsConfig, quicConfig)
	if err != nil {
		return errors.Wrap(err, "failed to start QUIC listener")
	}

	q.listener = listener
	q.logger.Info("QUIC protocol started", log.String("address", addr))

	// Start accepting connections
	go q.acceptConnections(ctx)

	// Start metrics collection
	go q.collectMetrics()

	return nil
}

// Stop stops the QUIC protocol listener
func (q *QUIC) Stop(_ context.Context) error {
	if !atomic.CompareAndSwapInt32(&q.running, 1, 0) {
		return errors.New("protocol is not running")
	}

	// Cancel context to stop workers
	q.cancel()

	// Close all client connections
	q.clientsMu.Lock()
	for _, client := range q.clients {
		_ = client.Close()
	}
	q.clients = make(map[string]*Connection)
	q.clientsMu.Unlock()

	// Close listener
	if q.listener != nil {
		if err := q.listener.Close(); err != nil {
			return errors.Wrap(err, "failed to close QUIC listener")
		}
	}

	// Wait for workers to finish
	q.workerWg.Wait()

	q.logger.Info("QUIC protocol stopped")
	return nil
}

// Restart restarts the protocol
func (q *QUIC) Restart(ctx context.Context) error {
	if err := q.Stop(ctx); err != nil {
		return err
	}
	return q.Start(ctx, q.config)
}

// IsRunning returns true if the protocol is running
func (q *QUIC) IsRunning() bool {
	return atomic.LoadInt32(&q.running) == 1
}

// startWorkers starts the message processing worker pool
func (q *QUIC) startWorkers() {
	workerCount := q.config.WorkerCount
	if workerCount == 0 {
		workerCount = 10 // Default worker count
	}

	for i := uint32(0); i < workerCount; i++ {
		q.workerWg.Add(1)
		go q.messageWorker()
	}
}

// messageWorker processes messages from the work queue
func (q *QUIC) messageWorker() {
	defer q.workerWg.Done()

	for {
		select {
		case work := <-q.messageWorkers:
			q.processMessage(work)
		case <-q.ctx.Done():
			return
		}
	}
}

// processMessage processes a single message
func (q *QUIC) processMessage(work *quicMessageWork) {
	defer func() {
		if r := recover(); r != nil {
			q.logger.Error("IMessage processing panic", log.Any("panic", r))
		}
	}()

	// Apply middleware before handling
	for _, mw := range q.middlewares {
		if err := mw.BeforeHandle(work.ctx, work.client.ClientInfo(), work.message); err != nil {
			q.logger.Error("Middleware BeforeHandle error", log.Error(err))
			return
		}
	}

	// Find handler
	handler, ok := q.handlers[work.message.Type()]
	if !ok {
		handler = q.defaultHandler
	}

	var response = &protocol.Message{}
	var err error

	if handler != nil {
		err = handler(work.ctx, work.client.ClientInfo(), work.message)
	} else {
		err = errors.New("no handler found for message type")
	}

	// Apply middleware after handling
	for i := len(q.middlewares) - 1; i >= 0; i-- {
		mw := q.middlewares[i]
		if mwErr := mw.AfterHandle(work.ctx, work.client.ClientInfo(), work.message, response, err); mwErr != nil {
			q.logger.Error("Middleware AfterHandle error", log.Error(mwErr))
		}
	}

	// Update metrics
	atomic.AddUint64(&q.metrics.MessagesReceived, 1)
}

func (q *QUIC) createTLSConfig() (*tls.Config, error) {
	if !q.config.TLSEnabled {
		// Generate self-signed certificate for development
		return q.generateTLSConfig(), nil
	}

	cert, err := tls.LoadX509KeyPair(q.config.CertFile, q.config.KeyFile)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load TLS certificate")
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"zeusync-quic"},
		MinVersion:   tls.VersionTLS13, // QUIC requires TLS 1.3
	}, nil
}

// generateTLSConfig generates a self-signed TLS config for development
func (q *QUIC) generateTLSConfig() *tls.Config {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"ZeuSync"},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"San Francisco"},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		DNSNames:    []string{"localhost"},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		panic(err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		NextProtos:   []string{"zeusync-quic"},
		MinVersion:   tls.VersionTLS13,
	}
}

func (q *QUIC) acceptConnections(ctx context.Context) {
	for {
		conn, err := q.listener.Accept(ctx)
		if err != nil {
			if q.IsRunning() {
				q.logger.Error("Failed to accept connection", log.Error(err))
				atomic.AddInt64(&q.metrics.handshakeErrors, 1)
			}
			return
		}

		go q.handleConnection(ctx, conn)
	}
}

func (q *QUIC) handleConnection(ctx context.Context, sess *quic.Conn) {
	client := NewQuicConnection(sess, q.config)

	// Add to clients map
	q.clientsMu.Lock()
	q.clients[client.ID()] = client
	q.clientsMu.Unlock()

	// Update metrics
	atomic.AddUint64(&q.metrics.ActiveConnections, 1)
	atomic.AddInt64(&q.metrics.TotalConnections, 1)

	// Apply connection middleware
	for _, mw := range q.middlewares {
		if err := mw.OnConnect(ctx, client.ClientInfo()); err != nil {
			q.logger.Error("Middleware OnConnect error", log.Error(err))
		}
	}

	q.logger.Info("QUIC client connected", log.String("client_id", client.ID()))

	// Handle client in separate goroutine
	go q.handleClient(ctx, client)
}

func (q *QUIC) handleClient(ctx context.Context, client *Connection) {
	defer func() {
		// Remove from clients map
		q.clientsMu.Lock()
		delete(q.clients, client.ID())
		q.clientsMu.Unlock()

		// Remove from all groups
		q.groupsMu.Lock()
		for groupID, members := range q.groups {
			delete(members, client.ID())
			if len(members) == 0 {
				delete(q.groups, groupID)
			}
		}
		q.groupsMu.Unlock()

		// Apply disconnect middleware
		for _, mw := range q.middlewares {
			if err := mw.OnDisconnect(ctx, client.ClientInfo(), "connection closed"); err != nil {
				q.logger.Error("Middleware OnDisconnect error", log.Error(err))
			}
		}

		// Update metrics
		atomic.AddUint64(&q.metrics.ActiveConnections, ^uint64(0)) // Decrement

		_ = client.Close()
		q.logger.Info("QUIC client disconnected", log.String("client_id", client.ID()))
	}()

	// Read messages from client
	for {
		message, err := client.ReceiveMessage()
		if err != nil {
			q.logger.Debug("Failed to receive message from client", log.Error(err))
			return
		}

		// Queue message for processing
		select {
		case q.messageWorkers <- &quicMessageWork{
			client:  client,
			message: message,
			ctx:     ctx,
		}:
		default:
			// Queue is full
			atomic.AddInt64(&q.metrics.messageQueueOverflow, 1)
			q.logger.Warn("IMessage queue overflow, dropping message")
		}
	}
}

// collectMetrics periodically collects and updates metrics
func (q *QUIC) collectMetrics() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var lastMessagesSent, lastMessagesReceived uint64
	var lastTime = time.Now()

	for {
		select {
		case <-ticker.C:
			now := time.Now()
			duration := now.Sub(lastTime).Seconds()

			currentSent := atomic.LoadUint64(&q.metrics.MessagesSent)
			currentReceived := atomic.LoadUint64(&q.metrics.MessagesReceived)

			if duration > 0 {
				q.metrics.mu.Lock()
				q.metrics.MessagesPerSecond = float64(currentSent+currentReceived-lastMessagesSent-lastMessagesReceived) / duration
				q.metrics.mu.Unlock()
			}

			lastMessagesSent = currentSent
			lastMessagesReceived = currentReceived
			lastTime = now

		case <-q.ctx.Done():
			return
		}
	}
}

// RegisterHandler registers a message handler for a specific message type
func (q *QUIC) RegisterHandler(messageType string, handler protocol.MessageHandler) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if _, exists := q.handlers[messageType]; exists {
		return errors.Errorf("handler for message type '%s' already registered", messageType)
	}

	q.handlers[messageType] = handler
	return nil
}

// UnregisterHandler unregisters a message handler
func (q *QUIC) UnregisterHandler(messageType string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if _, exists := q.handlers[messageType]; !exists {
		return errors.Errorf("no handler registered for message type '%s'", messageType)
	}

	delete(q.handlers, messageType)
	return nil
}

// GetHandler returns the handler for a specific message type
func (q *QUIC) GetHandler(messageType string) (protocol.MessageHandler, bool) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	handler, exists := q.handlers[messageType]
	return handler, exists
}

// SetDefaultHandler sets the default message handler
func (q *QUIC) SetDefaultHandler(handler protocol.MessageHandler) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.defaultHandler = handler
}

// Send sends a message to a specific client
func (q *QUIC) Send(clientID string, message protocol.IMessage) error {
	q.clientsMu.RLock()
	client, exists := q.clients[clientID]
	q.clientsMu.RUnlock()

	if !exists {
		return errors.Errorf("client '%s' not found", clientID)
	}

	if err := client.SendMessage(message); err != nil {
		return errors.Wrap(err, "failed to send message to client")
	}

	atomic.AddUint64(&q.metrics.MessagesSent, 1)
	return nil
}

// SendToMultiple sends a message to multiple clients
func (q *QUIC) SendToMultiple(clientIDs []string, message protocol.IMessage) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(clientIDs))

	for _, clientID := range clientIDs {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			if err := q.Send(id, message); err != nil {
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
func (q *QUIC) Broadcast(message protocol.IMessage) error {
	q.clientsMu.RLock()
	clientIDs := make([]string, 0, len(q.clients))
	for id := range q.clients {
		clientIDs = append(clientIDs, id)
	}
	q.clientsMu.RUnlock()

	return q.SendToMultiple(clientIDs, message)
}

// BroadcastExcept sends a message to all clients except the specified ones
func (q *QUIC) BroadcastExcept(excludeClientIDs []string, message protocol.IMessage) error {
	excludeMap := make(map[string]struct{}, len(excludeClientIDs))
	for _, id := range excludeClientIDs {
		excludeMap[id] = struct{}{}
	}

	q.clientsMu.RLock()
	var clientIDs []string
	for id := range q.clients {
		if _, excluded := excludeMap[id]; !excluded {
			clientIDs = append(clientIDs, id)
		}
	}
	q.clientsMu.RUnlock()

	return q.SendToMultiple(clientIDs, message)
}

// GetClient returns information about a specific client
func (q *QUIC) GetClient(clientID string) (protocol.ClientInfo, bool) {
	q.clientsMu.RLock()
	client, exists := q.clients[clientID]
	q.clientsMu.RUnlock()

	if !exists {
		return protocol.ClientInfo{}, false
	}

	return client.ClientInfo(), true
}

// GetAllClients returns a list of all connected clients
func (q *QUIC) GetAllClients() []protocol.ClientInfo {
	q.clientsMu.RLock()
	defer q.clientsMu.RUnlock()

	clients := make([]protocol.ClientInfo, 0, len(q.clients))
	for _, client := range q.clients {
		clients = append(clients, client.ClientInfo())
	}

	return clients
}

// DisconnectClient disconnects a client
func (q *QUIC) DisconnectClient(clientID string, reason string) error {
	q.clientsMu.RLock()
	client, exists := q.clients[clientID]
	q.clientsMu.RUnlock()

	if !exists {
		return errors.Errorf("client '%s' not found", clientID)
	}

	return client.CloseWithReason(reason)
}

// GetConnectionCount returns the number of connected clients
func (q *QUIC) GetConnectionCount() int {
	q.clientsMu.RLock()
	defer q.clientsMu.RUnlock()
	return len(q.clients)
}

// CreateGroup creates a new client group
func (q *QUIC) CreateGroup(groupID string) error {
	q.groupsMu.Lock()
	defer q.groupsMu.Unlock()

	if _, exists := q.groups[groupID]; exists {
		return errors.Errorf("group '%s' already exists", groupID)
	}

	q.groups[groupID] = make(map[string]struct{})
	return nil
}

// DeleteGroup deletes a client group
func (q *QUIC) DeleteGroup(groupID string) error {
	q.groupsMu.Lock()
	defer q.groupsMu.Unlock()

	if _, exists := q.groups[groupID]; !exists {
		return errors.Errorf("group '%s' not found", groupID)
	}

	delete(q.groups, groupID)
	return nil
}

// JoinGroup adds a client to a group
func (q *QUIC) JoinGroup(clientID, groupID string) error {
	q.groupsMu.Lock()
	defer q.groupsMu.Unlock()

	group, exists := q.groups[groupID]
	if !exists {
		return errors.Errorf("group '%s' not found", groupID)
	}

	// Verify client exists
	q.clientsMu.RLock()
	_, clientExists := q.clients[clientID]
	q.clientsMu.RUnlock()

	if !clientExists {
		return errors.Errorf("client '%s' not found", clientID)
	}

	group[clientID] = struct{}{}
	return nil
}

// LeaveGroup removes a client from a group
func (q *QUIC) LeaveGroup(clientID, groupID string) error {
	q.groupsMu.Lock()
	defer q.groupsMu.Unlock()

	group, exists := q.groups[groupID]
	if !exists {
		return errors.Errorf("group '%s' not found", groupID)
	}

	delete(group, clientID)
	return nil
}

// SendToGroup sends a message to all clients in a group
func (q *QUIC) SendToGroup(groupID string, message protocol.IMessage) error {
	q.groupsMu.RLock()
	group, exists := q.groups[groupID]
	if !exists {
		q.groupsMu.RUnlock()
		return errors.Errorf("group '%s' not found", groupID)
	}

	clientIDs := make([]string, 0, len(group))
	for id := range group {
		clientIDs = append(clientIDs, id)
	}
	q.groupsMu.RUnlock()

	return q.SendToMultiple(clientIDs, message)
}

// GetGroupMembers returns a list of client IDs in a group
func (q *QUIC) GetGroupMembers(groupID string) ([]string, error) {
	q.groupsMu.RLock()
	defer q.groupsMu.RUnlock()

	group, exists := q.groups[groupID]
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
func (q *QUIC) AddMiddleware(middleware protocol.Middleware) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Check for duplicate middleware
	for _, mw := range q.middlewares {
		if mw.Name() == middleware.Name() {
			return errors.Errorf("middleware '%s' already exists", middleware.Name())
		}
	}

	q.middlewares = append(q.middlewares, middleware)

	// Sort by priority (higher priority first)
	sort.Slice(q.middlewares, func(i, j int) bool {
		return q.middlewares[i].Priority() > q.middlewares[j].Priority()
	})

	return nil
}

// RemoveMiddleware removes a protocol middleware
func (q *QUIC) RemoveMiddleware(name string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	for i, mw := range q.middlewares {
		if mw.Name() == name {
			q.middlewares = append(q.middlewares[:i], q.middlewares[i+1:]...)
			return nil
		}
	}

	return errors.Errorf("middleware '%s' not found", name)
}

// GetMetrics returns protocol metrics
func (q *QUIC) GetMetrics() protocol.Metrics {
	q.metrics.mu.RLock()
	defer q.metrics.mu.RUnlock()

	return protocol.Metrics{
		ActiveConnections:    atomic.LoadUint64(&q.metrics.ActiveConnections),
		TotalConnections:     atomic.LoadInt64(&q.metrics.TotalConnections),
		FailedConnections:    atomic.LoadInt64(&q.metrics.FailedConnections),
		MessagesSent:         atomic.LoadUint64(&q.metrics.MessagesSent),
		MessagesReceived:     atomic.LoadUint64(&q.metrics.MessagesReceived),
		MessagesPerSecond:    q.metrics.MessagesPerSecond,
		ConnectionsPerSecond: 0, // TODO: Implement
		AverageMessageSize:   0, // TODO: Implement
	}
}

// GetClientMetrics returns metrics for a specific client
func (q *QUIC) GetClientMetrics(clientID string) (protocol.ClientMetrics, bool) {
	q.clientsMu.RLock()
	client, exists := q.clients[clientID]
	q.clientsMu.RUnlock()

	if !exists {
		return protocol.ClientMetrics{}, false
	}

	return client.GetMetrics(), true
}

// GetConfig returns the protocol configuration
func (q *QUIC) GetConfig() protocol.Config {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.config
}

// UpdateConfig updates the protocol configuration
func (q *QUIC) UpdateConfig(config protocol.Config) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	// In a production system, you'd want to validate the config
	// and potentially restart certain components
	q.config = config
	return nil
}
