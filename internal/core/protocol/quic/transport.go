package quic

import (
	"context"
	"crypto/tls"
	"github.com/zeusync/zeusync/internal/core/observability/log"
	"github.com/zeusync/zeusync/internal/core/protocol"
	"net"
	"sync"

	"time"

	"github.com/quic-go/quic-go"
)

// Transport implements the Transport interface for QUIC protocol
type Transport struct {
	*protocol.BaseTransport
	config *Config
	mutex  sync.RWMutex
	logger log.Log
}

// Config holds QUIC-specific configuration
type Config struct {
	// QUIC settings
	MaxIncomingStreams         int64
	MaxIncomingUniStreams      int64
	MaxStreamReceiveWindow     uint64
	MaxConnectionReceiveWindow uint64
	MaxIdleTimeout             time.Duration
	KeepAlivePeriod            time.Duration
	HandshakeIdleTimeout       time.Duration

	// TLS settings
	TLSConfig *tls.Config

	// Performance settings
	InitialStreamReceiveWindow     uint64
	InitialConnectionReceiveWindow uint64
	MaxDatagramFrameSize           int64
	EnableDatagrams                bool

	// Flow control
	DisablePathMTUDiscovery bool

	// Custom QUIC config
	QUICConfig *quic.Config
}

// DefaultQUICConfig returns default QUIC configuration
func DefaultQUICConfig() *Config {
	return &Config{
		MaxIncomingStreams:             100,
		MaxIncomingUniStreams:          100,
		MaxStreamReceiveWindow:         1024 * 1024,      // 1MB
		MaxConnectionReceiveWindow:     15 * 1024 * 1024, // 15MB
		MaxIdleTimeout:                 30 * time.Second,
		KeepAlivePeriod:                15 * time.Second,
		HandshakeIdleTimeout:           10 * time.Second,
		InitialStreamReceiveWindow:     512 * 1024,  // 512KB
		InitialConnectionReceiveWindow: 1024 * 1024, // 1MB
		MaxDatagramFrameSize:           1200,
		EnableDatagrams:                true,
		DisablePathMTUDiscovery:        false,
		TLSConfig:                      generateTLSConfig(),
		QUICConfig:                     &quic.Config{},
	}
}

// NewQUICTransport creates a new QUIC transport
func NewQUICTransport(config *Config, globalConfig *protocol.GlobalConfig, logger log.Log) *Transport {
	if config == nil {
		config = DefaultQUICConfig()
	}
	if globalConfig == nil {
		globalConfig = protocol.DefaultGlobalConfig()
	}
	if logger == nil {
		logger = log.Provide()
	}

	baseTransport := protocol.NewBaseTransport(protocol.TransportQUIC, "QUIC Transport", globalConfig, logger)

	transport := &Transport{
		BaseTransport: baseTransport,
		config:        config,
		logger:        logger.With(log.String("transport", "quic")),
	}

	transport.logger.Info("QUIC transport initialized",
		log.Int64("max_streams", config.MaxIncomingStreams),
		log.Duration("idle_timeout", config.MaxIdleTimeout),
		log.Bool("enable_datagrams", config.EnableDatagrams))

	return transport
}

// SupportedFeatures returns the features supported by QUIC transport
func (t *Transport) SupportedFeatures() protocol.TransportFeatures {
	return protocol.TransportFeatures{
		Reliable:        true,
		Ordered:         true,
		Multiplexing:    true,
		Encryption:      true,
		Compression:     false, // QUIC doesn't provide built-in compression
		FlowControl:     true,
		Bidirectional:   true,
		LowLatency:      true,
		HighThroughput:  true,
		ConnectionReuse: true,
		StreamSupport:   true,
		MessageFraming:  false, // We handle framing at application level
	}
}

// Listen creates a QUIC listener
func (t *Transport) Listen(ctx context.Context, addr string) (protocol.ConnectionListener, error) {
	return t.ListenWithConfig(ctx, addr, t.Config())
}

// ListenWithConfig creates a QUIC listener with custom configuration
func (t *Transport) ListenWithConfig(ctx context.Context, addr string, config *protocol.GlobalConfig) (protocol.ConnectionListener, error) {
	if t.IsClosed() {
		t.logger.Error("Attempted to listen on closed transport")
		return nil, protocol.ErrTransportClosed
	}

	t.logger.Info("Creating QUIC listener", log.String("addr", addr))

	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		t.UpdateStats("errors_encountered", 1)
		t.logger.Error("Failed to resolve UDP address", log.String("addr", addr), log.Error(err))
		return nil, protocol.WrapError(err, "failed to resolve UDP address")
	}

	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		t.UpdateStats("errors_encountered", 1)
		t.logger.Error("Failed to listen on UDP", log.String("addr", addr), log.Error(err))
		return nil, protocol.WrapError(err, "failed to listen on UDP")
	}

	quicConfig, err := t.buildQUICConfig(config)
	if err != nil {
		_ = udpConn.Close()
		t.UpdateStats("errors_encountered", 1)
		t.logger.Error("Failed to build QUIC config", log.Error(err))
		return nil, protocol.WrapError(err, "failed to build QUIC config")
	}

	listener, err := quic.Listen(udpConn, t.config.TLSConfig, quicConfig)
	if err != nil {
		_ = udpConn.Close()
		t.UpdateStats("errors_encountered", 1)
		t.logger.Error("Failed to create QUIC listener", log.Error(err))
		return nil, protocol.WrapError(err, "failed to create QUIC listener")
	}

	t.logger.Info("QUIC listener created successfully", log.String("addr", listener.Addr().String()))

	return NewQUICListener(listener, t, config, t.logger), nil
}

// Dial creates a QUIC connection
func (t *Transport) Dial(ctx context.Context, addr string) (protocol.Connection, error) {
	return t.DialWithConfig(ctx, addr, t.Config())
}

// DialWithConfig creates a QUIC connection with custom configuration
func (t *Transport) DialWithConfig(ctx context.Context, addr string, config *protocol.GlobalConfig) (protocol.Connection, error) {
	if t.IsClosed() {
		t.logger.Error("Attempted to dial on closed transport")
		return nil, protocol.ErrTransportClosed
	}

	t.logger.Debug("Dialing QUIC connection", log.String("addr", addr))

	quicConfig, err := t.buildQUICConfig(config)
	if err != nil {
		t.UpdateStats("errors_encountered", 1)
		t.logger.Error("Failed to build QUIC config", log.Error(err))
		return nil, protocol.WrapError(err, "failed to build QUIC config")
	}

	// Create TLS config for client
	tlsConfig := t.config.TLSConfig.Clone()
	if tlsConfig.ServerName == "" {
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			tlsConfig.ServerName = addr
		} else {
			tlsConfig.ServerName = host
		}
	}

	conn, err := quic.DialAddr(ctx, addr, tlsConfig, quicConfig)
	if err != nil {
		t.UpdateStats("errors_encountered", 1)
		t.UpdateStats("connections_failed", 1)
		t.logger.Error("Failed to dial QUIC connection", log.String("addr", addr), log.Error(err))
		return nil, protocol.WrapError(err, "failed to dial QUIC connection")
	}

	t.UpdateStats("connections_dialed", 1)
	t.UpdateStats("connections_active", 1)

	t.logger.Info("QUIC connection established",
		log.String("local_addr", conn.LocalAddr().String()),
		log.String("remote_addr", conn.RemoteAddr().String()))

	return NewQUICConnection(conn, t, config, t.logger), nil
}

// HealthCheck performs a health check on the QUIC transport
func (t *Transport) HealthCheck(ctx context.Context) protocol.HealthCheck {
	start := time.Now()

	if t.IsClosed() {
		return protocol.HealthCheck{
			Status:    protocol.HealthStatusUnhealthy,
			Timestamp: time.Now(),
			Latency:   time.Since(start),
			Error:     protocol.ErrTransportClosed,
		}
	}

	// For QUIC, we can check if we can create a basic configuration
	_, err := t.buildQUICConfig(t.Config())
	if err != nil {
		t.logger.Warn("QUIC health check failed", log.Error(err))
		return protocol.HealthCheck{
			Status:    protocol.HealthStatusUnhealthy,
			Timestamp: time.Now(),
			Latency:   time.Since(start),
			Error:     err,
		}
	}

	return protocol.HealthCheck{
		Status:    protocol.HealthStatusHealthy,
		Timestamp: time.Now(),
		Latency:   time.Since(start),
	}
}

// buildQUICConfig builds a quic.Config from protocol configuration
func (t *Transport) buildQUICConfig(config *protocol.GlobalConfig) (*quic.Config, error) {
	if config == nil {
		config = protocol.DefaultGlobalConfig()
	}

	quicConfig := &quic.Config{
		MaxIncomingStreams:             t.config.MaxIncomingStreams,
		MaxIncomingUniStreams:          t.config.MaxIncomingUniStreams,
		MaxStreamReceiveWindow:         t.config.MaxStreamReceiveWindow,
		MaxConnectionReceiveWindow:     t.config.MaxConnectionReceiveWindow,
		MaxIdleTimeout:                 t.config.MaxIdleTimeout,
		KeepAlivePeriod:                t.config.KeepAlivePeriod,
		HandshakeIdleTimeout:           t.config.HandshakeIdleTimeout,
		InitialStreamReceiveWindow:     t.config.InitialStreamReceiveWindow,
		InitialConnectionReceiveWindow: t.config.InitialConnectionReceiveWindow,
		EnableDatagrams:                t.config.EnableDatagrams,
		DisablePathMTUDiscovery:        t.config.DisablePathMTUDiscovery,
	}

	// Override with protocol config values if specified
	if config.IdleTimeout > 0 {
		quicConfig.MaxIdleTimeout = config.IdleTimeout
		t.logger.Debug("Overriding idle timeout", log.Duration("timeout", config.IdleTimeout))
	}
	if config.KeepAlive > 0 {
		quicConfig.KeepAlivePeriod = config.KeepAlive
		t.logger.Debug("Overriding keep alive", log.Duration("keep_alive", config.KeepAlive))
	}
	if config.MaxStreams > 0 {
		quicConfig.MaxIncomingStreams = int64(config.MaxStreams)
		quicConfig.MaxIncomingUniStreams = int64(config.MaxStreams)
		t.logger.Debug("Overriding max streams", log.Int("max_streams", config.MaxStreams))
	}

	return quicConfig, nil
}

// generateTLSConfig generates a basic TLS configuration for QUIC
func generateTLSConfig() *tls.Config {
	return &tls.Config{
		InsecureSkipVerify: true, // For development only
		NextProtos:         []string{"zeusync-quic"},
		MinVersion:         tls.VersionTLS13, // QUIC requires TLS 1.3
	}
}

// Close closes the QUIC transport
func (t *Transport) Close() error {
	t.logger.Info("Closing QUIC transport")
	return t.BaseTransport.Close()
}

// GetQUICConfig returns the QUIC-specific configuration
func (t *Transport) GetQUICConfig() *Config {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	return t.config
}

// UpdateQUICConfig updates the QUIC-specific configuration
func (t *Transport) UpdateQUICConfig(config *Config) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.config = config
	t.logger.Info("QUIC config updated")
}
