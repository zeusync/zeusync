package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/zeusync/zeusync/internal/core/observability/log"
	"github.com/zeusync/zeusync/internal/core/protocol"
	"github.com/zeusync/zeusync/internal/core/protocol/websocket"
)

// ExampleMiddleware демонстрирует создание middleware
type ExampleMiddleware struct {
	name string
}

func NewExampleMiddleware() *ExampleMiddleware {
	return &ExampleMiddleware{name: "example"}
}

func (m *ExampleMiddleware) Name() string {
	return m.name
}

func (m *ExampleMiddleware) Priority() uint16 {
	return 100
}

func (m *ExampleMiddleware) BeforeHandle(ctx context.Context, client protocol.ClientInfo, message protocol.IMessage) error {
	fmt.Printf("[Middleware] Before handling message type: %s from client: %s\n", message.Type(), client.ID)
	return nil
}

func (m *ExampleMiddleware) AfterHandle(ctx context.Context, client protocol.ClientInfo, message protocol.IMessage, response protocol.IMessage, err error) error {
	if err != nil {
		fmt.Printf("[Middleware] Error handling message: %v\n", err)
	} else {
		fmt.Printf("[Middleware] Successfully handled message type: %s\n", message.Type())
	}
	return nil
}

func (m *ExampleMiddleware) OnConnect(ctx context.Context, client protocol.ClientInfo) error {
	fmt.Printf("[Middleware] Client connected: %s from %s\n", client.ID, client.RemoteAddress)
	return nil
}

func (m *ExampleMiddleware) OnDisconnect(ctx context.Context, client protocol.ClientInfo, reason string) error {
	fmt.Printf("[Middleware] Client disconnected: %s, reason: %s\n", client.ID, reason)
	return nil
}

func main() {
	// Создаем логгер
	logger := log.New(log.LevelDebug)

	// Конфигурация протокола
	config := protocol.Config{
		Host:              "localhost",
		Port:              8080,
		MaxConnections:    1000,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		KeepAliveTimeout:  60 * time.Second,
		MaxMessageSize:    1024 * 1024, // 1MB
		CompressionLevel:  6,
		EnableCompression: true,
		BufferSize:        4096,
		WorkerCount:       10,
		QueueSize:         1000,
		EnableGroups:      true,
		EnableMiddleware:  true,
		EnableMetrics:     true,
		TLSEnabled:        false,
	}

	// Создаем WebSocket протокол
	wsProtocol := websocket.NewWebSocketProtocol(config, logger)

	// Добавляем middleware
	middleware := NewExampleMiddleware()
	if err := wsProtocol.AddMiddleware(middleware); err != nil {
		logger.Fatal("Failed to add middleware", log.Error(err))
	}

	// Регистрируем обработчики сообщений

	// Обработчик для приветствия
	_ = wsProtocol.RegisterHandler("greeting", func(ctx context.Context, client protocol.ClientInfo, message protocol.IMessage) error {
		fmt.Printf("Received greeting from %s: %v\n", client.ID, message.Payload())

		// Отправляем ответ
		response := message.CreateResponse(map[string]interface{}{
			"message": "Hello from server!",
			"time":    time.Now().Format(time.RFC3339),
		})

		return wsProtocol.Send(client.ID, response)
	})

	// Обработчик для эхо сообщений
	_ = wsProtocol.RegisterHandler("echo", func(ctx context.Context, client protocol.ClientInfo, message protocol.IMessage) error {
		fmt.Printf("Echo request from %s: %v\n", client.ID, message.Payload())

		// Отправляем эхо ответ
		response := message.CreateResponse(map[string]interface{}{
			"echo":      message.Payload(),
			"timestamp": time.Now().Unix(),
		})

		return wsProtocol.Send(client.ID, response)
	})

	// Обработчик для присоединения к группе
	_ = wsProtocol.RegisterHandler("join_group", func(ctx context.Context, client protocol.ClientInfo, message protocol.IMessage) error {
		payload, ok := message.Payload().(map[string]interface{})
		if !ok {
			return fmt.Errorf("invalid payload format")
		}

		groupID, ok := payload["group_id"].(string)
		if !ok {
			return fmt.Errorf("group_id is required")
		}

		// Создаем группу если не существует
		_ = wsProtocol.CreateGroup(groupID)

		// Добавляем клиента в группу
		if err := wsProtocol.JoinGroup(client.ID, groupID); err != nil {
			return err
		}

		fmt.Printf("Client %s joined group %s\n", client.ID, groupID)

		// Уведомляем группу о новом участнике
		notification := protocol.NewMessage("group_notification", map[string]interface{}{
			"type":     "user_joined",
			"user_id":  client.ID,
			"group_id": groupID,
			"message":  fmt.Sprintf("User %s joined the group", client.ID),
		})

		return wsProtocol.SendToGroup(groupID, notification)
	})

	// Обработчик для отправки сообщения в группу
	_ = wsProtocol.RegisterHandler("group_message", func(ctx context.Context, client protocol.ClientInfo, message protocol.IMessage) error {
		payload, ok := message.Payload().(map[string]interface{})
		if !ok {
			return fmt.Errorf("invalid payload format")
		}

		groupID, ok := payload["group_id"].(string)
		if !ok {
			return fmt.Errorf("group_id is required")
		}

		messageText, ok := payload["message"].(string)
		if !ok {
			return fmt.Errorf("message is required")
		}

		// Отправляем сообщение всем в группе кроме отправителя
		groupMessage := protocol.NewMessage("group_message_broadcast", map[string]interface{}{
			"from":     client.ID,
			"group_id": groupID,
			"message":  messageText,
			"time":     time.Now().Format(time.RFC3339),
		})

		return wsProtocol.BroadcastExcept([]string{client.ID}, groupMessage)
	})

	// Обработчик для получения метрик
	_ = wsProtocol.RegisterHandler("get_metrics", func(ctx context.Context, client protocol.ClientInfo, message protocol.IMessage) error {
		metrics := wsProtocol.GetMetrics()

		response := message.CreateResponse(map[string]interface{}{
			"active_connections":  metrics.ActiveConnections,
			"total_connections":   metrics.TotalConnections,
			"messages_sent":       metrics.MessagesSent,
			"messages_received":   metrics.MessagesReceived,
			"messages_per_second": metrics.MessagesPerSecond,
			"connection_count":    wsProtocol.GetConnectionCount(),
		})

		return wsProtocol.Send(client.ID, response)
	})

	// Обработчик для получения списка клиентов
	_ = wsProtocol.RegisterHandler("get_clients", func(ctx context.Context, client protocol.ClientInfo, message protocol.IMessage) error {
		clients := wsProtocol.GetAllClients()

		clientList := make([]map[string]interface{}, len(clients))
		for i, c := range clients {
			clientList[i] = map[string]interface{}{
				"id":            c.ID,
				"remote_addr":   c.RemoteAddress,
				"connected_at":  c.ConnectedAt.Format(time.RFC3339),
				"last_activity": c.LastActivity.Format(time.RFC3339),
				"groups":        c.Groups,
			}
		}

		response := message.CreateResponse(map[string]interface{}{
			"clients": clientList,
			"count":   len(clients),
		})

		return wsProtocol.Send(client.ID, response)
	})

	// Устанавливаем обработчик по умолчанию
	wsProtocol.SetDefaultHandler(func(ctx context.Context, client protocol.ClientInfo, message protocol.IMessage) error {
		fmt.Printf("Unknown message type '%s' from client %s\n", message.Type(), client.ID)

		response := message.CreateResponse(map[string]interface{}{
			"error": fmt.Sprintf("Unknown message type: %s", message.Type()),
		})

		return wsProtocol.Send(client.ID, response)
	})

	// Запускаем сервер
	ctx := context.Background()
	if err := wsProtocol.Start(ctx, config); err != nil {
		logger.Fatal("Failed to start WebSocket server", log.Error(err))
	}

	fmt.Printf("WebSocket server started on %s:%d\n", config.Host, config.Port)
	fmt.Println("Available endpoints:")
	fmt.Println("  - ws://localhost:8080/ws - WebSocket connection")
	fmt.Println("  - http://localhost:8080/health - Health check")
	fmt.Println("  - http://localhost:8080/metrics - Metrics")

	// Запускаем горутину для периодической отправки broadcast сообщений
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if wsProtocol.GetConnectionCount() > 0 {
					broadcastMsg := protocol.NewMessage("server_broadcast", map[string]interface{}{
						"message": "Server is alive and running",
						"time":    time.Now().Format(time.RFC3339),
						"clients": wsProtocol.GetConnectionCount(),
					})

					if err := wsProtocol.Broadcast(broadcastMsg); err != nil {
						fmt.Printf("Failed to send broadcast: %v\n", err)
					} else {
						fmt.Println("Sent periodic broadcast to all clients")
					}
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Ожидаем сигнал завершения
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nShutting down server...")

	// Останавливаем сервер
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := wsProtocol.Stop(shutdownCtx); err != nil {
		logger.Fatal("Failed to stop server", log.Error(err))
	}

	fmt.Println("Server stopped gracefully")
}
