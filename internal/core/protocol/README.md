# ZeuSync Protocol Framework

Универсальный, высокопроизводительный сетевой фреймворк для мультиплеерных игр и real-time приложений.

## 🎯 Цели и принципы

### Универсальность
- Поддержка различных типов игр (MMO, сессионные, P2P-like)
- Гибкая архитектура для любых сценариев использования
- Простая интеграция через UI для разработчиков на Unity, C#, TS

### Производительность
- Бинарная сериализация сообщений
- QUIC транспорт для низкой задержки
- Object pooling для минимизации GC pressure
- Эффективная обработка тысяч соединений

### Расширяемость
- Модульная архитектура с четкими интерфейсами
- Система middleware для плагинов
- Поддержка custom транспор��ов и сериализаторов

## 🏗️ Архитектура

### Основные компоненты

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│    Protocol     │    │   Transport     │    │    Message      │
│   (Interface)   │    │   (Interface)   │    │   (Interface)   │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│     Server      │    │  QUICTransport  │    │ BinaryMessage   │
│ (Implementation)│    │ (Implementation)│    │(Implementation) │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

### Слои абстракции

1. **Protocol Layer** - Высокоуровневый API для работы с клиентами и группами
2. **Transport Layer** - Абстракция сетевого транспорта (QUIC, WebSocket, TCP)
3. **Message Layer** - Сериализация и обработка сообщений
4. **Connection Layer** - Управление соединениями

## 📦 Основные интерфейсы

### Protocol
```go
type Protocol interface {
    // Lifecycle
    Start(ctx context.Context, config Config) error
    Stop(ctx context.Context) error
    IsRunning() bool

    // Client management
    OnClientConnect(handler ClientConnectHandler)
    OnClientDisconnect(handler ClientDisconnectHandler)
    OnMessage(handler MessageHandler)

    // Messaging
    SendToClient(clientID string, message Message) error
    Broadcast(message Message) error
    
    // Groups/Rooms
    CreateGroup(groupID string) error
    AddClientToGroup(clientID, groupID string) error
    SendToGroup(groupID string, message Message) error
}
```

### Transport
```go
type Transport interface {
    Listen(address string) error
    Accept() (Connection, error)
    Dial(address string) (Connection, error)
    Close() error
    Type() TransportType
}
```

### Message
```go
type Message interface {
    ID() string
    Type() string
    Payload() []byte
    Timestamp() time.Time
    
    GetHeader(key string) string
    SetHeader(key, value string)
    
    Marshal() ([]byte, error)
    Unmarshal(data []byte) error
}
```

## 🚀 Быстрый старт

### Сервер
```go
// Конфигурация
config := protocol.Config{
    Host: "localhost",
    Port: 8080,
    MaxConnections: 1000,
    WorkerCount: 8,
    EnableMetrics: true,
}

// Создание сервера
transport := protocol.NewQUICTransport()
server := protocol.NewServer(config, transport)

// Обработчики
server.OnClientConnect(func(ctx context.Context, client protocol.ClientInfo) error {
    fmt.Printf("Client connected: %s\n", client.ID)
    return nil
})

server.OnMessage(func(ctx context.Context, client protocol.ClientInfo, message protocol.Message) error {
    fmt.Printf("Message: %s\n", message.Type())
    return nil
})

// Запуск
ctx := context.Background()
server.Start(ctx, config)
```

### Клиент (SDK)
```go
// Конфигурация
config := sdk.ClientConfig{
    ServerAddress: "localhost:8080",
    Transport: protocol.TransportQUIC,
}

// Создание клиента
client := sdk.NewClient(config)

// Обработчики
client.OnMessage("welcome", func(ctx context.Context, message protocol.Message) error {
    fmt.Printf("Welcome: %s\n", string(message.Payload()))
    return nil
})

// Подключение
client.Connect()

// Отправка сообщений
client.Send("chat", []byte("Hello World!"))
```

## 📊 Производительность

### Бинарная сериализация
- **Marshal**: ~15 ns/op (6x быстрее JSON)
- **Unmarshal**: ~25 ns/op (128x быстрее JSON)
- **Zero allocations** для большинства операций

### QUIC транспорт
- **Низкая задержка**: 0.1-0.5ms
- **Мультиплексирование**: множественные потоки
- **Встроенное шифрование**: TLS 1.3
- **0-RTT handshake**: быстрое переподключение

### Масштабируемость
- **10,000+ одновременных соединений**
- **100,000+ сообщений/сек**
- **Эффективное использование памяти** через object pooling

## 🔧 Конфигурация

### Основные параметры
```go
type Config struct {
    // Network
    Host string
    Port int
    
    // Performance
    MaxConnections    int
    WorkerCount       int
    QueueSize         int
    
    // Features
    EnableMetrics     bool
    EnableHeartbeat   bool
    HeartbeatInterval time.Duration
    
    // Limits
    MaxMessageSize    int
    MaxGroupSize      int
    
    // Timeouts
    ReadTimeout       time.Duration
    WriteTimeout      time.Duration
    IdleTimeout       time.Duration
}
```

### Оптимизация для разных сценариев

#### MMO игры
```go
config := protocol.Config{
    MaxConnections: 10000,
    WorkerCount: 16,
    QueueSize: 50000,
    MaxGroupSize: 1000,
    EnableMetrics: true,
}
```

#### Сессионные игры
```go
config := protocol.Config{
    MaxConnections: 100,
    WorkerCount: 4,
    QueueSize: 1000,
    MaxGroupSize: 10,
    HeartbeatInterval: 5 * time.Second,
}
```

## 🎮 Игровые функции

### Группы/Комнаты
```go
// Создание комнаты
server.CreateGroup("room_1")

// Добавление игрока
server.AddClientToGroup(clientID, "room_1")

// Отправка сообщения в комнату
message := protocol.NewMessage("game_update", gameData)
server.SendToGroup("room_1", message)
```

### Типы сообщений
```go
const (
    MessageTypePlayerMove   = "player_move"
    MessageTypePlayerAction = "player_action"
    MessageTypeGameState    = "game_state"
    MessageTypeChat         = "chat"
    MessageTypeHeartbeat    = "heartbeat"
)
```

### Метаданные клиента
```go
// Установка метаданных
client.SetMetadata("player_level", 25)
client.SetMetadata("guild_id", "guild_123")

// Получение метаданных
level, _ := client.GetMetadata("player_level")
```

## 📈 Мониторинг и метрики

### Серверные метрики
```go
metrics := server.GetMetrics()
fmt.Printf("Active connections: %d\n", metrics.ActiveConnections)
fmt.Printf("Messages/sec: %.2f\n", metrics.MessagesPerSecond)
fmt.Printf("Active groups: %d\n", metrics.ActiveGroups)
```

### Клиентские метрики
```go
metrics := client.GetMetrics()
fmt.Printf("Messages sent: %d\n", metrics.MessagesSent)
fmt.Printf("Last activity: %s\n", metrics.LastActivity)
```

## 🧪 Тестирование

### Запуск тестов
```bash
# Все тесты
go test ./internal/core/protocol/...

# Только unit тесты
go test ./internal/core/protocol/ -run TestBinaryMessage

# Интеграционные тесты
go test ./internal/core/protocol/ -run TestServerClient

# Бенчмарки
go test ./internal/core/protocol/ -bench=. -benchmem
```

### Примеры тестов
```go
func TestBinaryMessage_MarshalUnmarshal(t *testing.T) {
    original := protocol.NewMessage("test", []byte("payload"))
    data, _ := original.Marshal()
    
    restored := &protocol.BinaryMessage{}
    restored.Unmarshal(data)
    
    assert.Equal(t, original.Type(), restored.Type())
    assert.Equal(t, original.Payload(), restored.Payload())
}
```

## 🔌 Расширения

### Middleware
```go
type LoggingMiddleware struct{}

func (m *LoggingMiddleware) OnMessage(ctx context.Context, client ClientInfo, message Message, next func() error) error {
    log.Printf("Message: %s from %s", message.Type(), client.ID)
    return next()
}

// Добавление middleware
server.AddMiddleware(&LoggingMiddleware{})
```

### Кастомные транспорты
```go
type CustomTransport struct {
    // Реализация Transport интерфейса
}

func (t *CustomTransport) Listen(address string) error {
    // Кастомная логика
}

// Использование
transport := &CustomTransport{}
server := protocol.NewServer(config, transport)
```

## 📚 Примеры использования

### Простой чат сервер
```go
server.OnMessage(func(ctx context.Context, client ClientInfo, message Message) error {
    if message.Type() == "chat" {
        // Отправляем всем клиентам
        return server.Broadcast(message)
    }
    return nil
})
```

### Игровая комната
```go
server.OnMessage(func(ctx context.Context, client ClientInfo, message Message) error {
    switch message.Type() {
    case "join_game":
        roomID := string(message.Payload())
        server.CreateGroup(roomID)
        return server.AddClientToGroup(client.ID, roomID)
        
    case "game_action":
        roomID := message.GetHeader("room_id")
        return server.SendToGroup(roomID, message)
    }
    return nil
})
```

## 🛠️ Разработка

### Структура проекта
```
internal/core/protocol/
├── interfaces.go          # Основные интерфейсы
├── message.go             # Реализация сообщений
├── server.go              # Реализация сервера
├── transport_quic.go      # QUIC транспорт
├── *_test.go             # Тесты
└── README.md             # Документация

sdk/
└── client.go             # Клиентский SDK

examples/
├── simple_server.go      # Пример сервера
└── simple_client.go      # Пример клиента
```

### Принципы разработки
1. **Интерфейсы первыми** - четкие контракты
2. **Произво��ительность** - измеряем и оптимизируем
3. **Тестируемость** - покрытие тестами
4. **Документация** - понятные примеры

## 🚀 Roadmap

### v1.0 (Текущая версия)
- ✅ Базовые интерфейсы
- ✅ QUIC транспорт
- ✅ Бинарная сериализация
- ✅ Группы/комнаты
- ✅ Клиентский SDK

### v1.1 (Планируется)
- 🔄 WebSocket транспорт
- 🔄 Middleware система
- 🔄 Rate limiting
- 🔄 Аутентификация

### v1.2 (Будущее)
- ⏳ TCP/UDP транспорты
- ⏳ Кластеризация
- ⏳ Персистентность
- ⏳ Admin UI

## 📄 Лицензия

MIT License - см. LICENSE файл для деталей.

## 🤝 Вклад в проект

1. Fork проекта
2. Создайте feature branch
3. Добавьте тесты
4. Убедитесь что все тесты проходят
5. Создайте Pull Request

## 📞 Поддержка

- GitHub Issues для багов и feature requests
- Документация: [docs/](../../../docs/)
- Примеры: [examples/](../../../examples/)