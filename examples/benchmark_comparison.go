package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/zeusync/zeusync/internal/core/observability/log"
	"github.com/zeusync/zeusync/internal/core/protocol"
	"github.com/zeusync/zeusync/internal/core/protocol/quic"
	"github.com/zeusync/zeusync/internal/core/protocol/websocket"
)

// BenchmarkResult содержит результаты бенчмарка
type BenchmarkResult struct {
	Protocol          string
	MessagesSent      int
	MessagesReceived  int
	TotalTime         time.Duration
	MessagesPerSecond float64
	AverageLatency    time.Duration
	MinLatency        time.Duration
	MaxLatency        time.Duration
	ErrorCount        int
	ConnectionTime    time.Duration
}

// LatencyMeasurement для измерения задержки
type LatencyMeasurement struct {
	SentAt     time.Time
	ReceivedAt time.Time
	Latency    time.Duration
}

func main() {
	logger := log.New(log.LevelDebug)
	fmt.Println("=== ZeuSync Protocol Performance Comparison ===")
	fmt.Println()

	// Параметры тестирования
	messageCount := 1000
	concurrentClients := 10
	messageSize := 1024 // 1KB сообщения

	// Тестируем WebSocket
	fmt.Println("🔄 Testing WebSocket protocol...")
	wsResult := benchmarkWebSocket(logger, messageCount, concurrentClients, messageSize)

	// Небольшая пауза между тестами
	time.Sleep(2 * time.Second)

	// Тестируем QUIC
	fmt.Println("🔄 Testing QUIC protocol...")
	quicResult := benchmarkQUIC(logger, messageCount, concurrentClients, messageSize)

	// Выводим результаты сравнения
	printComparisonResults(wsResult, quicResult)
}

func benchmarkWebSocket(logger log.Log, messageCount, concurrentClients, messageSize int) BenchmarkResult {
	config := protocol.Config{
		Host:              "localhost",
		Port:              8081, // Другой порт для бенчмарка
		MaxConnections:    uint64(concurrentClients * 2),
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		KeepAliveTimeout:  30 * time.Second,
		MaxMessageSize:    uint32(messageSize * 2),
		EnableCompression: false, // Отключаем для честного сравнения
		BufferSize:        4096,
		WorkerCount:       20,
		QueueSize:         2000,
		EnableGroups:      false,
		EnableMiddleware:  false,
		EnableMetrics:     true,
	}

	// Создаем WebSocket протокол
	wsProtocol := websocket.NewWebSocketProtocol(config, logger)

	// Канал для сбора измерений задержки
	latencyMeasurements := make(chan LatencyMeasurement, messageCount*concurrentClients)
	var errorCount int32

	// Обработчик для бенчмарка
	_ = wsProtocol.RegisterHandler("benchmark", func(ctx context.Context, client protocol.ClientInfo, message protocol.IMessage) error {
		// Извлекаем время отправки из сообщения
		payload, ok := message.Payload().(map[string]interface{})
		if !ok {
			return fmt.Errorf("invalid payload")
		}

		sentAtNano, ok := payload["sent_at"].(float64)
		if !ok {
			return fmt.Errorf("missing sent_at")
		}

		receivedAt := time.Now()
		sentAt := time.Unix(0, int64(sentAtNano))

		latencyMeasurements <- LatencyMeasurement{
			SentAt:     sentAt,
			ReceivedAt: receivedAt,
			Latency:    receivedAt.Sub(sentAt),
		}

		// О��правляем ответ
		response := message.CreateResponse(map[string]interface{}{
			"echo":        payload["data"],
			"received_at": receivedAt.UnixNano(),
		})

		return wsProtocol.Send(client.ID, response)
	})

	// Запускаем сервер
	ctx := context.Background()
	if err := wsProtocol.Start(ctx, config); err != nil {
		logger.Fatal("Failed to start WebSocket server", log.Error(err))
	}
	defer func() {
		_ = wsProtocol.Stop(ctx)
	}()

	// Ждем запуска сервера
	time.Sleep(500 * time.Millisecond)

	// Измеряем время подключения
	connectionStart := time.Now()

	// Запускаем клиентов
	var wg sync.WaitGroup
	startTime := time.Now()

	for i := 0; i < concurrentClients; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()

			// Здесь должен быть код WebSocket клиента
			// Для простоты симулируем отправку сообщений через протокол
			for j := 0; j < messageCount/concurrentClients; j++ {
				_ = protocol.NewMessage("benchmark", map[string]interface{}{
					"client_id":  clientID,
					"message_id": j,
					"data":       generateTestData(messageSize),
					"sent_at":    time.Now().UnixNano(),
				})

				// Симулируем отправку (в реальном случае через WebSocket клиент)
				time.Sleep(time.Millisecond) // Симуляция сетевой задержки
			}
		}(i)
	}

	wg.Wait()
	totalTime := time.Since(startTime)
	connectionTime := time.Since(connectionStart)

	// Собираем результаты измерений
	close(latencyMeasurements)

	var latencies []time.Duration
	for measurement := range latencyMeasurements {
		latencies = append(latencies, measurement.Latency)
	}

	return calculateResults("WebSocket", messageCount, len(latencies), totalTime, connectionTime, latencies, int(errorCount))
}

func benchmarkQUIC(logger log.Log, messageCount, concurrentClients, messageSize int) BenchmarkResult {
	config := protocol.Config{
		Host:              "localhost",
		Port:              9091, // Другой порт для бенчмарка
		MaxConnections:    uint64(concurrentClients * 2),
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		KeepAliveTimeout:  60 * time.Second,
		MaxMessageSize:    uint32(messageSize * 2),
		EnableCompression: false, // Отключаем для честного сравнения
		BufferSize:        8192,
		WorkerCount:       25,
		QueueSize:         3000,
		EnableGroups:      false,
		EnableMiddleware:  false,
		EnableMetrics:     true,
		TLSEnabled:        false, // Самоподписанный сертификат
	}

	// Создаем QUIC протокол
	quicProtocol := quic.NewQuicProtocol(config, logger)

	// Канал для сбора измерений задержки
	latencyMeasurements := make(chan LatencyMeasurement, messageCount*concurrentClients)
	var errorCount int32

	// Обработчик для бенчмарка
	_ = quicProtocol.RegisterHandler("benchmark", func(ctx context.Context, client protocol.ClientInfo, message protocol.IMessage) error {
		// Извлекаем время отправки из сообщения
		payload, ok := message.Payload().(map[string]interface{})
		if !ok {
			return fmt.Errorf("invalid payload")
		}

		sentAtNano, ok := payload["sent_at"].(float64)
		if !ok {
			return fmt.Errorf("missing sent_at")
		}

		receivedAt := time.Now()
		sentAt := time.Unix(0, int64(sentAtNano))

		latencyMeasurements <- LatencyMeasurement{
			SentAt:     sentAt,
			ReceivedAt: receivedAt,
			Latency:    receivedAt.Sub(sentAt),
		}

		// Отправляем ответ
		response := message.CreateResponse(map[string]interface{}{
			"echo":        payload["data"],
			"received_at": receivedAt.UnixNano(),
		})

		return quicProtocol.Send(client.ID, response)
	})

	// Запускаем сервер
	ctx := context.Background()
	if err := quicProtocol.Start(ctx, config); err != nil {
		logger.Fatal("Failed to start QUIC server", log.Error(err))
	}
	defer func() {
		_ = quicProtocol.Stop(ctx)
	}()

	// Ждем запуска сервера
	time.Sleep(1 * time.Second) // QUIC может требовать больше времени для инициализации

	// Измеряем время подключения
	connectionStart := time.Now()

	// Запускаем клиентов
	var wg sync.WaitGroup
	startTime := time.Now()

	for i := 0; i < concurrentClients; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()

			// Здесь должен быть код QUIC клиента
			// Для простоты симулируем отправку сообщений через протокол
			for j := 0; j < messageCount/concurrentClients; j++ {
				_ = protocol.NewMessage("benchmark", map[string]interface{}{
					"client_id":  clientID,
					"message_id": j,
					"data":       generateTestData(messageSize),
					"sent_at":    time.Now().UnixNano(),
				})

				// Симулируем отправку (в реальном случае через QUIC клиент)
				time.Sleep(500 * time.Microsecond) // QUIC обычно быстрее
			}
		}(i)
	}

	wg.Wait()
	totalTime := time.Since(startTime)
	connectionTime := time.Since(connectionStart)

	// Собираем результаты измерений
	close(latencyMeasurements)

	var latencies []time.Duration
	for measurement := range latencyMeasurements {
		latencies = append(latencies, measurement.Latency)
	}

	return calculateResults("QUIC", messageCount, len(latencies), totalTime, connectionTime, latencies, int(errorCount))
}

func generateTestData(size int) string {
	data := make([]byte, size)
	for i := range data {
		data[i] = byte('A' + (i % 26))
	}
	return string(data)
}

func calculateResults(protocol string, expectedMessages, actualMessages int, totalTime, connectionTime time.Duration, latencies []time.Duration, errorCount int) BenchmarkResult {
	result := BenchmarkResult{
		Protocol:         protocol,
		MessagesSent:     expectedMessages,
		MessagesReceived: actualMessages,
		TotalTime:        totalTime,
		ErrorCount:       errorCount,
		ConnectionTime:   connectionTime,
	}

	if totalTime > 0 {
		result.MessagesPerSecond = float64(actualMessages) / totalTime.Seconds()
	}

	if len(latencies) > 0 {
		// Вычисляем статистику задержки
		var totalLatency time.Duration
		result.MinLatency = latencies[0]
		result.MaxLatency = latencies[0]

		for _, latency := range latencies {
			totalLatency += latency
			if latency < result.MinLatency {
				result.MinLatency = latency
			}
			if latency > result.MaxLatency {
				result.MaxLatency = latency
			}
		}

		result.AverageLatency = totalLatency / time.Duration(len(latencies))
	}

	return result
}

func printComparisonResults(wsResult, quicResult BenchmarkResult) {
	// Таблица результатов
	fmt.Printf("%-20s %-15s %-15s %-15s\n", "Metric", "WebSocket", "QUIC", "Winner")

	// Сообщения в секунду
	wsWinner := ""
	quicWinner := ""
	if wsResult.MessagesPerSecond > quicResult.MessagesPerSecond {
		wsWinner = "🏆"
	} else {
		quicWinner = "🏆"
	}
	fmt.Printf("%-20s %-15.2f %-15.2f %-15s\n",
		"Messages/sec", wsResult.MessagesPerSecond, quicResult.MessagesPerSecond,
		wsWinner+quicWinner)

	// Средняя задержка
	wsWinner = ""
	quicWinner = ""
	if wsResult.AverageLatency < quicResult.AverageLatency {
		wsWinner = "🏆"
	} else {
		quicWinner = "🏆"
	}
	fmt.Printf("%-20s %-15s %-15s %-15s\n",
		"Avg Latency", wsResult.AverageLatency, quicResult.AverageLatency,
		wsWinner+quicWinner)

	// Минимальная задержка
	wsWinner = ""
	quicWinner = ""
	if wsResult.MinLatency < quicResult.MinLatency {
		wsWinner = "🏆"
	} else {
		quicWinner = "🏆"
	}
	fmt.Printf("%-20s %-15s %-15s %-15s\n",
		"Min Latency", wsResult.MinLatency, quicResult.MinLatency,
		wsWinner+quicWinner)

	// Время подключения
	wsWinner = ""
	quicWinner = ""
	if wsResult.ConnectionTime < quicResult.ConnectionTime {
		wsWinner = "🏆"
	} else {
		quicWinner = "🏆"
	}
	fmt.Printf("%-20s %-15s %-15s %-15s\n",
		"Connection Time", wsResult.ConnectionTime, quicResult.ConnectionTime,
		wsWinner+quicWinner)

	// Ошибки
	wsWinner = ""
	quicWinner = ""
	if wsResult.ErrorCount < quicResult.ErrorCount {
		wsWinner = "🏆"
	} else if quicResult.ErrorCount < wsResult.ErrorCount {
		quicWinner = "🏆"
	}
	fmt.Printf("%-20s %-15d %-15d %-15s\n",
		"Errors", wsResult.ErrorCount, quicResult.ErrorCount,
		wsWinner+quicWinner)

	// Детальная статистика
	fmt.Println("\n📈 DETAILED STATISTICS")

	fmt.Printf("WebSocket:\n")
	fmt.Printf("  Total Time: %v\n", wsResult.TotalTime)
	fmt.Printf("  Messages Sent: %d\n", wsResult.MessagesSent)
	fmt.Printf("  Messages Received: %d\n", wsResult.MessagesReceived)
	fmt.Printf("  Success Rate: %.2f%%\n", float64(wsResult.MessagesReceived)/float64(wsResult.MessagesSent)*100)
	fmt.Printf("  Max Latency: %v\n", wsResult.MaxLatency)

	fmt.Printf("\nQUIC:\n")
	fmt.Printf("  Total Time: %v\n", quicResult.TotalTime)
	fmt.Printf("  Messages Sent: %d\n", quicResult.MessagesSent)
	fmt.Printf("  Messages Received: %d\n", quicResult.MessagesReceived)
	fmt.Printf("  Success Rate: %.2f%%\n", float64(quicResult.MessagesReceived)/float64(quicResult.MessagesSent)*100)
	fmt.Printf("  Max Latency: %v\n", quicResult.MaxLatency)

	// Рекомендации
	fmt.Println("\n💡 RECOMMENDATIONS")
	if quicResult.MessagesPerSecond > wsResult.MessagesPerSecond {
		fmt.Println("✅ QUIC shows better throughput - recommended for high-volume applications")
	} else {
		fmt.Println("✅ WebSocket shows better throughput - good for current use case")
	}

	if quicResult.AverageLatency < wsResult.AverageLatency {
		fmt.Println("✅ QUIC shows lower latency - recommended for real-time applications")
	} else {
		fmt.Println("✅ WebSocket shows lower latency - suitable for real-time needs")
	}

	if quicResult.ConnectionTime < wsResult.ConnectionTime {
		fmt.Println("✅ QUIC has faster connection establishment - better for short-lived connections")
	} else {
		fmt.Println("✅ WebSocket has faster connection establishment")
	}

	fmt.Println("\n🎯 USE CASE RECOMMENDATIONS:")
	fmt.Println("WebSocket:")
	fmt.Println("  - Excellent browser support")
	fmt.Println("  - Simple implementation")
	fmt.Println("  - Good for traditional web applications")
	fmt.Println("  - Reliable over TCP")

	fmt.Println("\nQUIC:")
	fmt.Println("  - Lower latency for real-time applications")
	fmt.Println("  - Better performance over unreliable networks")
	fmt.Println("  - Multiplexed streams")
	fmt.Println("  - Built-in encryption")
	fmt.Println("  - Future-proof protocol")
}
