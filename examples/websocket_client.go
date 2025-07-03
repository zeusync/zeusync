package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/zeusync/zeusync/internal/core/protocol"
)

type WebSocketClient struct {
	conn     *websocket.Conn
	messages chan protocol.IMessage
	done     chan struct{}
}

func NewWebSocketClient(serverURL string) (*WebSocketClient, error) {
	u, err := url.Parse(serverURL)
	if err != nil {
		return nil, err
	}

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return nil, err
	}

	client := &WebSocketClient{
		conn:     conn,
		messages: make(chan protocol.IMessage, 100),
		done:     make(chan struct{}),
	}

	// Запускаем горутину для чтения сообщений
	go client.readMessages()

	return client, nil
}

func (c *WebSocketClient) readMessages() {
	defer close(c.messages)

	for {
		select {
		case <-c.done:
			return
		default:
			_, data, err := c.conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					fmt.Printf("WebSocket error: %v\n", err)
				}
				return
			}

			// Парсим сообщение
			message := protocol.NewMessage("", nil)
			if err := message.Unmarshal(data); err != nil {
				fmt.Printf("Failed to unmarshal message: %v\n", err)
				continue
			}

			select {
			case c.messages <- message:
			case <-c.done:
				return
			}
		}
	}
}

func (c *WebSocketClient) SendMessage(message protocol.IMessage) error {
	data, err := message.Marshal()
	if err != nil {
		return err
	}

	return c.conn.WriteMessage(websocket.TextMessage, data)
}

func (c *WebSocketClient) ReceiveMessage() (protocol.IMessage, error) {
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

func (c *WebSocketClient) Close() error {
	close(c.done)
	return c.conn.Close()
}

func main() {
	// Подключаемся к серверу
	client, err := NewWebSocketClient("ws://localhost:8080/ws")
	if err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}
	defer func() {
		_ = client.Close()
	}()

	fmt.Println("Connected to WebSocket server")

	// Запускаем горутину для обработки входящих сообщений
	go func() {
		for {
			message, err := client.ReceiveMessage()
			if err != nil {
				fmt.Printf("Error receiving message: %v\n", err)
				return
			}

			fmt.Printf("\n[RECEIVED] Type: %s\n", message.Type())
			fmt.Printf("           Payload: %v\n", message.Payload())
			fmt.Printf("           Time: %s\n", message.Timestamp().Format(time.RFC3339))

			if message.IsResponse() {
				fmt.Printf("           Response to: %s\n", message.ResponseTo())
			}
		}
	}()

	// Демонстрируем различные функции протокола

	// 1. Отправляем приветствие
	fmt.Println("\n=== Sending greeting ===")
	greetingMsg := protocol.NewMessage("greeting", map[string]interface{}{
		"name":    "Go Client",
		"version": "1.0.0",
		"message": "Hello from Go client!",
	})

	if err := client.SendMessage(greetingMsg); err != nil {
		log.Printf("Failed to send greeting: %v", err)
	}

	time.Sleep(1 * time.Second)

	// 2. Отправляем эхо сообщение
	fmt.Println("\n=== Sending echo message ===")
	echoMsg := protocol.NewMessage("echo", map[string]interface{}{
		"text": "This is an echo test",
		"data": []int{1, 2, 3, 4, 5},
	})

	if err := client.SendMessage(echoMsg); err != nil {
		log.Printf("Failed to send echo: %v", err)
	}

	time.Sleep(1 * time.Second)

	// 3. Присоединяемся к группе
	fmt.Println("\n=== Joining group ===")
	joinGroupMsg := protocol.NewMessage("join_group", map[string]interface{}{
		"group_id": "test_group",
	})

	if err := client.SendMessage(joinGroupMsg); err != nil {
		log.Printf("Failed to join group: %v", err)
	}

	time.Sleep(1 * time.Second)

	// 4. Отправляем сообщение в группу
	fmt.Println("\n=== Sending group message ===")
	groupMsg := protocol.NewMessage("group_message", map[string]interface{}{
		"group_id": "test_group",
		"message":  "Hello everyone in the group!",
	})

	if err := client.SendMessage(groupMsg); err != nil {
		log.Printf("Failed to send group message: %v", err)
	}

	time.Sleep(1 * time.Second)

	// 5. Запрашиваем метрики сервера
	fmt.Println("\n=== Requesting server metrics ===")
	metricsMsg := protocol.NewMessage("get_metrics", nil)

	if err := client.SendMessage(metricsMsg); err != nil {
		log.Printf("Failed to request metrics: %v", err)
	}

	time.Sleep(1 * time.Second)

	// 6. Запрашиваем список клиентов
	fmt.Println("\n=== Requesting client list ===")
	clientsMsg := protocol.NewMessage("get_clients", nil)

	if err := client.SendMessage(clientsMsg); err != nil {
		log.Printf("Failed to request clients: %v", err)
	}

	time.Sleep(1 * time.Second)

	// 7. Тестируем сжатие с большим сообщением
	fmt.Println("\n=== Testing compression with large message ===")
	largeData := make([]string, 1000)
	for i := range largeData {
		largeData[i] = fmt.Sprintf("This is line %d of a large message for compression testing", i)
	}

	largeMsg := protocol.NewMessage("echo", map[string]interface{}{
		"large_data": largeData,
		"size":       len(largeData),
	})

	// Включаем сжатие для большого сообщения
	if err = largeMsg.Compress(); err != nil {
		log.Printf("Failed to compress message: %v", err)
	} else {
		fmt.Printf("Message compressed: %t, size: %d bytes\n", largeMsg.IsCompressed(), largeMsg.Size())
	}

	if err = client.SendMessage(largeMsg); err != nil {
		log.Printf("Failed to send large message: %v", err)
	}

	time.Sleep(1 * time.Second)

	// 8. Тестируем приоритеты сообщений
	fmt.Println("\n=== Testing message priorities ===")

	// Низкий приоритет
	lowPriorityMsg := protocol.NewMessage("echo", map[string]interface{}{
		"priority": "low",
		"message":  "Low priority message",
	})
	lowPriorityMsg.SetPriority(protocol.PriorityLow)

	// Высокий приоритет
	highPriorityMsg := protocol.NewMessage("echo", map[string]interface{}{
		"priority": "high",
		"message":  "High priority message",
	})
	highPriorityMsg.SetPriority(protocol.PriorityHigh)

	// Критический приоритет
	criticalMsg := protocol.NewMessage("echo", map[string]interface{}{
		"priority": "critical",
		"message":  "Critical priority message",
	})
	criticalMsg.SetPriority(protocol.PriorityCritical)

	// Отправляем в порядке: низкий, высокий, критический
	_ = client.SendMessage(lowPriorityMsg)
	_ = client.SendMessage(highPriorityMsg)
	_ = client.SendMessage(criticalMsg)

	time.Sleep(1 * time.Second)

	// 9. Тестируем QoS
	fmt.Println("\n=== Testing QoS levels ===")

	// At most once
	qosMsg1 := protocol.NewMessage("echo", map[string]interface{}{
		"qos":     "at_most_once",
		"message": "Fire and forget message",
	})
	qosMsg1.SetQoS(protocol.QoSAtMostOnce)

	// At least once
	qosMsg2 := protocol.NewMessage("echo", map[string]interface{}{
		"qos":     "at_least_once",
		"message": "Acknowledged delivery message",
	})
	qosMsg2.SetQoS(protocol.QoSAtLeastOnce)

	// Exactly once
	qosMsg3 := protocol.NewMessage("echo", map[string]interface{}{
		"qos":     "exactly_once",
		"message": "Assured delivery message",
	})
	qosMsg3.SetQoS(protocol.QoSExactlyOnce)

	_ = client.SendMessage(qosMsg1)
	_ = client.SendMessage(qosMsg2)
	_ = client.SendMessage(qosMsg3)

	time.Sleep(1 * time.Second)

	// 10. Тестируем неизвестный тип сообщения
	fmt.Println("\n=== Testing unknown message type ===")
	unknownMsg := protocol.NewMessage("unknown_type", map[string]interface{}{
		"test": "This should trigger the default handler",
	})

	if err := client.SendMessage(unknownMsg); err != nil {
		log.Printf("Failed to send unknown message: %v", err)
	}

	time.Sleep(2 * time.Second)

	// Ожидаем сигнал завершения
	fmt.Println("\n=== Client ready, press Ctrl+C to exit ===")
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nShutting down client...")
}
