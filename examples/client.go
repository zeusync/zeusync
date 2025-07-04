package main

import (
	"context"
	"github.com/zeusync/zeusync/internal/core/observability/log"
	"github.com/zeusync/zeusync/internal/core/protocol"
	"github.com/zeusync/zeusync/sdk/go/client"
	"time"
)

func main() {
	logger := log.New(log.LevelDebug)

	cfg := client.ClientConfig{
		ServerAddress:     "localhost:8989",
		Transport:         protocol.TransportQUIC,
		ConnectTimeout:    time.Second * 5,
		ReadTimeout:       time.Second * 5,
		WriteTimeout:      time.Second * 5,
		EnableReconnect:   true,
		ReconnectInterval: time.Second * 5,
		MaxReconnectTries: 5,
		EnableHeartbeat:   true,
		HeartbeatInterval: time.Second * 10,
	}

	c := client.NewClient(cfg)
	defer func() {
		_ = c.Disconnect()
	}()

	c.OnConnect(func() {
		logger.Info("Connected to server")
	})

	c.OnDisconnect(func(reason string) {
		logger.Info("Disconnected from server", log.String("reason", reason))
	})

	if err := c.Connect(); err != nil {
		logger.Error("Failed to connect to server", log.Error(err))
		return
	}

	if err := c.Send(protocol.MessageTypePlayerAction, []byte("move forward")); err != nil {
		logger.Error("Failed to send message", log.Error(err))
	}

	c.OnMessage(protocol.MessageTypeGameState, func(ctx context.Context, message protocol.Message) error {
		logger.Info("Received message", log.String("data", string(message.Payload())))
		return nil
	})

	c.On

	time.Sleep(time.Second * 10)
}
