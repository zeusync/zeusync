package protocol

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestServerClient_BasicCommunication(t *testing.T) {
	// Создаем сервер
	config := Config{
		Host:              "localhost",
		Port:              0, // Автоматический выбор порта
		MaxConnections:    10,
		WorkerCount:       2,
		QueueSize:         100,
		EnableHeartbeat:   false,
		EnableMetrics:     true,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      5 * time.Second,
		HeartbeatInterval: 1 * time.Second,
	}

	transport := NewQUICTransport()
	server := NewServer(config, transport)

	// Обработчики сервера
	var connectCount int32
	var messageCount int32

	server.OnClientConnect(func(ctx context.Context, client ClientInfo) error {
		atomic.AddInt32(&connectCount, 1)
		return nil
	})

	server.OnMessage(func(ctx context.Context, client ClientInfo, message Message) error {
		atomic.AddInt32(&messageCount, 1)

		// Отправляем ответ
		response := NewMessage("response", []byte("pong"))
		return server.(*Server).SendToClient(client.ID, response)
	})

	// Запускаем сервер
	ctx := context.Background()
	err := server.Start(ctx, config)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() {
		_ = server.Stop(ctx)
	}()

	// Ждем запуска сервера
	time.Sleep(100 * time.Millisecond)

	// Получаем адрес сервера
	serverAddr := transport.LocalAddr().String()

	// Создаем клиентское соединение
	clientTransport := NewQUICTransport()
	conn, err := clientTransport.Dial(serverAddr)
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer func() {
		_ = conn.Close()
	}()

	// Ждем подключения
	time.Sleep(100 * time.Millisecond)

	// Проверяем, что клиент подключился
	if atomic.LoadInt32(&connectCount) != 1 {
		t.Errorf("Expected 1 connection, got %d", atomic.LoadInt32(&connectCount))
	}

	// Отправляем сообщение
	message := NewMessage("ping", []byte("hello"))
	err = conn.SendMessage(message)
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// Ждем обработки
	time.Sleep(100 * time.Millisecond)

	// Проверяем, что сообщение получено
	if atomic.LoadInt32(&messageCount) != 1 {
		t.Errorf("Expected 1 message, got %d", atomic.LoadInt32(&messageCount))
	}

	// Получаем ответ
	response, err := conn.ReceiveMessage()
	if err != nil {
		t.Fatalf("Failed to receive response: %v", err)
	}

	if response.Type() != "response" {
		t.Errorf("Expected response type 'response', got '%s'", response.Type())
	}

	if string(response.Payload()) != "pong" {
		t.Errorf("Expected response payload 'pong', got '%s'", string(response.Payload()))
	}
}

func TestServerClient_MultipleClients(t *testing.T) {
	// Создаем сервер
	config := Config{
		Host:           "localhost",
		Port:           0,
		MaxConnections: 100,
		WorkerCount:    4,
		QueueSize:      1000,
		EnableMetrics:  true,
	}

	transport := NewQUICTransport()
	server := NewServer(config, transport)

	var connectCount int32
	var messageCount int32

	server.OnClientConnect(func(ctx context.Context, client ClientInfo) error {
		atomic.AddInt32(&connectCount, 1)
		return nil
	})

	server.OnMessage(func(ctx context.Context, client ClientInfo, message Message) error {
		atomic.AddInt32(&messageCount, 1)
		return nil
	})

	// Запускаем сервер
	ctx := context.Background()
	err := server.Start(ctx, config)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() {
		_ = server.Stop(ctx)
	}()

	time.Sleep(100 * time.Millisecond)
	serverAddr := transport.LocalAddr().String()

	// Создаем несколько клиентов
	clientCount := 10
	var wg sync.WaitGroup
	var clientErrors int32

	for i := 0; i < clientCount; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()

			clientTransport := NewQUICTransport()
			conn, err := clientTransport.Dial(serverAddr)
			if err != nil {
				atomic.AddInt32(&clientErrors, 1)
				return
			}
			defer func() {
				_ = conn.Close()
			}()

			// Отправляем несколько сообщений
			for j := 0; j < 5; j++ {
				message := NewMessage("test", []byte("message"))
				err = conn.SendMessage(message)
				if err != nil {
					atomic.AddInt32(&clientErrors, 1)
					return
				}
			}

			time.Sleep(100 * time.Millisecond)
		}(i)
	}

	wg.Wait()

	// Проверяем результаты
	if atomic.LoadInt32(&clientErrors) > 0 {
		t.Errorf("Got %d client errors", atomic.LoadInt32(&clientErrors))
	}

	if atomic.LoadInt32(&connectCount) != int32(clientCount) {
		t.Errorf("Expected %d connections, got %d", clientCount, atomic.LoadInt32(&connectCount))
	}

	// Ждем обработки всех сообщений
	time.Sleep(200 * time.Millisecond)

	expectedMessages := int32(clientCount * 5)
	if atomic.LoadInt32(&messageCount) != expectedMessages {
		t.Errorf("Expected %d messages, got %d", expectedMessages, atomic.LoadInt32(&messageCount))
	}
}

func TestServerClient_Groups(t *testing.T) {
	// Создаем сервер
	config := Config{
		Host:         "localhost",
		Port:         0,
		WorkerCount:  2,
		QueueSize:    100,
		MaxGroupSize: 10,
	}

	transport := NewQUICTransport()
	server := NewServer(config, transport)

	// Запускаем сервер
	ctx := context.Background()
	err := server.Start(ctx, config)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() {
		_ = server.Stop(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// Создаем группу
	err = server.CreateGroup("test_group")
	if err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}

	// Создаем клиентов
	serverAddr := transport.LocalAddr().String()

	client1Transport := NewQUICTransport()
	conn1, err := client1Transport.Dial(serverAddr)
	if err != nil {
		t.Fatalf("Failed to connect client1: %v", err)
	}
	defer func() {
		_ = conn1.Close()
	}()

	client2Transport := NewQUICTransport()
	conn2, err := client2Transport.Dial(serverAddr)
	if err != nil {
		t.Fatalf("Failed to connect client2: %v", err)
	}
	defer func() {
		_ = conn2.Close()
	}()

	time.Sleep(100 * time.Millisecond)

	// Получаем ID клиентов
	clients := server.GetClients()
	if len(clients) != 2 {
		t.Fatalf("Expected 2 clients, got %d", len(clients))
	}

	client1ID := clients[0].ID
	client2ID := clients[1].ID

	// Добавляем клиентов в группу
	err = server.AddClientToGroup(client1ID, "test_group")
	if err != nil {
		t.Fatalf("Failed to add client1 to group: %v", err)
	}

	err = server.AddClientToGroup(client2ID, "test_group")
	if err != nil {
		t.Fatalf("Failed to add client2 to group: %v", err)
	}

	// Проверяем участников группы
	groupClients := server.GetGroupClients("test_group")
	if len(groupClients) != 2 {
		t.Errorf("Expected 2 clients in group, got %d", len(groupClients))
	}

	// Отправляем сообщение в группу
	groupMessage := NewMessage("group_test", []byte("hello group"))
	err = server.SendToGroup("test_group", groupMessage)
	if err != nil {
		t.Fatalf("Failed to send to group: %v", err)
	}

	// Проверяем, что оба клиента получили сообщение
	// (В реальном тесте нужно было бы настроить получение сообщений)

	// Удаляем клиента из группы
	err = server.RemoveClientFromGroup(client1ID, "test_group")
	if err != nil {
		t.Fatalf("Failed to remove client from group: %v", err)
	}

	// Проверяем, что в группе остался один клиент
	groupClients = server.GetGroupClients("test_group")
	if len(groupClients) != 1 {
		t.Errorf("Expected 1 client in group after removal, got %d", len(groupClients))
	}

	// Удаляем группу
	err = server.DeleteGroup("test_group")
	if err != nil {
		t.Fatalf("Failed to delete group: %v", err)
	}

	// Проверяем, что группа удалена
	groupClients = server.GetGroupClients("test_group")
	if groupClients != nil {
		t.Error("Group should be deleted")
	}
}

func TestServerClient_Metrics(t *testing.T) {
	// Создаем сервер с метриками
	config := Config{
		Host:          "localhost",
		Port:          0,
		WorkerCount:   2,
		QueueSize:     100,
		EnableMetrics: true,
	}

	transport := NewQUICTransport()
	server := NewServer(config, transport)

	var messageCount int32
	server.OnMessage(func(ctx context.Context, client ClientInfo, message Message) error {
		atomic.AddInt32(&messageCount, 1)
		return nil
	})

	// Запускаем сервер
	ctx := context.Background()
	err := server.Start(ctx, config)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() {
		_ = server.Stop(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// Подключаем клиента
	serverAddr := transport.LocalAddr().String()
	clientTransport := NewQUICTransport()
	conn, err := clientTransport.Dial(serverAddr)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer func() {
		_ = conn.Close()
	}()

	time.Sleep(100 * time.Millisecond)

	// Отправляем сообщения
	for i := 0; i < 10; i++ {
		message := NewMessage("test", []byte("test"))
		err = conn.SendMessage(message)
		if err != nil {
			t.Fatalf("Failed to send message %d: %v", i, err)
		}
	}

	time.Sleep(200 * time.Millisecond)

	// Проверяем метрики
	metrics := server.GetMetrics()

	if metrics.ActiveConnections != 1 {
		t.Errorf("Expected 1 active connection, got %d", metrics.ActiveConnections)
	}

	if metrics.TotalConnections != 1 {
		t.Errorf("Expected 1 total connection, got %d", metrics.TotalConnections)
	}

	if metrics.MessagesReceived != 10 {
		t.Errorf("Expected 10 messages received, got %d", metrics.MessagesReceived)
	}

	if atomic.LoadInt32(&messageCount) != 10 {
		t.Errorf("Expected 10 messages processed, got %d", atomic.LoadInt32(&messageCount))
	}
}

func TestServerClient_Heartbeat(t *testing.T) {
	// Создаем сервер с heartbeat
	config := Config{
		Host:              "localhost",
		Port:              0,
		WorkerCount:       2,
		QueueSize:         100,
		EnableHeartbeat:   true,
		HeartbeatInterval: 100 * time.Millisecond,
	}

	transport := NewQUICTransport()
	server := NewServer(config, transport)

	var heartbeatCount int32
	server.OnMessage(func(ctx context.Context, client ClientInfo, message Message) error {
		if message.Type() == MessageTypeHeartbeat {
			atomic.AddInt32(&heartbeatCount, 1)
		}
		return nil
	})

	// Запускаем сервер
	ctx := context.Background()
	err := server.Start(ctx, config)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() {
		_ = server.Stop(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// Подключаем клиента
	serverAddr := transport.LocalAddr().String()
	clientTransport := NewQUICTransport()
	conn, err := clientTransport.Dial(serverAddr)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer func() {
		_ = conn.Close()
	}()

	// Ждем несколько heartbeat
	time.Sleep(350 * time.Millisecond)

	// Проверяем, что heartbeat сообщения отправляются
	// (В реальной реализации клиент должен получать heartbeat от сервера)
	// Здесь мы просто проверяем, что сервер работает
	if !server.IsRunning() {
		t.Error("Server should be running")
	}
}

// Бенчмарки для интеграционных тестов

func BenchmarkServerClient_MessageThroughput(b *testing.B) {
	// Создаем сервер
	config := Config{
		Host:        "localhost",
		Port:        0,
		WorkerCount: 4,
		QueueSize:   10000,
	}

	transport := NewQUICTransport()
	server := NewServer(config, transport)

	server.OnMessage(func(ctx context.Context, client ClientInfo, message Message) error {
		return nil
	})

	// Запускаем сервер
	ctx := context.Background()
	err := server.Start(ctx, config)
	if err != nil {
		b.Fatalf("Failed to start server: %v", err)
	}
	defer func() {
		_ = server.Stop(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// Подключаем клиента
	serverAddr := transport.LocalAddr().String()
	clientTransport := NewQUICTransport()
	conn, err := clientTransport.Dial(serverAddr)
	if err != nil {
		b.Fatalf("Failed to connect: %v", err)
	}
	defer func() {
		_ = conn.Close()
	}()

	time.Sleep(100 * time.Millisecond)

	// Подготавливаем сообщение
	message := NewMessage("benchmark", []byte("benchmark payload"))

	b.ResetTimer()
	b.ReportAllocs()

	// Отправляе�� сообщения
	for i := 0; i < b.N; i++ {
		err = conn.SendMessage(message)
		if err != nil {
			b.Fatalf("Failed to send message: %v", err)
		}
	}
}

func BenchmarkServerClient_ConcurrentClients(b *testing.B) {
	// Создаем сервер
	config := Config{
		Host:           "localhost",
		Port:           0,
		WorkerCount:    8,
		QueueSize:      10000,
		MaxConnections: 1000,
	}

	transport := NewQUICTransport()
	server := NewServer(config, transport)

	server.OnMessage(func(ctx context.Context, client ClientInfo, message Message) error {
		return nil
	})

	// Запускаем сервер
	ctx := context.Background()
	err := server.Start(ctx, config)
	if err != nil {
		b.Fatalf("Failed to start server: %v", err)
	}
	defer func() {
		_ = server.Stop(ctx)
	}()

	time.Sleep(100 * time.Millisecond)
	serverAddr := transport.LocalAddr().String()

	// Создаем пул клиентов
	clientCount := 10
	clients := make([]Connection, clientCount)
	defer func() {
		for _, conn := range clients {
			_ = conn.Close()
		}
	}()

	for i := 0; i < clientCount; i++ {
		var conn Connection
		clientTransport := NewQUICTransport()
		conn, err = clientTransport.Dial(serverAddr)
		if err != nil {
			b.Fatalf("Failed to connect client %d: %v", i, err)
		}
		clients[i] = conn
	}

	time.Sleep(200 * time.Millisecond)

	message := NewMessage("benchmark", []byte("concurrent test"))

	b.ResetTimer()
	b.ReportAllocs()

	// Отправляем сообщения параллельно
	b.RunParallel(func(pb *testing.PB) {
		clientIndex := 0
		for pb.Next() {
			client := clients[clientIndex%clientCount]
			err = client.SendMessage(message)
			if err != nil {
				b.Errorf("Failed to send message: %v", err)
			}
			clientIndex++
		}
	})
}
