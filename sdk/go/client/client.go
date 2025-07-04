package client

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zeusync/zeusync/internal/core/protocol"
)

// Client - SDK клиент для подключения к ZeuSync серверу
type Client struct {
	conn      protocol.Connection
	transport protocol.Transport
	config    ClientConfig

	// Message handling
	messageHandlers map[string]MessageHandler
	handlersMu      sync.RWMutex
	defaultHandler  MessageHandler

	// State
	connected int32
	ctx       context.Context
	cancel    context.CancelFunc

	// Metrics
	metrics ClientMetrics

	// Events
	onConnect    func()
	onDisconnect func(reason string)
	onError      func(error)
}

// ClientConfig - конфигурация клиента
type ClientConfig struct {
	// Connection
	ServerAddress string
	Transport     protocol.TransportType

	// Timeouts
	ConnectTimeout time.Duration
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration

	// Reconnection
	EnableReconnect   bool
	ReconnectInterval time.Duration
	MaxReconnectTries int

	// Features
	EnableHeartbeat   bool
	HeartbeatInterval time.Duration

	// Custom options
	Metadata map[string]interface{}
}

// MessageHandler - обработчик сообщений
type MessageHandler func(ctx context.Context, message protocol.Message) error

// ClientMetrics - метрики клиента
type ClientMetrics struct {
	ConnectedAt      time.Time
	LastActivity     time.Time
	MessagesSent     int64
	MessagesReceived int64
	BytesSent        int64
	BytesReceived    int64
	ReconnectCount   int64
	ErrorCount       int64
}

// NewClient создает новый клиент
func NewClient(config ClientConfig) *Client {
	ctx, cancel := context.WithCancel(context.Background())

	var transport protocol.Transport
	switch config.Transport {
	case protocol.TransportQUIC:
		transport = protocol.NewQUICTransport()
	default:
		transport = protocol.NewQUICTransport() // По умолчанию QUIC
	}

	return &Client{
		transport:       transport,
		config:          config,
		messageHandlers: make(map[string]MessageHandler),
		ctx:             ctx,
		cancel:          cancel,
	}
}

// Connect подключается к с��рверу
func (c *Client) Connect() error {
	if !atomic.CompareAndSwapInt32(&c.connected, 0, 1) {
		return fmt.Errorf("client already connected")
	}

	conn, err := c.transport.Dial(c.config.ServerAddress)
	if err != nil {
		atomic.StoreInt32(&c.connected, 0)
		return fmt.Errorf("failed to connect to %s: %w", c.config.ServerAddress, err)
	}

	c.conn = conn
	c.metrics.ConnectedAt = time.Now()
	c.metrics.LastActivity = time.Now()

	// Настраиваем таймауты
	if c.config.ReadTimeout > 0 {
		conn.SetReadTimeout(c.config.ReadTimeout)
	}
	if c.config.WriteTimeout > 0 {
		conn.SetWriteTimeout(c.config.WriteTimeout)
	}

	// Запускаем обработку сообщений
	go c.messageLoop()

	// Запускаем heartbeat если включен
	if c.config.EnableHeartbeat {
		go c.heartbeatLoop()
	}

	// Вызываем callback
	if c.onConnect != nil {
		c.onConnect()
	}

	return nil
}

// Disconnect отключается от сервера
func (c *Client) Disconnect() error {
	if !atomic.CompareAndSwapInt32(&c.connected, 1, 0) {
		return fmt.Errorf("client not connected")
	}

	c.cancel()

	if c.conn != nil {
		return c.conn.Close()
	}

	return nil
}

// IsConnected проверяет, подключен ли клиент
func (c *Client) IsConnected() bool {
	return atomic.LoadInt32(&c.connected) == 1 && c.conn != nil && c.conn.IsAlive()
}

// Send отправляет сообщение серверу
func (c *Client) Send(msgType string, payload []byte) error {
	if !c.IsConnected() {
		return fmt.Errorf("client not connected")
	}

	message := protocol.NewMessage(msgType, payload)

	if err := c.conn.SendMessage(message); err != nil {
		atomic.AddInt64(&c.metrics.ErrorCount, 1)
		return fmt.Errorf("failed to send message: %w", err)
	}

	atomic.AddInt64(&c.metrics.MessagesSent, 1)
	c.metrics.LastActivity = time.Now()

	return nil
}

// SendMessage отправляет готовое сообщение
func (c *Client) SendMessage(message protocol.Message) error {
	if !c.IsConnected() {
		return fmt.Errorf("client not connected")
	}

	if err := c.conn.SendMessage(message); err != nil {
		atomic.AddInt64(&c.metrics.ErrorCount, 1)
		return fmt.Errorf("failed to send message: %w", err)
	}

	atomic.AddInt64(&c.metrics.MessagesSent, 1)
	c.metrics.LastActivity = time.Now()

	return nil
}

// OnMessage устанавливает обработчик для определенного типа сообщений
func (c *Client) OnMessage(msgType string, handler MessageHandler) {
	c.handlersMu.Lock()
	defer c.handlersMu.Unlock()
	c.messageHandlers[msgType] = handler
}

// OnAnyMessage устанавливает обработчик по умолчанию
func (c *Client) OnAnyMessage(handler MessageHandler) {
	c.defaultHandler = handler
}

// OnConnect устанавливает callback подключения
func (c *Client) OnConnect(callback func()) {
	c.onConnect = callback
}

// OnDisconnect устанавливает callback отключения
func (c *Client) OnDisconnect(callback func(reason string)) {
	c.onDisconnect = callback
}

// OnError устанавливает callback ошибок
func (c *Client) OnError(callback func(error)) {
	c.onError = callback
}

// GetMetrics возвращает метрики клиента
func (c *Client) GetMetrics() ClientMetrics {
	return c.metrics
}

// SetMetadata устанавливает метаданные
func (c *Client) SetMetadata(key string, value interface{}) {
	if c.conn != nil {
		c.conn.SetMetadata(key, value)
	}
}

// GetMetadata получает метаданные
func (c *Client) GetMetadata(key string) (interface{}, bool) {
	if c.conn != nil {
		return c.conn.GetMetadata(key)
	}
	return nil, false
}

// messageLoop обрабатывает входящие сообщения
func (c *Client) messageLoop() {
	defer func() {
		if c.onDisconnect != nil {
			c.onDisconnect("message loop ended")
		}
	}()

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			if !c.IsConnected() {
				return
			}

			message, err := c.conn.ReceiveMessage()
			if err != nil {
				if c.onError != nil {
					c.onError(err)
				}
				atomic.AddInt64(&c.metrics.ErrorCount, 1)
				return
			}

			atomic.AddInt64(&c.metrics.MessagesReceived, 1)
			c.metrics.LastActivity = time.Now()

			// Обрабатываем сообщение
			go c.handleMessage(message)
		}
	}
}

// handleMessage обрабатывает полученное сообщение
func (c *Client) handleMessage(message protocol.Message) {
	defer func() {
		if r := recover(); r != nil {
			if c.onError != nil {
				c.onError(fmt.Errorf("panic in message handler: %v", r))
			}
		}
	}()

	msgType := message.Type()

	// Ищем специфичный обработчик
	c.handlersMu.RLock()
	handler, exists := c.messageHandlers[msgType]
	c.handlersMu.RUnlock()

	if exists {
		if err := handler(c.ctx, message); err != nil && c.onError != nil {
			c.onError(fmt.Errorf("handler error for %s: %w", msgType, err))
		}
		return
	}

	// Используем обработчик по умолчанию
	if c.defaultHandler != nil {
		if err := c.defaultHandler(c.ctx, message); err != nil && c.onError != nil {
			c.onError(fmt.Errorf("default handler error: %w", err))
		}
	}
}

// heartbeatLoop отправляет heartbeat сообщения
func (c *Client) heartbeatLoop() {
	ticker := time.NewTicker(c.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if c.IsConnected() {
				_ = c.Send(protocol.MessageTypeHeartbeat, []byte("ping"))
			}
		case <-c.ctx.Done():
			return
		}
	}
}

// Convenience methods для частых операций

// JoinGroup присоединяется к группе
func (c *Client) JoinGroup(groupID string) error {
	message := protocol.NewMessage(protocol.MessageTypeJoinGroup, []byte(groupID))
	message.SetHeader("group_id", groupID)
	return c.SendMessage(message)
}

// LeaveGroup покидает группу
func (c *Client) LeaveGroup(groupID string) error {
	message := protocol.NewMessage(protocol.MessageTypeLeaveGroup, []byte(groupID))
	message.SetHeader("group_id", groupID)
	return c.SendMessage(message)
}

// SendToGroup отправляет сообщение в группу (через сервер)
func (c *Client) SendToGroup(groupID, msgType string, payload []byte) error {
	message := protocol.NewMessage(msgType, payload)
	message.SetHeader("target_group", groupID)
	return c.SendMessage(message)
}

// SendChat отправляет чат сообщение
func (c *Client) SendChat(text string) error {
	return c.Send(protocol.MessageTypeChat, []byte(text))
}

// SendPlayerMove о��правляет сообщение о движении игрока
func (c *Client) SendPlayerMove(x, y, z float32) error {
	// Простая сериализация координат
	payload := fmt.Sprintf("%.2f,%.2f,%.2f", x, y, z)
	return c.Send(protocol.MessageTypePlayerMove, []byte(payload))
}

// SendPlayerAction отправляет сообщение о действии игрока
func (c *Client) SendPlayerAction(action string, data []byte) error {
	message := protocol.NewMessage(protocol.MessageTypePlayerAction, data)
	message.SetHeader("action", action)
	return c.SendMessage(message)
}

// Ping отправляет ping и измеряет latency
func (c *Client) Ping() (time.Duration, error) {
	if !c.IsConnected() {
		return 0, fmt.Errorf("client not connected")
	}

	start := time.Now()
	pingID := fmt.Sprintf("ping_%d", start.UnixNano())

	// Отправляем ping
	message := protocol.NewMessage("ping", []byte(pingID))
	if err := c.SendMessage(message); err != nil {
		return 0, err
	}

	// Ждем pong (упрощенная реализация)
	// В реальной реализации нужно было ��ы ждать конкретный ответ
	return time.Since(start), nil
}

// GetConnectionInfo возвращает информацию о соединении
func (c *Client) GetConnectionInfo() map[string]interface{} {
	info := make(map[string]interface{})

	if c.conn != nil {
		info["id"] = c.conn.ID()
		info["remote_addr"] = c.conn.RemoteAddr().String()
		info["local_addr"] = c.conn.LocalAddr().String()
		info["last_activity"] = c.conn.LastActivity()
	}

	info["connected"] = c.IsConnected()
	info["metrics"] = c.metrics

	return info
}
