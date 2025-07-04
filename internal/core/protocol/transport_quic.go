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
	"math/big"
	rand2 "math/rand"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/quic-go/quic-go"
)

// QUICTransport - реализация Transport для QUIC
type QUICTransport struct {
	listener   *quic.Listener
	tlsConfig  *tls.Config
	quicConfig *quic.Config
	localAddr  net.Addr
	listening  int32
	mu         sync.RWMutex
}

// NewQUICTransport создает новый QUIC транспорт
func NewQUICTransport() Transport {
	return &QUICTransport{
		tlsConfig: generateTLSConfig(),
		quicConfig: &quic.Config{
			MaxIdleTimeout:        30 * time.Second,
			MaxIncomingStreams:    1000,
			MaxIncomingUniStreams: 1000,
			KeepAlivePeriod:       15 * time.Second,
		},
	}
}

// Type возвращает тип транспорта
func (t *QUICTransport) Type() TransportType {
	return TransportQUIC
}

// Listen начинает прослушивание на указанном адресе
func (t *QUICTransport) Listen(address string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if atomic.LoadInt32(&t.listening) == 1 {
		return fmt.Errorf("transport already listening")
	}

	listener, err := quic.ListenAddr(address, t.tlsConfig, t.quicConfig)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", address, err)
	}

	t.listener = listener
	t.localAddr = listener.Addr()
	atomic.StoreInt32(&t.listening, 1)

	return nil
}

// Accept принимает новое соединение
func (t *QUICTransport) Accept() (Connection, error) {
	if atomic.LoadInt32(&t.listening) == 0 {
		return nil, fmt.Errorf("transport not listening")
	}

	conn, err := t.listener.Accept(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to accept connection: %w", err)
	}

	return NewQUICConnection(conn), nil
}

// Dial устанавливает соединение с удаленным адресом
func (t *QUICTransport) Dial(address string) (Connection, error) {
	// Создаем клиентский TLS config
	clientTLSConfig := &tls.Config{
		InsecureSkipVerify: true, // Для разработки
		NextProtos:         []string{"zeusync-quic"},
	}

	conn, err := quic.DialAddr(context.Background(), address, clientTLSConfig, t.quicConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to dial %s: %w", address, err)
	}

	return NewQUICConnection(conn), nil
}

// Close закрывает транспорт
func (t *QUICTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if atomic.LoadInt32(&t.listening) == 0 {
		return nil
	}

	atomic.StoreInt32(&t.listening, 0)

	if t.listener != nil {
		return t.listener.Close()
	}

	return nil
}

// LocalAddr возвращает локальный адрес
func (t *QUICTransport) LocalAddr() net.Addr {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.localAddr
}

// IsListening проверяет, прослушивает ли транспорт
func (t *QUICTransport) IsListening() bool {
	return atomic.LoadInt32(&t.listening) == 1
}

// QUICConnection - реализация Connection для QUIC
type QUICConnection struct {
	conn         *quic.Conn
	stream       *quic.Stream
	id           string
	metadata     sync.Map
	lastActivity int64
	closed       int32
	mu           sync.RWMutex
}

// NewQUICConnection создает новое QUIC соединение
func NewQUICConnection(conn *quic.Conn) *QUICConnection {
	return &QUICConnection{
		conn:         conn,
		id:           generateConnectionID(),
		lastActivity: time.Now().UnixNano(),
	}
}

// ID возвращает уникальный идентификатор соединения
func (c *QUICConnection) ID() string {
	return c.id
}

// RemoteAddr возвращает удаленный адрес
func (c *QUICConnection) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

// LocalAddr возвращает локальный адрес
func (c *QUICConnection) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

// Send отправляет данные
func (c *QUICConnection) Send(data []byte) error {
	if c.IsClosed() {
		return fmt.Errorf("connection closed")
	}

	stream, err := c.getOrCreateStream()
	if err != nil {
		return fmt.Errorf("failed to get stream: %w", err)
	}

	// Отправляем длину сообщения (4 байта) + данные
	lengthBytes := make([]byte, 4)
	lengthBytes[0] = byte(len(data) >> 24)
	lengthBytes[1] = byte(len(data) >> 16)
	lengthBytes[2] = byte(len(data) >> 8)
	lengthBytes[3] = byte(len(data))

	if _, err := stream.Write(lengthBytes); err != nil {
		return fmt.Errorf("failed to write length: %w", err)
	}

	if _, err := stream.Write(data); err != nil {
		return fmt.Errorf("failed to write data: %w", err)
	}

	atomic.StoreInt64(&c.lastActivity, time.Now().UnixNano())
	return nil
}

// Receive получает данные
func (c *QUICConnection) Receive() ([]byte, error) {
	if c.IsClosed() {
		return nil, fmt.Errorf("connection closed")
	}

	stream, err := c.acceptStream()
	if err != nil {
		return nil, fmt.Errorf("failed to accept stream: %w", err)
	}

	// Читаем длину сообщения (4 байта)
	lengthBytes := make([]byte, 4)
	if _, err := stream.Read(lengthBytes); err != nil {
		return nil, fmt.Errorf("failed to read length: %w", err)
	}

	messageLength := int(lengthBytes[0])<<24 | int(lengthBytes[1])<<16 | int(lengthBytes[2])<<8 | int(lengthBytes[3])

	// Читаем данные
	data := make([]byte, messageLength)
	if _, err := stream.Read(data); err != nil {
		return nil, fmt.Errorf("failed to read data: %w", err)
	}

	atomic.StoreInt64(&c.lastActivity, time.Now().UnixNano())
	return data, nil
}

// SendMessage отправляет сообщение
func (c *QUICConnection) SendMessage(message Message) error {
	data, err := message.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}
	return c.Send(data)
}

// ReceiveMessage получает сообщение
func (c *QUICConnection) ReceiveMessage() (Message, error) {
	data, err := c.Receive()
	if err != nil {
		return nil, err
	}

	message := NewMessage("", nil)
	if err := message.Unmarshal(data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	return message, nil
}

// IsAlive проверяет, активно ли соединение
func (c *QUICConnection) IsAlive() bool {
	if c.IsClosed() {
		return false
	}

	// Проверяем контекст соединения
	select {
	case <-c.conn.Context().Done():
		return false
	default:
		return true
	}
}

// IsClosed проверяет, закрыто ли соединение
func (c *QUICConnection) IsClosed() bool {
	return atomic.LoadInt32(&c.closed) == 1
}

// LastActivity возвращает время последней активности
func (c *QUICConnection) LastActivity() time.Time {
	timestamp := atomic.LoadInt64(&c.lastActivity)
	return time.Unix(0, timestamp)
}

// Close закрывает соединение
func (c *QUICConnection) Close() error {
	if !atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		return nil // Уже закрыто
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stream != nil {
		_ = c.stream.Close()
	}

	return c.conn.CloseWithError(0, "connection closed")
}

// SetReadTimeout устанавливает таймаут чтения
func (c *QUICConnection) SetReadTimeout(timeout time.Duration) {
	// QUIC управляет таймаутами на уровне соединения
	// Можно реализовать через context с таймаутом
}

// SetWriteTimeout устанавливает таймаут записи
func (c *QUICConnection) SetWriteTimeout(timeout time.Duration) {
	// QUIC управляет таймаутами на уровне соединения
}

// SetKeepAlive включает/выключает keep-alive
func (c *QUICConnection) SetKeepAlive(enabled bool) {
	// QUIC имеет встроенный keep-alive
}

// GetMetadata получает метаданные
func (c *QUICConnection) GetMetadata(key string) (interface{}, bool) {
	return c.metadata.Load(key)
}

// SetMetadata устанавливает метаданные
func (c *QUICConnection) SetMetadata(key string, value interface{}) {
	c.metadata.Store(key, value)
}

// getOrCreateStream получает или создает поток для отправки
func (c *QUICConnection) getOrCreateStream() (*quic.Stream, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stream == nil {
		stream, err := c.conn.OpenStreamSync(context.Background())
		if err != nil {
			return nil, err
		}
		c.stream = stream
	}

	return c.stream, nil
}

// acceptStream принимает входящий поток
func (c *QUICConnection) acceptStream() (*quic.Stream, error) {
	return c.conn.AcceptStream(context.Background())
}

// Вспомогательные функции

// generateTLSConfig генерирует TLS конфигурацию для разработки
func generateTLSConfig() *tls.Config {
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
	}
}

// generateConnectionID генерирует уникальный ID соединения
func generateConnectionID() string {
	return fmt.Sprintf("conn_%d_%d", time.Now().UnixNano(), rand2.Int63())
}
