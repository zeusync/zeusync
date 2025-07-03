# 🚀 ZeuSync Protocol Quick Start

Быстрое руководство по запуску и тестированию протоколов ZeuSync.

## ⚡ Быстрый старт (5 минут)

### 1. Установка зависимостей
```bash
cd examples
make deps
```

### 2. Тест WebSocket
```bash
# Терминал 1: Запуск сервера
make ws-server

# Терминал 2: Запуск клиента
make ws-client
```

### 3. Тест QUIC
```bash
# Терминал 1: Запуск сервера
make quic-server

# Терминал 2: Запуск клиента
make quic-client
```

### 4. Сравнение производительности
```bash
make benchmark
```

## 📋 Что вы увидите

### WebSocket Demo
- ✅ Подключение к серверу
- ✅ Отправка приветствия
- ✅ Эхо сообщения
- ✅ Работа с группами
- ✅ Broadcast сообщения
- ✅ Метрики сервера
- ✅ Сжатие больших сообщений
- ✅ Приоритеты и QoS

### QUIC Demo
- ✅ Быстрое подключение
- ✅ Мультиплексированные потоки
- ✅ Файловый обмен
- ✅ Чат-комнаты
- ✅ Стресс-тестирование
- ✅ Большие данные
- ✅ Низкая задержка

### Benchmark Results
- 📊 Сравнение производительности
- 📈 Метрики задержки
- 🏆 Рекомендации по использованию

## 🔧 Основные команды

| Команда | Описание |
|---------|----------|
| `make help` | Показать все доступные команды |
| `make ws-server` | Запустить WebSocket сервер |
| `make ws-client` | Запустить WebSocket клиент |
| `make quic-server` | Запустить QUIC сервер |
| `make quic-client` | Запустить QUIC клиент |
| `make benchmark` | Сравнить производительность |
| `make test` | Запустить тесты |
| `make dev-ws` | Запустить WebSocket сервер и клиент |
| `make dev-quic` | Запустить QUIC сервер и клиент |

## 🌐 Endpoints

### WebSocket Server (порт 8080)
- `ws://localhost:8080/ws` - WebSocket соединение
- `http://localhost:8080/health` - Проверка здоровья
- `http://localhost:8080/metrics` - Метрики сервера

### QUIC Server (порт 9090)
- `localhost:9090` - QUIC соединение

## 📝 Типы сообщений

### WebSocket
- `greeting` - Приветствие
- `echo` - Эхо сообщения
- `join_group` - Присоединение к группе
- `group_message` - Сообщение в группу
- `get_metrics` - Получение метрик
- `get_clients` - Список клиентов

### QUIC
- `fast_response` - Быстрый ответ
- `stress_test` - Стресс-тест
- `large_data` - Большие данные
- `chat_room` - Чат-комната
- `file_transfer` - Файловый обмен
- `quic_metrics` - QUIC метрики

## 🎯 Примеры использования

### Создание сообщения
```go
message := protocol.NewMessage("greeting", map[string]interface{}{
    "name": "Client",
    "message": "Hello, Server!",
})
```

### Установка приоритета
```go
message.SetPriority(protocol.PriorityHigh)
message.SetQoS(protocol.QoSExactlyOnce)
```

### Сжатие
```go
message.Compress()
```

### Регистрация обработчика
```go
protocol.RegisterHandler("my_type", func(ctx context.Context, client protocol.ClientInfo, message protocol.IMessage) error {
    // Обработка сообщения
    response := message.CreateResponse(responseData)
    return protocol.Send(client.ID, response)
})
```

## 🔍 Troubleshooting

### Порты заняты
```bash
# Проверить занятые порты
netstat -an | grep :8080
netstat -an | grep :9090

# Или изменить порты в конфигурации
```

### Проблемы с QUIC
- Убедитесь что UDP порт 9090 открыт
- Некоторые корпоративные сети блокируют QUIC
- Проверьте firewall настройки

### Ошибки компиляции
```bash
# Обновить зависимости
go mod tidy
go mod download

# Очистить кэш
go clean -cache
```

## 📚 Дополнительная информа��ия

- [Полная документация](README.md)
- [Архитектура протоколов](../internal/core/protocol/)
- [Тесты](../tests/)

## 🎉 Готово!

Теперь вы можете:
1. ✅ Запускать WebSocket и QUIC серверы
2. ✅ Тестировать различные функции протоколов
3. ✅ Сравнивать производительность
4. ✅ Интегрировать в свои проекты

Удачного использования ZeuSync! 🚀