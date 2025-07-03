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
	"github.com/zeusync/zeusync/internal/core/protocol/quic"
)

// QuicMiddleware демонстрирует создание middleware для QUIC
type QuicMiddleware struct {
	name string
}

func NewQuicMiddleware() *QuicMiddleware {
	return &QuicMiddleware{name: "quic-middleware"}
}

func (m *QuicMiddleware) Name() string {
	return m.name
}

func (m *QuicMiddleware) Priority() uint16 {
	return 200
}

func (m *QuicMiddleware) BeforeHandle(ctx context.Context, client protocol.ClientInfo, message protocol.IMessage) error {
	fmt.Printf("[QUIC Middleware] Before handling message type: %s from client: %s\n", message.Type(), client.ID)
	return nil
}

func (m *QuicMiddleware) AfterHandle(ctx context.Context, client protocol.ClientInfo, message protocol.IMessage, response protocol.IMessage, err error) error {
	if err != nil {
		fmt.Printf("[QUIC Middleware] Error handling message: %v\n", err)
	} else {
		fmt.Printf("[QUIC Middleware] Successfully handled message type: %s\n", message.Type())
	}
	return nil
}

func (m *QuicMiddleware) OnConnect(ctx context.Context, client protocol.ClientInfo) error {
	fmt.Printf("[QUIC Middleware] Client connected: %s from %s\n", client.ID, client.RemoteAddress)
	return nil
}

func (m *QuicMiddleware) OnDisconnect(ctx context.Context, client protocol.ClientInfo, reason string) error {
	fmt.Printf("[QUIC Middleware] Client disconnected: %s, reason: %s\n", client.ID, reason)
	return nil
}

func main() {
	// Создаем логгер
	logger := log.New(log.LevelDebug)
	// Конфигурация QUIC протокола
	config := protocol.Config{
		Host:              "localhost",
		Port:              9090,
		MaxConnections:    500,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		KeepAliveTimeout:  120 * time.Second, // QUIC поддерживает более длительные соединения
		MaxMessageSize:    2 * 1024 * 1024,   // 2MB для QUIC
		CompressionLevel:  9,                 // Максимальное сжатие
		EnableCompression: true,
		BufferSize:        8192, // Больший буфер для QUIC
		WorkerCount:       15,   // Больше воркеров для QUIC
		QueueSize:         2000,
		EnableGroups:      true,
		EnableMiddleware:  true,
		EnableMetrics:     true,
		TLSEnabled:        false, // Используем самоподписанный сертификат
	}

	// Создаем QUIC протокол
	quicProtocol := quic.NewQuicProtocol(config, logger)

	// Добавляем middleware
	middleware := NewQuicMiddleware()
	if err := quicProtocol.AddMiddleware(middleware); err != nil {
		logger.Fatal("Failed to add middleware", log.Error(err))
	}

	// Регистрируем обработчики сообщений

	// Обработчик для быстрого ответа (демонстрация скорости QUIC)
	_ = quicProtocol.RegisterHandler("fast_response", func(ctx context.Context, client protocol.ClientInfo, message protocol.IMessage) error {
		start := time.Now()

		response := message.CreateResponse(map[string]interface{}{
			"message":      "Fast QUIC response",
			"processed_in": time.Since(start).Nanoseconds(),
			"protocol":     "QUIC",
			"timestamp":    time.Now().Unix(),
		})

		return quicProtocol.Send(client.ID, response)
	})

	// Обработчик для стресс-теста
	_ = quicProtocol.RegisterHandler("stress_test", func(ctx context.Context, client protocol.ClientInfo, message protocol.IMessage) error {
		payload, ok := message.Payload().(map[string]interface{})
		if !ok {
			return fmt.Errorf("invalid payload format")
		}

		count, ok := payload["count"].(float64)
		if !ok {
			count = 10
		}

		// Отправляем множественные ответы для тестирования производительности
		for i := 0; i < int(count); i++ {
			response := protocol.NewMessage("stress_response", map[string]interface{}{
				"sequence":  i + 1,
				"total":     int(count),
				"data":      fmt.Sprintf("Stress test message #%d", i+1),
				"timestamp": time.Now().UnixNano(),
			})

			if err := quicProtocol.Send(client.ID, response); err != nil {
				return err
			}
		}

		return nil
	})

	// Обработчик для больших данных (демонстрация эффективности QUIC)
	_ = quicProtocol.RegisterHandler("large_data", func(ctx context.Context, client protocol.ClientInfo, message protocol.IMessage) error {
		// Генерируем большой объем данных
		largeData := make([]map[string]interface{}, 1000)
		for i := range largeData {
			largeData[i] = map[string]interface{}{
				"id":          i,
				"name":        fmt.Sprintf("Item_%d", i),
				"description": fmt.Sprintf("This is a detailed description for item number %d with some additional text to make it larger", i),
				"timestamp":   time.Now().Add(time.Duration(i) * time.Second).Unix(),
				"metadata": map[string]interface{}{
					"category": "test",
					"priority": i % 5,
					"tags":     []string{fmt.Sprintf("tag_%d", i%10), fmt.Sprintf("category_%d", i%3)},
				},
			}
		}

		response := message.CreateResponse(map[string]interface{}{
			"data":       largeData,
			"count":      len(largeData),
			"size_bytes": len(fmt.Sprintf("%v", largeData)),
			"compressed": true,
		})

		// Сжимаем большие данные
		if err := response.Compress(); err != nil {
			fmt.Printf("Failed to compress response: %v\n", err)
		}

		return quicProtocol.Send(client.ID, response)
	})

	// Обработчик для многопоточного чата
	_ = quicProtocol.RegisterHandler("chat_room", func(ctx context.Context, client protocol.ClientInfo, message protocol.IMessage) error {
		payload, ok := message.Payload().(map[string]interface{})
		if !ok {
			return fmt.Errorf("invalid payload format")
		}

		action, ok := payload["action"].(string)
		if !ok {
			return fmt.Errorf("action is required")
		}

		switch action {
		case "join":
			roomID, ok := payload["room_id"].(string)
			if !ok {
				return fmt.Errorf("room_id is required")
			}

			// Создаем комнату если не существует
			_ = quicProtocol.CreateGroup(roomID)

			// Добавляем клиента в комнату
			if err := quicProtocol.JoinGroup(client.ID, roomID); err != nil {
				return err
			}

			// Уведомляем всех в комнате
			notification := protocol.NewMessage("chat_notification", map[string]interface{}{
				"type":    "user_joined",
				"room_id": roomID,
				"user_id": client.ID,
				"message": fmt.Sprintf("User %s joined the room", client.ID),
				"time":    time.Now().Format(time.RFC3339),
			})

			return quicProtocol.SendToGroup(roomID, notification)

		case "message":
			roomID, ok := payload["room_id"].(string)
			if !ok {
				return fmt.Errorf("room_id is required")
			}

			messageText, ok := payload["message"].(string)
			if !ok {
				return fmt.Errorf("message is required")
			}

			// Отправляем сообщение всем в комнате
			chatMessage := protocol.NewMessage("chat_message", map[string]interface{}{
				"from":     client.ID,
				"room_id":  roomID,
				"message":  messageText,
				"time":     time.Now().Format(time.RFC3339),
				"protocol": "QUIC",
			})

			return quicProtocol.SendToGroup(roomID, chatMessage)

		case "leave":
			roomID, ok := payload["room_id"].(string)
			if !ok {
				return fmt.Errorf("room_id is required")
			}

			// Удаляем клиента из комнаты
			if err := quicProtocol.LeaveGroup(client.ID, roomID); err != nil {
				return err
			}

			// Уведомляем остальных
			notification := protocol.NewMessage("chat_notification", map[string]interface{}{
				"type":    "user_left",
				"room_id": roomID,
				"user_id": client.ID,
				"message": fmt.Sprintf("User %s left the room", client.ID),
				"time":    time.Now().Format(time.RFC3339),
			})

			return quicProtocol.SendToGroup(roomID, notification)
		}

		return nil
	})

	// Обработчик для файлового обмена (д��монстрация надежности QUIC)
	_ = quicProtocol.RegisterHandler("file_transfer", func(ctx context.Context, client protocol.ClientInfo, message protocol.IMessage) error {
		payload, ok := message.Payload().(map[string]interface{})
		if !ok {
			return fmt.Errorf("invalid payload format")
		}

		action, ok := payload["action"].(string)
		if !ok {
			return fmt.Errorf("action is required")
		}

		switch action {
		case "upload":
			filename, _ := payload["filename"].(string)
			_, _ = payload["data"].(string)
			fileSize, _ := payload["size"].(float64)

			// Симулируем обработку файла
			fmt.Printf("Received file upload: %s (%.0f bytes) from client %s\n", filename, fileSize, client.ID)

			response := message.CreateResponse(map[string]interface{}{
				"status":   "success",
				"filename": filename,
				"size":     fileSize,
				"message":  "File uploaded successfully via QUIC",
				"file_id":  fmt.Sprintf("file_%d", time.Now().Unix()),
			})

			return quicProtocol.Send(client.ID, response)

		case "download":
			fileID, ok := payload["file_id"].(string)
			if !ok {
				return fmt.Errorf("file_id is required")
			}

			// Симулируем отправку файла
			simulatedFileData := make([]byte, 1024*100) // 100KB файл
			for i := range simulatedFileData {
				simulatedFileData[i] = byte(i % 256)
			}

			response := message.CreateResponse(map[string]interface{}{
				"status":   "success",
				"file_id":  fileID,
				"filename": fmt.Sprintf("download_%s.bin", fileID),
				"data":     simulatedFileData,
				"size":     len(simulatedFileData),
				"checksum": "mock_checksum_123456",
			})

			// Сжимаем файл для передачи
			if err := response.Compress(); err != nil {
				fmt.Printf("Failed to compress file: %v\n", err)
			}

			return quicProtocol.Send(client.ID, response)
		}

		return nil
	})

	// Обработчик для получения расширенных метрик QUIC
	_ = quicProtocol.RegisterHandler("quic_metrics", func(ctx context.Context, client protocol.ClientInfo, message protocol.IMessage) error {
		metrics := quicProtocol.GetMetrics()

		response := message.CreateResponse(map[string]interface{}{
			"protocol":              "QUIC",
			"active_connections":    metrics.ActiveConnections,
			"total_connections":     metrics.TotalConnections,
			"failed_connections":    metrics.FailedConnections,
			"messages_sent":         metrics.MessagesSent,
			"messages_received":     metrics.MessagesReceived,
			"messages_per_second":   metrics.MessagesPerSecond,
			"connection_count":      quicProtocol.GetConnectionCount(),
			"server_uptime_seconds": time.Since(time.Now().Add(-time.Hour)).Seconds(), // Mock uptime
			"quic_features": map[string]bool{
				"multiplexing":         true,
				"0rtt_handshake":       true,
				"connection_migration": true,
				"built_in_security":    true,
			},
		})

		return quicProtocol.Send(client.ID, response)
	})

	// Устанавливаем обработчик по умолчанию
	quicProtocol.SetDefaultHandler(func(ctx context.Context, client protocol.ClientInfo, message protocol.IMessage) error {
		fmt.Printf("Unknown QUIC message type '%s' from client %s\n", message.Type(), client.ID)

		response := message.CreateResponse(map[string]interface{}{
			"error":      fmt.Sprintf("Unknown message type: %s", message.Type()),
			"protocol":   "QUIC",
			"suggestion": "Available types: fast_response, stress_test, large_data, chat_room, file_transfer, quic_metrics",
		})

		return quicProtocol.Send(client.ID, response)
	})

	// Запускаем сервер
	ctx := context.Background()
	if err := quicProtocol.Start(ctx, config); err != nil {
		logger.Fatal("Failed to start QUIC server", log.Error(err))
	}

	fmt.Printf("QUIC server started on %s:%d\n", config.Host, config.Port)
	fmt.Println("QUIC Features:")
	fmt.Println("  - Multiplexed streams")
	fmt.Println("  - 0-RTT handshake")
	fmt.Println("  - Built-in encryption (TLS 1.3)")
	fmt.Println("  - Connection migration")
	fmt.Println("  - Improved congestion control")

	// Запускаем горутину для периодических QUIC-специфичных операций
	go func() {
		ticker := time.NewTicker(45 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if quicProtocol.GetConnectionCount() > 0 {
					// Отправляем QUIC-специфичное broadcast сообщение
					broadcastMsg := protocol.NewMessage("quic_broadcast", map[string]interface{}{
						"message":  "QUIC server performance update",
						"time":     time.Now().Format(time.RFC3339),
						"clients":  quicProtocol.GetConnectionCount(),
						"protocol": "QUIC",
						"performance": map[string]interface{}{
							"low_latency":     true,
							"high_throughput": true,
							"reliable":        true,
						},
					})

					if err := quicProtocol.Broadcast(broadcastMsg); err != nil {
						fmt.Printf("Failed to send QUIC broadcast: %v\n", err)
					} else {
						fmt.Println("Sent QUIC performance broadcast to all clients")
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

	fmt.Println("\nShutting down QUIC server...")

	// Останавливаем сервер
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := quicProtocol.Stop(shutdownCtx); err != nil {
		logger.Fatal("Failed to stop QUIC server", log.Error(err))
	}

	fmt.Println("QUIC server stopped gracefully")
}
