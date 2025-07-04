package protocol

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Server - основная реализация Protocol
type Server struct {
	config    Config
	transport Transport
	running   int32

	// Handlers
	connectHandler    ClientConnectHandler
	disconnectHandler ClientDisconnectHandler
	messageHandler    MessageHandler

	// Client management
	clients   map[string]*ClientSession
	clientsMu sync.RWMutex

	// Group management
	groups   map[string]*Group
	groupsMu sync.RWMutex

	// Middleware
	middlewares  []Middleware
	middlewareMu sync.RWMutex

	// Message processing
	messageQueue chan *MessageWork
	workers      []*Worker
	workerWg     sync.WaitGroup

	// Metrics
	metrics *ServerMetrics

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
}

// ClientSession представляет сессию клиента
type ClientSession struct {
	conn     Connection
	info     ClientInfo
	groups   map[string]bool
	groupsMu sync.RWMutex
	lastPing time.Time
	pingMu   sync.RWMutex
}

// Group представляет группу клиентов
type Group struct {
	id      string
	clients map[string]*ClientSession
	mu      sync.RWMutex
	created time.Time
	maxSize int
}

// MessageWork представляет работу по обработке сообщения
type MessageWork struct {
	client  *ClientSession
	message Message
	ctx     context.Context
}

// Worker обрабатывает сообщения
type Worker struct {
	id     int
	server *Server
	queue  chan *MessageWork
	ctx    context.Context
}

// ServerMetrics содержит метрики сервера
type ServerMetrics struct {
	mu sync.RWMutex
	Metrics
	startTime time.Time
}

// NewServer создает новый сервер
func NewServer(config Config, transport Transport) Protocol {
	ctx, cancel := context.WithCancel(context.Background())

	return &Server{
		config:       config,
		transport:    transport,
		clients:      make(map[string]*ClientSession),
		groups:       make(map[string]*Group),
		middlewares:  make([]Middleware, 0),
		messageQueue: make(chan *MessageWork, config.QueueSize),
		metrics: &ServerMetrics{
			startTime: time.Now(),
		},
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start запускает сервер
func (s *Server) Start(ctx context.Context, config Config) error {
	if !atomic.CompareAndSwapInt32(&s.running, 0, 1) {
		return fmt.Errorf("server already running")
	}

	s.config = config

	// Запускаем транспорт
	address := fmt.Sprintf("%s:%d", config.Host, config.Port)
	if err := s.transport.Listen(address); err != nil {
		atomic.StoreInt32(&s.running, 0)
		return fmt.Errorf("failed to start transport: %w", err)
	}

	// Запускаем воркеров
	s.startWorkers()

	// Запускаем обработку соединений
	go s.acceptConnections()

	// Запускаем heartbeat если включен
	if config.EnableHeartbeat {
		go s.heartbeatLoop()
	}

	// Запускаем сборщик метрик
	if config.EnableMetrics {
		go s.metricsLoop()
	}

	return nil
}

// Stop останавливает сервер
func (s *Server) Stop(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&s.running, 1, 0) {
		return fmt.Errorf("server not running")
	}

	// Отменяем контекст
	s.cancel()

	// Закрываем все соединения
	s.clientsMu.Lock()
	for _, client := range s.clients {
		_ = client.conn.Close()
	}
	s.clients = make(map[string]*ClientSession)
	s.clientsMu.Unlock()

	// Ждем завершения воркеров
	close(s.messageQueue)
	s.workerWg.Wait()

	// Закрываем транспорт
	return s.transport.Close()
}

// IsRunning проверяет, запущен ли сервер
func (s *Server) IsRunning() bool {
	return atomic.LoadInt32(&s.running) == 1
}

// OnClientConnect устанавливает обработчик подключения клиента
func (s *Server) OnClientConnect(handler ClientConnectHandler) {
	s.connectHandler = handler
}

// OnClientDisconnect устанавливает обработчик отключения клиента
func (s *Server) OnClientDisconnect(handler ClientDisconnectHandler) {
	s.disconnectHandler = handler
}

// OnMessage устанавливает обработчик сообщений
func (s *Server) OnMessage(handler MessageHandler) {
	s.messageHandler = handler
}

// SendToClient отправляет сообщение клиенту
func (s *Server) SendToClient(clientID string, message Message) error {
	s.clientsMu.RLock()
	client, exists := s.clients[clientID]
	s.clientsMu.RUnlock()

	if !exists {
		return &ProtocolError{
			Code:    ErrorCodeClientNotFound,
			Message: fmt.Sprintf("client %s not found", clientID),
		}
	}

	if err := client.conn.SendMessage(message); err != nil {
		return fmt.Errorf("failed to send message to client %s: %w", clientID, err)
	}

	atomic.AddInt64(&s.metrics.MessagesSent, 1)
	return nil
}

// SendToClients отправляет сообщение нескольким клиентам
func (s *Server) SendToClients(clientIDs []string, message Message) error {
	var errors []error

	for _, clientID := range clientIDs {
		if err := s.SendToClient(clientID, message); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to send to %d clients", len(errors))
	}

	return nil
}

// Broadcast отправляет сообщение всем клиентам
func (s *Server) Broadcast(message Message) error {
	s.clientsMu.RLock()
	clientIDs := make([]string, 0, len(s.clients))
	for id := range s.clients {
		clientIDs = append(clientIDs, id)
	}
	s.clientsMu.RUnlock()

	return s.SendToClients(clientIDs, message)
}

// BroadcastExcept отправляет сообщение всем клиентам кроме исключенных
func (s *Server) BroadcastExcept(excludeClientIDs []string, message Message) error {
	excludeMap := make(map[string]bool, len(excludeClientIDs))
	for _, id := range excludeClientIDs {
		excludeMap[id] = true
	}

	s.clientsMu.RLock()
	var clientIDs []string
	for id := range s.clients {
		if !excludeMap[id] {
			clientIDs = append(clientIDs, id)
		}
	}
	s.clientsMu.RUnlock()

	return s.SendToClients(clientIDs, message)
}

// CreateGroup создает новую группу
func (s *Server) CreateGroup(groupID string) error {
	s.groupsMu.Lock()
	defer s.groupsMu.Unlock()

	if _, exists := s.groups[groupID]; exists {
		return &ProtocolError{
			Code:    "GROUP_EXISTS",
			Message: fmt.Sprintf("group %s already exists", groupID),
		}
	}

	s.groups[groupID] = &Group{
		id:      groupID,
		clients: make(map[string]*ClientSession),
		created: time.Now(),
		maxSize: s.config.MaxGroupSize,
	}

	atomic.AddInt64(&s.metrics.TotalGroups, 1)
	atomic.AddInt64(&s.metrics.ActiveGroups, 1)

	return nil
}

// DeleteGroup удаляет группу
func (s *Server) DeleteGroup(groupID string) error {
	s.groupsMu.Lock()
	defer s.groupsMu.Unlock()

	group, exists := s.groups[groupID]
	if !exists {
		return &ProtocolError{
			Code:    ErrorCodeGroupNotFound,
			Message: fmt.Sprintf("group %s not found", groupID),
		}
	}

	// Удаляем клиентов из группы
	group.mu.Lock()
	for _, client := range group.clients {
		client.groupsMu.Lock()
		delete(client.groups, groupID)
		client.groupsMu.Unlock()
	}
	group.mu.Unlock()

	delete(s.groups, groupID)
	atomic.AddInt64(&s.metrics.ActiveGroups, -1)

	return nil
}

// AddClientToGroup добавляет клиента в группу
func (s *Server) AddClientToGroup(clientID, groupID string) error {
	s.clientsMu.RLock()
	client, clientExists := s.clients[clientID]
	s.clientsMu.RUnlock()

	if !clientExists {
		return &ProtocolError{
			Code:    ErrorCodeClientNotFound,
			Message: fmt.Sprintf("client %s not found", clientID),
		}
	}

	s.groupsMu.RLock()
	group, groupExists := s.groups[groupID]
	s.groupsMu.RUnlock()

	if !groupExists {
		return &ProtocolError{
			Code:    ErrorCodeGroupNotFound,
			Message: fmt.Sprintf("group %s not found", groupID),
		}
	}

	group.mu.Lock()
	defer group.mu.Unlock()

	if len(group.clients) >= group.maxSize {
		return &ProtocolError{
			Code:    "GROUP_FULL",
			Message: fmt.Sprintf("group %s is full", groupID),
		}
	}

	group.clients[clientID] = client

	client.groupsMu.Lock()
	client.groups[groupID] = true
	client.groupsMu.Unlock()

	return nil
}

// RemoveClientFromGroup удаляет клиента из группы
func (s *Server) RemoveClientFromGroup(clientID, groupID string) error {
	s.groupsMu.RLock()
	group, exists := s.groups[groupID]
	s.groupsMu.RUnlock()

	if !exists {
		return &ProtocolError{
			Code:    ErrorCodeGroupNotFound,
			Message: fmt.Sprintf("group %s not found", groupID),
		}
	}

	group.mu.Lock()
	client, clientInGroup := group.clients[clientID]
	if clientInGroup {
		delete(group.clients, clientID)
	}
	group.mu.Unlock()

	if clientInGroup {
		client.groupsMu.Lock()
		delete(client.groups, groupID)
		client.groupsMu.Unlock()
	}

	return nil
}

// SendToGroup отправляет сообщение всем клиентам в группе
func (s *Server) SendToGroup(groupID string, message Message) error {
	s.groupsMu.RLock()
	group, exists := s.groups[groupID]
	s.groupsMu.RUnlock()

	if !exists {
		return &ProtocolError{
			Code:    ErrorCodeGroupNotFound,
			Message: fmt.Sprintf("group %s not found", groupID),
		}
	}

	group.mu.RLock()
	clientIDs := make([]string, 0, len(group.clients))
	for id := range group.clients {
		clientIDs = append(clientIDs, id)
	}
	group.mu.RUnlock()

	return s.SendToClients(clientIDs, message)
}

// GetGroupClients возвращает список клиентов в группе
func (s *Server) GetGroupClients(groupID string) []string {
	s.groupsMu.RLock()
	group, exists := s.groups[groupID]
	s.groupsMu.RUnlock()

	if !exists {
		return nil
	}

	group.mu.RLock()
	defer group.mu.RUnlock()

	clientIDs := make([]string, 0, len(group.clients))
	for id := range group.clients {
		clientIDs = append(clientIDs, id)
	}

	return clientIDs
}

// GetMetrics возвращает метрики сервера
func (s *Server) GetMetrics() Metrics {
	s.metrics.mu.RLock()
	defer s.metrics.mu.RUnlock()

	return s.metrics.Metrics
}

// GetClients возвращает информацию о всех клиентах
func (s *Server) GetClients() []ClientInfo {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()

	clients := make([]ClientInfo, 0, len(s.clients))
	for _, client := range s.clients {
		clients = append(clients, client.info)
	}

	return clients
}

// GetConfig возвращает конфигурацию сервера
func (s *Server) GetConfig() Config {
	return s.config
}

// acceptConnections принимает новые соединения
func (s *Server) acceptConnections() {
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			conn, err := s.transport.Accept()
			if err != nil {
				if s.IsRunning() {
					// Логируем ошибку
					atomic.AddInt64(&s.metrics.FailedConnections, 1)
				}
				continue
			}

			go s.handleConnection(conn)
		}
	}
}

// handleConnection обрабатывает новое соединение
func (s *Server) handleConnection(conn Connection) {
	client := &ClientSession{
		conn: conn,
		info: ClientInfo{
			ID:            conn.ID(),
			RemoteAddress: conn.RemoteAddr().String(),
			ConnectedAt:   time.Now(),
			LastActivity:  time.Now(),
			Transport:     s.transport.Type(),
			Groups:        make([]string, 0),
			Metadata:      make(map[string]interface{}),
		},
		groups:   make(map[string]bool),
		lastPing: time.Now(),
	}

	// Добавляем клиента
	s.clientsMu.Lock()
	s.clients[client.info.ID] = client
	s.clientsMu.Unlock()

	atomic.AddInt64(&s.metrics.ActiveConnections, 1)
	atomic.AddInt64(&s.metrics.TotalConnections, 1)

	// Вызываем обработчик подключения
	if s.connectHandler != nil {
		if err := s.connectHandler(s.ctx, client.info); err != nil {
			_ = conn.Close()
			s.removeClient(client.info.ID)
			return
		}
	}

	// Обрабатываем сообщения от клиента
	s.handleClientMessages(client)
}

// handleClientMessages обрабатывает сообщения от клиента
func (s *Server) handleClientMessages(client *ClientSession) {
	defer func() {
		s.removeClient(client.info.ID)
		_ = client.conn.Close()

		// Вызываем обработчик отключения
		if s.disconnectHandler != nil {
			_ = s.disconnectHandler(s.ctx, client.info, "connection closed")
		}
	}()

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			message, err := client.conn.ReceiveMessage()
			if err != nil {
				return
			}

			// Обновляем время последней активности
			client.info.LastActivity = time.Now()

			// Добавляем в очередь обработки
			select {
			case s.messageQueue <- &MessageWork{
				client:  client,
				message: message,
				ctx:     s.ctx,
			}:
				atomic.AddInt64(&s.metrics.MessagesQueued, 1)
			default:
				// Очередь переполнена
				atomic.AddInt64(&s.metrics.MessagesDropped, 1)
			}
		}
	}
}

// removeClient удаляет клиента
func (s *Server) removeClient(clientID string) {
	s.clientsMu.Lock()
	client, exists := s.clients[clientID]
	if exists {
		delete(s.clients, clientID)
	}
	s.clientsMu.Unlock()

	if !exists {
		return
	}

	// Удаляем из всех групп
	client.groupsMu.RLock()
	groupIDs := make([]string, 0, len(client.groups))
	for groupID := range client.groups {
		groupIDs = append(groupIDs, groupID)
	}
	client.groupsMu.RUnlock()

	for _, groupID := range groupIDs {
		_ = s.RemoveClientFromGroup(clientID, groupID)
	}

	atomic.AddInt64(&s.metrics.ActiveConnections, -1)
}

// startWorkers запускает воркеров для обработки сообщений
func (s *Server) startWorkers() {
	workerCount := s.config.WorkerCount
	if workerCount <= 0 {
		workerCount = 4
	}

	s.workers = make([]*Worker, workerCount)
	for i := 0; i < workerCount; i++ {
		worker := &Worker{
			id:     i,
			server: s,
			queue:  s.messageQueue,
			ctx:    s.ctx,
		}
		s.workers[i] = worker

		s.workerWg.Add(1)
		go worker.run()
	}
}

// run запускает воркер
func (w *Worker) run() {
	defer w.server.workerWg.Done()

	for {
		select {
		case work := <-w.queue:
			if work == nil {
				return
			}
			w.processMessage(work)
		case <-w.ctx.Done():
			return
		}
	}
}

// processMessage обрабатывает сообщение
func (w *Worker) processMessage(work *MessageWork) {
	defer func() {
		if r := recover(); r != nil {
			// Логируем панику
			atomic.AddInt64(&w.server.metrics.MessagesDropped, 1)
		}
	}()

	// Применяем middleware
	err := w.applyMiddleware(work)
	if err != nil {
		atomic.AddInt64(&w.server.metrics.MessagesDropped, 1)
		return
	}

	// Вызываем основной обработчик
	if w.server.messageHandler != nil {
		err = w.server.messageHandler(work.ctx, work.client.info, work.message)
		if err != nil {
			// Логируем ошибку
		}
	}

	atomic.AddInt64(&w.server.metrics.MessagesReceived, 1)
	atomic.AddInt64(&w.server.metrics.MessagesQueued, -1)
}

// applyMiddleware применяет middleware
func (w *Worker) applyMiddleware(work *MessageWork) error {
	w.server.middlewareMu.RLock()
	middlewares := make([]Middleware, len(w.server.middlewares))
	copy(middlewares, w.server.middlewares)
	w.server.middlewareMu.RUnlock()

	for _, middleware := range middlewares {
		err := middleware.OnMessage(work.ctx, work.client.info, work.message, func() error {
			return nil
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// heartbeatLoop отправляет heartbeat сообщения
func (s *Server) heartbeatLoop() {
	ticker := time.NewTicker(s.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.sendHeartbeats()
		case <-s.ctx.Done():
			return
		}
	}
}

// sendHeartbeats отправляет heartbeat всем клиентам
func (s *Server) sendHeartbeats() {
	heartbeat := NewMessage(MessageTypeHeartbeat, []byte("ping"))

	s.clientsMu.RLock()
	for _, client := range s.clients {
		_ = client.conn.SendMessage(heartbeat)
	}
	s.clientsMu.RUnlock()
}

// metricsLoop собирает метрики
func (s *Server) metricsLoop() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var lastMessagesSent, lastMessagesReceived int64
	var lastTime = time.Now()

	for {
		select {
		case <-ticker.C:
			s.updateMetrics(&lastMessagesSent, &lastMessagesReceived, &lastTime)
		case <-s.ctx.Done():
			return
		}
	}
}

// updateMetrics обновляет метрики
func (s *Server) updateMetrics(lastSent, lastReceived *int64, lastTime *time.Time) {
	now := time.Now()
	duration := now.Sub(*lastTime).Seconds()

	currentSent := atomic.LoadInt64(&s.metrics.MessagesSent)
	currentReceived := atomic.LoadInt64(&s.metrics.MessagesReceived)

	if duration > 0 {
		s.metrics.mu.Lock()
		s.metrics.MessagesPerSecond = float64(currentSent+currentReceived-*lastSent-*lastReceived) / duration
		s.metrics.mu.Unlock()
	}

	*lastSent = currentSent
	*lastReceived = currentReceived
	*lastTime = now
}
