package protocol

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
	"github.com/zeusync/zeusync/internal/core/protocol/intrefaces"
	"math/big"
	"net"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var _ intrefaces.Protocol = (*QuicProtocol)(nil)

// QuicProtocol implements the Protocol interface for QUIC
type QuicProtocol struct {
	name     string
	version  string
	config   intrefaces.ProtocolConfig
	listener *quic.Listener
	running  int32
	mu       sync.RWMutex

	// Message handling
	handlers       map[string]intrefaces.MessageHandler
	defaultHandler intrefaces.MessageHandler
	middlewares    []intrefaces.ProtocolMiddleware

	// Client management
	clients   map[string]*QuicConnection
	clientsMu sync.RWMutex
	groups    map[string]map[string]struct{}
	groupsMu  sync.RWMutex

	// Metrics
	metrics *QuicMetrics
	logger  *logrus.Entry

	// Worker pools
	messageWorkers chan *quicMessageWork
	workerWg       sync.WaitGroup
	ctx            context.Context
	cancel         context.CancelFunc
}

// QuicMetrics extends the interface metrics with QUIC-specific data
type QuicMetrics struct {
	intrefaces.ProtocolMetrics
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
	client  *QuicConnection
	message intrefaces.Message
	ctx     context.Context
}

// NewQuicProtocol creates a new QUIC protocol instance
func NewQuicProtocol(config intrefaces.ProtocolConfig, logger *logrus.Logger) *QuicProtocol {
	ctx, cancel := context.WithCancel(context.Background())

	return &QuicProtocol{
		name:           "QUIC",
		version:        "1.0.0",
		config:         config,
		handlers:       make(map[string]intrefaces.MessageHandler),
		clients:        make(map[string]*QuicConnection),
		groups:         make(map[string]map[string]struct{}),
		metrics:        &QuicMetrics{},
		logger:         logger.WithField("protocol", "quic"),
		messageWorkers: make(chan *quicMessageWork, config.QueueSize),
		ctx:            ctx,
		cancel:         cancel,
	}
}

// Name returns the protocol name
func (p *QuicProtocol) Name() string {
	return p.name
}

// Version returns the protocol version
func (p *QuicProtocol) Version() string {
	return p.version
}

// Type returns the protocol type
func (p *QuicProtocol) Type() intrefaces.ProtocolType {
	return intrefaces.ProtocolCustom
}

// Start starts the QUIC protocol listener
func (p *QuicProtocol) Start(ctx context.Context, config intrefaces.ProtocolConfig) error {
	if !atomic.CompareAndSwapInt32(&p.running, 0, 1) {
		return errors.New("protocol is already running")
	}

	p.mu.Lock()
	p.config = config
	p.mu.Unlock()

	// Start worker pool
	p.startWorkers()

	tlsConfig, err := p.createTLSConfig()
	if err != nil {
		return errors.Wrap(err, "failed to create TLS config")
	}

	// Configure QUIC
	quicConfig := &quic.Config{
		MaxIdleTimeout:        p.config.KeepAliveTimeout,
		MaxIncomingStreams:    int64(p.config.MaxConnections),
		MaxIncomingUniStreams: int64(p.config.MaxConnections / 2),
	}

	addr := net.JoinHostPort(p.config.Host, fmt.Sprintf("%d", p.config.Port))
	listener, err := quic.ListenAddr(addr, tlsConfig, quicConfig)
	if err != nil {
		return errors.Wrap(err, "failed to start QUIC listener")
	}

	p.listener = listener
	p.logger.Infof("QUIC protocol started on %s", addr)

	// Start accepting connections
	go p.acceptConnections(ctx)

	// Start metrics collection
	go p.collectMetrics()

	return nil
}

// Stop stops the QUIC protocol listener
func (p *QuicProtocol) Stop(ctx context.Context) error {
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
	p.clients = make(map[string]*QuicConnection)
	p.clientsMu.Unlock()

	// Close listener
	if p.listener != nil {
		if err := p.listener.Close(); err != nil {
			return errors.Wrap(err, "failed to close QUIC listener")
		}
	}

	// Wait for workers to finish
	p.workerWg.Wait()

	p.logger.Info("QUIC protocol stopped")
	return nil
}

// Restart restarts the protocol
func (p *QuicProtocol) Restart(ctx context.Context) error {
	if err := p.Stop(ctx); err != nil {
		return err
	}
	return p.Start(ctx, p.config)
}

// IsRunning returns true if the protocol is running
func (p *QuicProtocol) IsRunning() bool {
	return atomic.LoadInt32(&p.running) == 1
}

// startWorkers starts the message processing worker pool
func (p *QuicProtocol) startWorkers() {
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
func (p *QuicProtocol) messageWorker() {
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
func (p *QuicProtocol) processMessage(work *quicMessageWork) {
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

func (p *QuicProtocol) createTLSConfig() (*tls.Config, error) {
	if !p.config.TLSEnabled {
		// Generate self-signed certificate for development
		return p.generateTLSConfig(), nil
	}

	cert, err := tls.LoadX509KeyPair(p.config.CertFile, p.config.KeyFile)
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
func (p *QuicProtocol) generateTLSConfig() *tls.Config {
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

func (p *QuicProtocol) acceptConnections(ctx context.Context) {
	for {
		sess, err := p.listener.Accept(ctx)
		if err != nil {
			if p.IsRunning() {
				p.logger.WithError(err).Error("failed to accept connection")
				atomic.AddInt64(&p.metrics.handshakeErrors, 1)
			}
			return
		}

		go p.handleConnection(ctx, sess)
	}
}

func (p *QuicProtocol) handleConnection(ctx context.Context, sess *quic.Conn) {
	client := NewQuicConnection(sess, p.config)

	// Add to clients map
	p.clientsMu.Lock()
	p.clients[client.ID()] = client
	p.clientsMu.Unlock()

	// Update metrics
	atomic.AddUint64(&p.metrics.ActiveConnections, 1)
	atomic.AddInt64(&p.metrics.TotalConnections, 1)

	// Apply connection middleware
	for _, mw := range p.middlewares {
		if err := mw.OnConnect(ctx, client.ClientInfo()); err != nil {
			p.logger.WithError(err).Error("Middleware OnConnect error")
		}
	}

	p.logger.WithField("client_id", client.ID()).Info("QUIC client connected")

	// Handle client in separate goroutine
	go p.handleClient(ctx, client)
}

func (p *QuicProtocol) handleClient(ctx context.Context, client *QuicConnection) {
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
			if err := mw.OnDisconnect(ctx, client.ClientInfo(), "connection closed"); err != nil {
				p.logger.WithError(err).Error("Middleware OnDisconnect error")
			}
		}

		// Update metrics
		atomic.AddUint64(&p.metrics.ActiveConnections, ^uint64(0)) // Decrement

		_ = client.Close()
		p.logger.WithField("client_id", client.ID()).Info("QUIC client disconnected")
	}()

	// Read messages from client
	for {
		message, err := client.ReceiveMessage()
		if err != nil {
			p.logger.WithError(err).Debug("Failed to receive message from client")
			return
		}

		// Queue message for processing
		select {
		case p.messageWorkers <- &quicMessageWork{
			client:  client,
			message: message,
			ctx:     ctx,
		}:
		default:
			// Queue is full
			atomic.AddInt64(&p.metrics.messageQueueOverflow, 1)
			p.logger.Warn("Message queue overflow, dropping message")
		}
	}
}

// collectMetrics periodically collects and updates metrics
func (p *QuicProtocol) collectMetrics() {
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
func (p *QuicProtocol) RegisterHandler(messageType string, handler intrefaces.MessageHandler) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.handlers[messageType]; exists {
		return errors.Errorf("handler for message type '%s' already registered", messageType)
	}

	p.handlers[messageType] = handler
	return nil
}

// UnregisterHandler unregisters a message handler
func (p *QuicProtocol) UnregisterHandler(messageType string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.handlers[messageType]; !exists {
		return errors.Errorf("no handler registered for message type '%s'", messageType)
	}

	delete(p.handlers, messageType)
	return nil
}

// GetHandler returns the handler for a specific message type
func (p *QuicProtocol) GetHandler(messageType string) (intrefaces.MessageHandler, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	handler, exists := p.handlers[messageType]
	return handler, exists
}

// SetDefaultHandler sets the default message handler
func (p *QuicProtocol) SetDefaultHandler(handler intrefaces.MessageHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.defaultHandler = handler
}

// Send sends a message to a specific client
func (p *QuicProtocol) Send(clientID string, message intrefaces.Message) error {
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
func (p *QuicProtocol) SendToMultiple(clientIDs []string, message intrefaces.Message) error {
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
func (p *QuicProtocol) Broadcast(message intrefaces.Message) error {
	p.clientsMu.RLock()
	clientIDs := make([]string, 0, len(p.clients))
	for id := range p.clients {
		clientIDs = append(clientIDs, id)
	}
	p.clientsMu.RUnlock()

	return p.SendToMultiple(clientIDs, message)
}

// BroadcastExcept sends a message to all clients except the specified ones
func (p *QuicProtocol) BroadcastExcept(excludeClientIDs []string, message intrefaces.Message) error {
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
func (p *QuicProtocol) GetClient(clientID string) (intrefaces.ClientInfo, bool) {
	p.clientsMu.RLock()
	client, exists := p.clients[clientID]
	p.clientsMu.RUnlock()

	if !exists {
		return intrefaces.ClientInfo{}, false
	}

	return client.ClientInfo(), true
}

// GetAllClients returns a list of all connected clients
func (p *QuicProtocol) GetAllClients() []intrefaces.ClientInfo {
	p.clientsMu.RLock()
	defer p.clientsMu.RUnlock()

	clients := make([]intrefaces.ClientInfo, 0, len(p.clients))
	for _, client := range p.clients {
		clients = append(clients, client.ClientInfo())
	}

	return clients
}

// DisconnectClient disconnects a client
func (p *QuicProtocol) DisconnectClient(clientID string, reason string) error {
	p.clientsMu.RLock()
	client, exists := p.clients[clientID]
	p.clientsMu.RUnlock()

	if !exists {
		return errors.Errorf("client '%s' not found", clientID)
	}

	return client.CloseWithReason(reason)
}

// GetConnectionCount returns the number of connected clients
func (p *QuicProtocol) GetConnectionCount() int {
	p.clientsMu.RLock()
	defer p.clientsMu.RUnlock()
	return len(p.clients)
}

// CreateGroup creates a new client group
func (p *QuicProtocol) CreateGroup(groupID string) error {
	p.groupsMu.Lock()
	defer p.groupsMu.Unlock()

	if _, exists := p.groups[groupID]; exists {
		return errors.Errorf("group '%s' already exists", groupID)
	}

	p.groups[groupID] = make(map[string]struct{})
	return nil
}

// DeleteGroup deletes a client group
func (p *QuicProtocol) DeleteGroup(groupID string) error {
	p.groupsMu.Lock()
	defer p.groupsMu.Unlock()

	if _, exists := p.groups[groupID]; !exists {
		return errors.Errorf("group '%s' not found", groupID)
	}

	delete(p.groups, groupID)
	return nil
}

// JoinGroup adds a client to a group
func (p *QuicProtocol) JoinGroup(clientID, groupID string) error {
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
func (p *QuicProtocol) LeaveGroup(clientID, groupID string) error {
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
func (p *QuicProtocol) SendToGroup(groupID string, message intrefaces.Message) error {
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
func (p *QuicProtocol) GetGroupMembers(groupID string) ([]string, error) {
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
func (p *QuicProtocol) AddMiddleware(middleware intrefaces.ProtocolMiddleware) error {
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
func (p *QuicProtocol) RemoveMiddleware(name string) error {
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
func (p *QuicProtocol) GetMetrics() intrefaces.ProtocolMetrics {
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
func (p *QuicProtocol) GetClientMetrics(clientID string) (intrefaces.ClientMetrics, bool) {
	p.clientsMu.RLock()
	client, exists := p.clients[clientID]
	p.clientsMu.RUnlock()

	if !exists {
		return intrefaces.ClientMetrics{}, false
	}

	return client.GetMetrics(), true
}

// GetConfig returns the protocol configuration
func (p *QuicProtocol) GetConfig() intrefaces.ProtocolConfig {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.config
}

// UpdateConfig updates the protocol configuration
func (p *QuicProtocol) UpdateConfig(config intrefaces.ProtocolConfig) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// In a production system, you'd want to validate the config
	// and potentially restart certain components
	p.config = config
	return nil
}
