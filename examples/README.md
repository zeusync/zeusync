# ZeuSync Protocol Examples

Этот каталог содержит примеры использования протоколов ZeuSync, включая WebSocket и QUIC реализации.

## Обзор архитектуры

ZeuSync предоставляет унифицированный интерфейс для различных сетевых протоколов:

### Основные компоненты

1. **Protocol Interface** (`internal/core/protocol/protocol.go`)
   - Единый интерфейс для всех протоколов
   - Поддержка обработчиков сообщений
   - Управление клиентами и группами
   - Middleware поддержка
   - Метрики и мониторинг

2. **Message System** (`internal/core/protocol/message.go`)
   - Типизированные сообщения с метаданными
   - Сжатие и сериализация
   - Приоритеты и QoS
   - Маршрутизация и ответы

3. **WebSocket Implementation** (`internal/core/protocol/websocket/`)
   - Полная реализация WebSocket протокола
   - Поддержка групп и broadcast
   - Ping/Pong для keep-alive
   - Middleware поддержка

4. **QUIC Implementation** (`internal/core/protocol/quic/`)
   - Современный QUIC протокол
   - Мультиплексирование потоков
   - Встроенное шифрование (TLS 1.3)
   - Низкая задержка

## Примеры

### WebSocket

#### Сервер (`websocket_server.go`)
```bash
go run examples/websocket_server.go
```

**Возможности:**
- HTTP сервер на порту 8080
- WebSocket endpoint: `ws://localhost:8080/ws`
- Health check: `http://localhost:8080/health`
- Metrics: `http://localhost:8080/metrics`

**Поддерживаемые типы сообщений:**
- `greeting` - Приветствие
- `echo` - Эхо сообщения
- `join_group` - Присоединение к группе
- `group_message` - Сообщение в группу
- `get_metrics` - Получение метрик
- `get_clients` - Список клиентов

#### Клиент (`websocket_client.go`)
```bash
go run examples/websocket_client.go
```

**Демонстрирует:**
- Подключение к WebSocket серверу
- Отправка различных типо�� сообщений
- Работа с группами
- Тестирование сжатия
- Приоритеты и QoS

### QUIC

#### Сервер (`quic_server.go`)
```bash
go run examples/quic_server.go
```

**Возможности:**
- QUIC сервер на порту 9090
- Самоподписанный TLS сертификат
- Мультиплексирование потоков
- Улучшенная производительность

**Поддерживаемые типы сообщений:**
- `fast_response` - Быстрый ответ
- `stress_test` - Стресс-тест
- `large_data` - Большие данные
- `chat_room` - Чат-комната
- `file_transfer` - Файловый обмен
- `quic_metrics` - QUIC метрики

#### Клиент (`quic_client.go`)
```bash
go run examples/quic_client.go
```

**Демонстрирует:**
- Подключение к QUIC серверу
- Тестирование скорости
- Передача больших данных
- Файловый обмен
- Мультиплексирование

## Запуск примеров

### Предварительные требования

1. Go 1.21+
2. Зависимости проекта:
```bash
go mod tidy
```

### WebSocket пример

1. Запустите сервер:
```bash
cd examples
go run websocket_server.go
```

2. В другом терминале запустите клиент:
```bash
cd examples
go run websocket_client.go
```

### QUIC пример

1. Запустите QUIC сервер:
```bash
cd examples
go run quic_server.go
```

2. В другом терминале запустите QUIC клиент:
```bash
cd examples
go run quic_client.go
```

## Основные концепции

### 1. Создание протокола

```go
// WebSocket
config := protocol.Config{
    Host: "localhost",
    Port: 8080,
    // ... другие настройки
}

wsProtocol := websocket.NewWebSocketProtocol(config, logger)

// QUIC
quicProtocol := quic.NewQuicProtocol(config, logger)
```

### 2. Регистрация обработчиков

```go
protocol.RegisterHandler("message_type", func(ctx context.Context, client protocol.ClientInfo, message protocol.IMessage) error {
    // Обработка сообщения
    response := message.CreateResponse(responseData)
    return protocol.Send(client.ID, response)
})
```

### 3. Создание сообщений

```go
message := protocol.NewMessage("greeting", map[string]interface{}{
    "text": "Hello, World!",
    "timestamp": time.Now(),
})

// Установка приоритета
message.SetPriority(protocol.PriorityHigh)

// Установка QoS
message.SetQoS(protocol.QoSExactlyOnce)

// Сжатие
message.Compress()
```

### 4. Работа с группами

```go
// Создание группы
protocol.CreateGroup("chat_room")

// Добавление клиента в группу
protocol.JoinGroup(clientID, "chat_room")

// Отправка сообщения группе
protocol.SendToGroup("chat_room", message)
```

### 5. Middleware

```go
type CustomMiddleware struct{}

func (m *CustomMiddleware) BeforeHandle(ctx context.Context, client protocol.ClientInfo, message protocol.IMessage) error {
    // Логика до обработки
    return nil
}

func (m *CustomMiddleware) AfterHandle(ctx context.Context, client protocol.ClientInfo, message protocol.IMessage, response protocol.IMessage, err error) error {
    // Логика после обработки
    return nil
}

// Добавление middleware
protocol.AddMiddleware(&CustomMiddleware{})
```

## Сравнение протоколов

| Особенность | WebSocket | QUIC |
|-------------|-----------|------|
| Транспорт | TCP | UDP |
| Шифрование | Опционально (WSS) | Встроенное (TLS 1.3) |
| Мультиплексирование | Нет | Да |
| Задержка | Средняя | Низкая |
| Надежность | Высокая | Высокая |
| Поддержка браузеров | Отличная | Растущая |
| 0-RTT | Нет | Да |
| Миграция соединений | Нет | Да |

## Метрики и мониторинг

Оба протокола предоставляют метрики:

```go
metrics := protocol.GetMetrics()
fmt.Printf("Active connections: %d\n", metrics.ActiveConnections)
fmt.Printf("Messages per second: %.2f\n", metrics.MessagesPerSecond)
```

## Лучшие практики

1. **Обработка ошибок**: Всегда проверяйте ошибки при отправке сообщений
2. **Graceful shutdown**: Используйте контексты для корректного завершения
3. **Middleware**: Используйте middleware для логирования, аутентификации, метрик
4. **Группы**: Используйте группы для эффективного broadcast
5. **Сжатие**: Включайте сжатие для больших сообщений
6. **QoS**: Выбирайте подходящий уровень QoS для ваших нужд

## Troubleshooting

### WebSocket
- Проверьте CORS настройки для браузерных клиентов
- Убедитесь что порт 8080 свободен
- Проверьте firewall настройки

### QUIC
- QUIC требует UDP порт 9090
- Некоторые корпоративные сети блокируют QUIC
- Убедитесь что TLS 1.3 поддерживается

## Дополнительные ресурсы

- [WebSocket RFC 6455](https://tools.ietf.org/html/rfc6455)
- [QUIC RFC 9000](https://tools.ietf.org/html/rfc9000)
- [ZeuSync Documentation](../docs/)