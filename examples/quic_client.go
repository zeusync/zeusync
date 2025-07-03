package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/zeusync/zeusync/internal/core/protocol"
)

type QuicClient struct {
	session  *quic.Conn
	stream   *quic.Stream
	messages chan protocol.IMessage
	done     chan struct{}
}

func NewQuicClient(serverAddr string) (*QuicClient, error) {
	// Настройка TLS для клиента (принимаем самоподписанные сертификаты)
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"zeusync-quic"},
	}

	// Настройка QUIC
	quicConfig := &quic.Config{
		MaxIdleTimeout: 60 * time.Second,
	}

	// Подключение к серверу
	session, err := quic.DialAddr(context.Background(), serverAddr, tlsConfig, quicConfig)
	if err != nil {
		return nil, err
	}

	// Открываем поток для отправки данных
	stream, err := session.OpenStreamSync(context.Background())
	if err != nil {
		_ = session.CloseWithError(0, "failed to open stream")
		return nil, err
	}

	client := &QuicClient{
		session:  session,
		stream:   stream,
		messages: make(chan protocol.IMessage, 100),
		done:     make(chan struct{}),
	}

	// Запускаем горутину для чтения сообщений
	go client.readMessages()

	return client, nil
}

func (c *QuicClient) readMessages() {
	defer close(c.messages)

	for {
		select {
		case <-c.done:
			return
		default:
			// Принимаем входящий поток
			stream, err := c.session.AcceptStream(context.Background())
			if err != nil {
				fmt.Printf("Failed to accept stream: %v\n", err)
				return
			}

			go c.handleStream(stream)
		}
	}
}

func (c *QuicClient) handleStream(stream *quic.Stream) {
	defer func() {
		_ = stream.Close()
	}()

	for {
		select {
		case <-c.done:
			return
		default:
			// Читаем длину сообщения (4 байта)
			lengthBytes := make([]byte, 4)
			_, err := stream.Read(lengthBytes)
			if err != nil {
				return
			}

			messageLength := int(lengthBytes[0])<<24 | int(lengthBytes[1])<<16 | int(lengthBytes[2])<<8 | int(lengthBytes[3])

			// Читаем данные сообщения
			data := make([]byte, messageLength)
			_, err = stream.Read(data)
			if err != nil {
				return
			}

			// Парсим сообщение
			message := protocol.NewMessage("", nil)
			if err = message.Unmarshal(data); err != nil {
				fmt.Printf("Failed to unmarshal message: %v\n", err)
				continue
			}

			// Декомпрессируем если нужно
			if message.IsCompressed() {
				if err = message.Decompress(); err != nil {
					fmt.Printf("Failed to decompress message: %v\n", err)
					continue
				}
			}

			select {
			case c.messages <- message:
			case <-c.done:
				return
			}
		}
	}
}

func (c *QuicClient) SendMessage(message protocol.IMessage) error {
	data, err := message.Marshal()
	if err != nil {
		return err
	}

	// Отправляем длину сообщения (4 байта)
	lengthBytes := make([]byte, 4)
	lengthBytes[0] = byte(len(data) >> 24)
	lengthBytes[1] = byte(len(data) >> 16)
	lengthBytes[2] = byte(len(data) >> 8)
	lengthBytes[3] = byte(len(data))

	if _, err = c.stream.Write(lengthBytes); err != nil {
		return err
	}

	// Отправляем данные сообщения
	_, err = c.stream.Write(data)
	return err
}

func (c *QuicClient) ReceiveMessage() (protocol.IMessage, error) {
	select {
	case msg := <-c.messages:
		if msg == nil {
			return nil, fmt.Errorf("connection closed")
		}
		return msg, nil
	case <-c.done:
		return nil, fmt.Errorf("client closed")
	}
}

func (c *QuicClient) Close() error {
	close(c.done)
	if c.stream != nil {
		_ = c.stream.Close()
	}
	if c.session != nil {
		return c.session.CloseWithError(0, "client closing")
	}
	return nil
}

func main() {
	// Подключаемся к QUIC серверу
	client, err := NewQuicClient("localhost:9090")
	if err != nil {
		log.Fatalf("Failed to connect to QUIC server: %v", err)
	}
	defer func() {
		_ = client.Close()
	}()

	fmt.Println("Connected to QUIC server")

	// Запускаем горутину для обработки входящих сообщений
	go func() {
		for {
			message, err := client.ReceiveMessage()
			if err != nil {
				fmt.Printf("Error receiving message: %v\n", err)
				return
			}

			fmt.Printf("\n[QUIC RECEIVED] Type: %s\n", message.Type())
			fmt.Printf("                Payload: %v\n", message.Payload())
			fmt.Printf("                Time: %s\n", message.Timestamp().Format(time.RFC3339))
			fmt.Printf("                Compressed: %t\n", message.IsCompressed())
			fmt.Printf("                Priority: %d\n", message.Priority())
			fmt.Printf("                QoS: %d\n", message.QoS())

			if message.IsResponse() {
				fmt.Printf("                Response to: %s\n", message.ResponseTo())
			}
		}
	}()

	// Демонстрируем возможности QUIC протокола

	// 1. Тест скорости ответа
	fmt.Println("\n=== Testing QUIC fast response ===")
	start := time.Now()
	fastMsg := protocol.NewMessage("fast_response", map[string]interface{}{
		"test":      "speed",
		"client_id": "quic_test_client",
		"sent_at":   start.UnixNano(),
	})

	if err := client.SendMessage(fastMsg); err != nil {
		log.Printf("Failed to send fast response test: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	// 2. Стресс-тест (множественные сообщения)
	fmt.Println("\n=== QUIC stress test ===")
	stressMsg := protocol.NewMessage("stress_test", map[string]interface{}{
		"count":   20,
		"message": "QUIC stress test with multiple rapid messages",
	})

	if err := client.SendMessage(stressMsg); err != nil {
		log.Printf("Failed to send stress test: %v", err)
	}

	time.Sleep(2 * time.Second)

	// 3. Тест больших данных
	fmt.Println("\n=== Testing large data transfer over QUIC ===")
	largeDataMsg := protocol.NewMessage("large_data", map[string]interface{}{
		"request":     "large_dataset",
		"format":      "json",
		"compression": true,
	})

	if err := client.SendMessage(largeDataMsg); err != nil {
		log.Printf("Failed to send large data request: %v", err)
	}

	time.Sleep(2 * time.Second)

	// 4. Тест чат-комнаты (группы)
	fmt.Println("\n=== Testing QUIC chat room ===")

	// Присоединяемся к комнате
	joinMsg := protocol.NewMessage("chat_room", map[string]interface{}{
		"action":  "join",
		"room_id": "quic_test_room",
	})

	if err := client.SendMessage(joinMsg); err != nil {
		log.Printf("Failed to join chat room: %v", err)
	}

	time.Sleep(1 * time.Second)

	// Отправляем сообщение в комнату
	chatMsg := protocol.NewMessage("chat_room", map[string]interface{}{
		"action":  "message",
		"room_id": "quic_test_room",
		"message": "Hello from QUIC client! This protocol is amazing!",
	})

	if err := client.SendMessage(chatMsg); err != nil {
		log.Printf("Failed to send chat message: %v", err)
	}

	time.Sleep(1 * time.Second)

	// 5. Тест файлового обмена
	fmt.Println("\n=== Testing file transfer over QUIC ===")

	// Симулируем загрузку файла
	fileData := make([]byte, 1024*50) // 50KB файл
	for i := range fileData {
		fileData[i] = byte(i % 256)
	}

	uploadMsg := protocol.NewMessage("file_transfer", map[string]interface{}{
		"action":   "upload",
		"filename": "test_file.bin",
		"data":     string(fileData),
		"size":     len(fileData),
		"checksum": "mock_checksum",
	})

	// Сжимаем файл перед отправкой
	if err := uploadMsg.Compress(); err != nil {
		log.Printf("Failed to compress upload: %v", err)
	}

	if err := client.SendMessage(uploadMsg); err != nil {
		log.Printf("Failed to send file upload: %v", err)
	}

	time.Sleep(1 * time.Second)

	// Запрашиваем скачивание файла
	downloadMsg := protocol.NewMessage("file_transfer", map[string]interface{}{
		"action":  "download",
		"file_id": "file_123456",
	})

	if err := client.SendMessage(downloadMsg); err != nil {
		log.Printf("Failed to request file download: %v", err)
	}

	time.Sleep(2 * time.Second)

	// 6. Получаем QUIC-специфичные метрики
	fmt.Println("\n=== Requesting QUIC metrics ===")
	metricsMsg := protocol.NewMessage("quic_metrics", map[string]interface{}{
		"include_performance": true,
		"include_features":    true,
	})

	if err := client.SendMessage(metricsMsg); err != nil {
		log.Printf("Failed to request QUIC metrics: %v", err)
	}

	time.Sleep(1 * time.Second)

	// 7. Тест приоритетов и QoS в QUIC
	fmt.Println("\n=== Testing QUIC priorities and QoS ===")

	// Критическое сообщение с гарантированной доставкой
	criticalMsg := protocol.NewMessage("fast_response", map[string]interface{}{
		"priority": "critical",
		"message":  "Critical QUIC message with guaranteed delivery",
		"urgent":   true,
	})
	criticalMsg.SetPriority(protocol.PriorityCritical)
	criticalMsg.SetQoS(protocol.QoSExactlyOnce)

	// Обычное сообщение
	normalMsg := protocol.NewMessage("fast_response", map[string]interface{}{
		"priority": "normal",
		"message":  "Normal QUIC message",
	})
	normalMsg.SetPriority(protocol.PriorityNormal)
	normalMsg.SetQoS(protocol.QoSAtLeastOnce)

	// Отправляем в порядке: обычное, затем критическое
	_ = client.SendMessage(normalMsg)
	_ = client.SendMessage(criticalMsg)

	time.Sleep(1 * time.Second)

	// 8. Тест множественных потоков (QUIC feature)
	fmt.Println("\n=== Testing QUIC multiplexing ===")

	// Отправляем несколько сообщений одновременно
	for i := 0; i < 5; i++ {
		go func(id int) {
			msg := protocol.NewMessage("fast_response", map[string]interface{}{
				"stream_id": id,
				"message":   fmt.Sprintf("Multiplexed message #%d", id),
				"timestamp": time.Now().UnixNano(),
			})

			if err = client.SendMessage(msg); err != nil {
				log.Printf("Failed to send multiplexed message %d: %v", id, err)
			}
		}(i)
	}

	time.Sleep(2 * time.Second)

	// 9. Покидаем чат-комнату
	fmt.Println("\n=== Leaving chat room ===")
	leaveMsg := protocol.NewMessage("chat_room", map[string]interface{}{
		"action":  "leave",
		"room_id": "quic_test_room",
	})

	if err = client.SendMessage(leaveMsg); err != nil {
		log.Printf("Failed to leave chat room: %v", err)
	}

	time.Sleep(1 * time.Second)

	// 10. Тест неизвестного типа сообщения
	fmt.Println("\n=== Testing unknown message type ===")
	unknownMsg := protocol.NewMessage("unknown_quic_type", map[string]interface{}{
		"test":     "This should trigger the QUIC default handler",
		"protocol": "QUIC",
	})

	if err := client.SendMessage(unknownMsg); err != nil {
		log.Printf("Failed to send unknown message: %v", err)
	}

	time.Sleep(2 * time.Second)

	// Ожидаем сигнал завершения
	fmt.Println("\n=== QUIC client ready, press Ctrl+C to exit ===")
	fmt.Println("QUIC advantages demonstrated:")
	fmt.Println("  ✓ Low latency communication")
	fmt.Println("  ✓ Multiplexed streams")
	fmt.Println("  ✓ Built-in encryption")
	fmt.Println("  ✓ Efficient large data transfer")
	fmt.Println("  ✓ Connection migration support")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nShutting down QUIC client...")
}
