// Package quic provides QUIC transport implementation for the ZeuSync networking protocol.
// This implementation offers high-performance, multiplexed, encrypted connections with
// built-in flow control and congestion management.
package quic

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"github.com/zeusync/zeusync/internal/core/observability/log"
	"github.com/zeusync/zeusync/internal/core/protocol"
	"math/big"
	"net"
	"time"
)

// Package constants
const (
	// DefaultMaxStreams is the default maximum number of concurrent streams per connection
	DefaultMaxStreams = 100

	// DefaultMaxMessageSize is the default maximum message size in bytes
	DefaultMaxMessageSize = 1024 * 1024 // 1MB

	// DefaultIdleTimeout is the default connection idle timeout
	DefaultIdleTimeout = 30 * time.Second

	// DefaultKeepAlive is the default keep-alive interval
	DefaultKeepAlive = 15 * time.Second
)

// NewQUICTransportWithTLS creates a new QUIC transport with custom TLS configuration
func NewQUICTransportWithTLS(tlsConfig *tls.Config) *Transport {
	config := DefaultQUICConfig()
	config.TLSConfig = tlsConfig
	return NewQUICTransport(config, protocol.DefaultGlobalConfig(), log.Provide())
}

// GenerateSelfSignedTLS generates a self-signed TLS certificate for development
func GenerateSelfSignedTLS() (*tls.Config, error) {
	// Generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"ZeuSync"},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{""},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour), // Valid for 1 year
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		DNSNames:              []string{"localhost"},
	}

	// Generate certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, err
	}

	// Create TLS certificate
	cert := tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  privateKey,
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"zeusync-quic"},
		MinVersion:   tls.VersionTLS13,
	}, nil
}

// TransportFactory creates QUIC transport instances
type TransportFactory struct{}

// CreateTransport creates a new QUIC transport instance
func (f *TransportFactory) CreateTransport(config *Config) *Transport {
	return NewQUICTransport(config, protocol.DefaultGlobalConfig(), log.Provide())
}

// CreateSecureTransport creates a new QUIC transport with TLS
func (f *TransportFactory) CreateSecureTransport(tlsConfig *tls.Config) *Transport {
	config := DefaultQUICConfig()
	config.TLSConfig = tlsConfig
	return NewQUICTransport(config, protocol.DefaultGlobalConfig(), log.Provide())
}

// CreateDevelopmentTransport creates a QUIC transport suitable for development
func (f *TransportFactory) CreateDevelopmentTransport() (*Transport, error) {
	tlsConfig, err := GenerateSelfSignedTLS()
	if err != nil {
		return nil, err
	}

	config := DefaultQUICConfig()
	config.TLSConfig = tlsConfig

	return NewQUICTransport(config, protocol.DefaultGlobalConfig(), log.Provide()), nil
}

// Factory is global factory instance
var Factory = &TransportFactory{}

// Helper functions for common QUIC operations

// CreateClientConnection creates a QUIC client connection
func CreateClientConnection(addr string) (protocol.Connection, error) {
	transport := NewQUICTransport(DefaultQUICConfig(), protocol.DefaultGlobalConfig(), log.Provide())
	return transport.Dial(context.Background(), addr)
}

// CreateServerListener creates a QUIC server listener
func CreateServerListener(addr string) (protocol.ConnectionListener, error) {
	transport := NewQUICTransport(DefaultQUICConfig(), protocol.DefaultGlobalConfig(), log.Provide())
	return transport.Listen(context.Background(), addr)
}

// CreateSecureClientConnection creates a secure QUIC client connection
func CreateSecureClientConnection(addr string, tlsConfig *tls.Config) (protocol.Connection, error) {
	transport := NewQUICTransportWithTLS(tlsConfig)
	return transport.Dial(context.Background(), addr)
}

// CreateSecureServerListener creates a secure QUIC server listener
func CreateSecureServerListener(addr string, tlsConfig *tls.Config) (protocol.ConnectionListener, error) {
	transport := NewQUICTransportWithTLS(tlsConfig)
	return transport.Listen(context.Background(), addr)
}
