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
	"io"
	"math/big"
	"net"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
)

// --- QUIC Transport Implementation ---

// QuicTransport implements the Transport interface using the QUIC protocol.
type QuicTransport struct {
	listener *quic.Listener
	mu       sync.RWMutex
}

// NewQuicTransport creates a new, uninitialized QuicTransport.
func NewQuicTransport() *QuicTransport {
	return &QuicTransport{}
}

// Listen starts a QUIC listener on the given address.
func (t *QuicTransport) Listen(ctx context.Context, address string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.listener != nil {
		return fmt.Errorf("transport is already listening")
	}

	tlsConfig, err := generateInMemoryTLSConfig()
	if err != nil {
		return fmt.Errorf("failed to generate TLS config: %w", err)
	}

	listener, err := quic.ListenAddr(address, tlsConfig, &quic.Config{
		EnableDatagrams: true,
	})
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", address, err)
	}

	t.listener = listener
	return nil
}

// Accept waits for and returns the next connection to the listener.
func (t *QuicTransport) Accept(ctx context.Context) (Connection, error) {
	t.mu.RLock()
	if t.listener == nil {
		t.mu.RUnlock()
		return nil, fmt.Errorf("transport is not listening")
	}
	t.mu.RUnlock()

	conn, err := t.listener.Accept(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to accept connection: %w", err)
	}

	return &QuicConnection{conn: conn}, nil
}

// Dial creates a new client connection to the given address.
func (t *QuicTransport) Dial(ctx context.Context, address string) (Connection, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true, // WARNING: Not for production use!
		NextProtos:         []string{"zeusync-quic"},
	}

	conn, err := quic.DialAddr(ctx, address, tlsConfig, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to dial %s: %w", address, err)
	}

	return &QuicConnection{conn: conn}, nil
}

// Close closes the transport listener.
func (t *QuicTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.listener != nil {
		return t.listener.Close()
	}

	return nil
}

// --- QUIC Connection Implementation ---

// QuicConnection wraps a quic.Connection to implement the Connection interface.
type QuicConnection struct {
	conn quic.Connection
}

// Read reads data from an incoming QUIC stream.
func (c *QuicConnection) Read(ctx context.Context) ([]byte, error) {
	stream, err := c.conn.AcceptStream(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to accept stream: %w", err)
	}

	// Use a simple length-prefix framing to read the message.
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(stream, lenBuf); err != nil {
		return nil, fmt.Errorf("failed to read message length: %w", err)
	}

	length := int(lenBuf[0])<<24 | int(lenBuf[1])<<16 | int(lenBuf[2])<<8 | int(lenBuf[3])

	// It's a good idea to have a max message size to prevent abuse.
	// const maxMessageSize = 1 << 20 // 1MB
	// if length > maxMessageSize {
	// 	return nil, fmt.Errorf("message size %d exceeds limit %d", length, maxMessageSize)
	// }

	data := make([]byte, length)
	if _, err := io.ReadFull(stream, data); err != nil {
		return nil, fmt.Errorf("failed to read message data: %w", err)
	}

	return data, nil
}

// Write writes data to an outgoing QUIC stream.
func (c *QuicConnection) Write(ctx context.Context, data []byte) error {
	stream, err := c.conn.OpenStreamSync(ctx)
	if err != nil {
		return fmt.Errorf("failed to open stream: %w", err)
	}
	defer stream.Close()

	// Use a simple length-prefix framing to write the message.
	lenBuf := make([]byte, 4)
	length := len(data)
	lenBuf[0] = byte(length >> 24)
	lenBuf[1] = byte(length >> 16)
	lenBuf[2] = byte(length >> 8)
	lenBuf[3] = byte(length)

	if _, err := stream.Write(lenBuf); err != nil {
		return fmt.Errorf("failed to write message length: %w", err)
	}

	if _, err := stream.Write(data); err != nil {
		return fmt.Errorf("failed to write message data: %w", err)
	}

	return nil
}

// Close closes the QUIC connection with a normal status.
func (c *QuicConnection) Close() error {
	return c.conn.CloseWithError(0, "connection closed")
}

// LocalAddr returns the local network address.
func (c *QuicConnection) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

// RemoteAddr returns the remote network address.
func (c *QuicConnection) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

// --- TLS Configuration ---

// generateInMemoryTLSConfig creates a self-signed TLS configuration for demonstration purposes.
// In a real application, you would load a certificate from a file.
func generateInMemoryTLSConfig() (*tls.Config, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"ZeuSync"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}
	privBytes, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privBytes})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		NextProtos:   []string{"zeusync-quic"}, // Protocol negotiation
	}, nil
}
