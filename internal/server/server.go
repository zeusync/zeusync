package server

import (
	"context"
	"github.com/zeusync/zeusync/internal/core/fields"
	"github.com/zeusync/zeusync/internal/core/protocol"
	"time"

	"github.com/zeusync/zeusync/internal/core/observability/log"
)

type Server struct {
	ctx       context.Context
	runCancel context.CancelFunc
	server    protocol.Protocol
	serverCfg protocol.Config
	logger    *log.Logger
}

func NewServer(globalCtx context.Context) *Server {
	logger := log.New(log.LevelDebug)

	transport := protocol.NewQUICTransport()
	cfg := protocol.Config{
		Host:              "localhost",
		Port:              8989,
		ReadTimeout:       time.Second * 5,
		WriteTimeout:      time.Second * 5,
		IdleTimeout:       time.Second * 30,
		ConnectTimeout:    time.Second * 5,
		MaxConnections:    1,
		MaxMessageSize:    1024 * 1024 * 2,
		BufferSize:        1024 * 1024 * 2,
		MaxGroupSize:      16,
		WorkerCount:       8,
		QueueSize:         100,
		CompressionLevel:  1,
		TLSEnabled:        false,
		EnableMetrics:     true,
		EnableHeartbeat:   true,
		HeartbeatInterval: time.Second * 10,
		SerializerType:    "binary",
	}

	server := protocol.NewServer(cfg, transport)

	connectField := fields.NewFA(0)
	activeField := fields.NewFA(0)
	metricsField := fields.NewFA(server.GetMetrics())

	runCtx, cancel := context.WithCancel(context.Background())

	ticker := time.NewTicker(time.Second)

	go func() {
		for {
			select {
			case <-runCtx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				metricsField.Set(server.GetMetrics())
				activeField.Transaction(func(current int) int {
					return current + 1
				})
			default:
			}
		}
	}()

	activeField.Subscribe(func(newValue int) {
		if data, err := activeField.Serialize(); err != nil {
			logger.Error("Failed to serialize active field", log.Error(err))
		} else {
			msg := protocol.NewMessage(protocol.MessageTypeGameState, data)
			if err = server.Broadcast(msg); err != nil {
				logger.Error("Failed to broadcast message", log.Error(err))
			}
		}
	})

	/*metricsField.Subscribe(func(metrics protocol.Metrics) {
		logger.Info("Metrics updated",
			log.Int64("active_connections", metrics.ActiveConnections),
			log.Int64("total_connections", metrics.TotalConnections),
			log.Int64("failed_connection", metrics.FailedConnections),

			log.Int64("message_sent", metrics.MessagesSent),
			log.Int64("message_received", metrics.MessagesReceived),
			log.Int64("message_dropped", metrics.MessagesDropped),
			log.Int64("message_queued", metrics.MessagesQueued),

			log.Float64("messages_per_second", metrics.MessagesPerSecond),
			log.Duration("average_message_latency", metrics.AverageLatency),
			log.Float64("average_message_size", metrics.AverageMessageSize),
			log.Int64("bytes_sent", metrics.BytesSent),
			log.Int64("bytes_received", metrics.BytesReceived),

			log.Int64("active_groups", metrics.ActiveGroups),
			log.Int64("total_groups", metrics.TotalGroups),
		)
	})*/

	server.OnClientConnect(func(ctx context.Context, client protocol.ClientInfo) error {
		connectField.Transaction(func(current int) int {
			return current + 1
		})

		msgPayload, err := connectField.Serialize()
		if err != nil {
			return err
		}

		connectMsg := protocol.NewMessage(protocol.MessageTypeConnect, msgPayload)
		return server.Broadcast(connectMsg)
	})

	server.OnClientDisconnect(func(ctx context.Context, client protocol.ClientInfo, reason string) error {
		connectField.Transaction(func(current int) int {
			return current - 1
		})

		msgPayload, err := connectField.Serialize()
		if err != nil {
			return err
		}

		connectMsg := protocol.NewMessage(protocol.MessageTypeConnect, msgPayload)
		return server.Broadcast(connectMsg)
	})

	server.OnMessage(func(ctx context.Context, client protocol.ClientInfo, message protocol.Message) error {
		logger.Info("Received message from client",
			log.String("client_id", client.ID),
			log.String("remoter_address", client.RemoteAddress),
			log.String("message_type", message.Type()),
			log.String("data", string(message.Payload())),
		)

		return nil
	})

	return &Server{
		ctx:       globalCtx,
		runCancel: cancel,
		server:    server,
		serverCfg: cfg,
		logger:    logger,
	}
}

func (s *Server) Start(ctx context.Context) error {

	s.logger.Info("Starting server", log.String("host", s.serverCfg.Host), log.Int("port", s.serverCfg.Port))
	if err := s.server.Start(ctx, s.serverCfg); err != nil {
		s.logger.Fatal("Failed to start server", log.Error(err))
		return err
	}

	return nil
}

func (s *Server) Stop() error {
	s.runCancel()
	s.logger.Info("Stopping server")
	return s.server.Stop(s.ctx)
}
