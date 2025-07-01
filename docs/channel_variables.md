# Channel-Based Synchronized Variables

ZeuSync поддерживает несколько типов channel-based синхронизированных переменных, которые используют Go каналы для координации доступа к данным.

## Типы Channel Variables

### 1. BufferedChannelVar
Использует буферизованные Go каналы для синхронизации.

**Особенности:**
- Поддерживает неблокирующие операции Send/Receive
- Настраиваемый размер буфера
- Подходит для producer-consumer паттернов

**Пример использования:**
```go
// Создание с буфером размером 10
ch := vars.NewBufferedChannelVar[string](10, "initial")
wrapper := wrappers.NewChannelWrapper(ch)

// Отправка значения (неблокирующая)
success := wrapper.Send("message")

// Получение значения (неблокирующая)
value, ok := wrapper.Receive()
```

### 2. UnbufferedChannelVar
Использует небуферизованные Go каналы для синхронизации.

**Особенности:**
- Синхронная передача данных
- Отправитель блокируется до получения
- Подходит для строгой синхронизации

**Пример использования:**
```go
ch := vars.NewUnbufferedChannelVar[int](42)
wrapper := wrappers.NewChannelWrapper(ch)

// Блокирующая отправка
wrapper.SendBlocking(100)

// Блокирующее получение
value := wrapper.ReceiveBlocking()
```

### 3. PriorityChannelVar
Реализует очередь с приоритетами на основе heap.

**Особенности:**
- Сообщения обрабатываются по приоритету
- Высший приоритет обрабатывается первым
- Настраиваемый максимальный размер очереди

**Пример использования:**
```go
ch := vars.NewPriorityChannelVar[string](100, "")
wrapper := wrappers.NewChannelWrapper(ch)

// Отправка с п��иоритетом
ch.SendWithPriority("Low priority", 1)
ch.SendWithPriority("High priority", 10)
ch.SendWithPriority("Medium priority", 5)

// Получение в порядке приоритета: High -> Medium -> Low
for i := 0; i < 3; i++ {
    value, ok := wrapper.Receive()
    fmt.Printf("Received: %s\n", value)
}
```

### 4. BroadcastChannelVar
Реализует broadcast канал для отправки сообщений множеству подписчиков.

**Особенности:**
- Одно сообщение доставляется всем подписчикам
- Динамическое управление подписчиками
- Настраиваемый размер буфера для подписчиков

**Пример использования:**
```go
ch := vars.NewBroadcastChannelVar[int](5, 0)
wrapper := wrappers.NewChannelWrapper(ch)

// Подписка на сообщения
sub1 := ch.Subscribe()
sub2 := ch.Subscribe()

// Отправка broadcast сообщения
wrapper.Send(42)

// Все подписчики получат сообщение
go func() {
    value := <-sub1
    fmt.Printf("Sub1 received: %d\n", value)
}()

go func() {
    value := <-sub2
    fmt.Printf("Sub2 received: %d\n", value)
}()
```

## Интерфейс ChannelRoot

Все channel-based переменные реализуют интерфейс `ChannelRoot[T]`:

```go
type ChannelRoot[T any] interface {
    Root[T]
    
    // Неблокирующие операции
    Send(value T) bool
    Receive() (T, bool)
    
    // Блокирующие операции
    SendBlocking(value T)
    ReceiveBlocking() T
    
    // Управление буфером
    BufferSize() int
    SetBufferSize(size int) error
}
```

## Wrapper и Core интеграция

Channel переменные интегрируются с системой через `ChannelWrapper`:

```go
// Создание wrapper'а
wrapper := wrappers.NewChannelWrapper(channelRoot)

// Wrapper предоставляет полный Core интерфейс
wrapper.OnChange(func(old, new T) {
    fmt.Printf("Value changed: %v -> %v\n", old, new)
})

// Метрики
metrics := wrapper.Metrics()
fmt.Printf("Reads: %d, Writes: %d\n", metrics.Reads, metrics.Writes)
```

## Использование через Factory

```go
config := types.Configuration{
    ChannelImpl: types.BufferedChannel,
    BufferSize:  10,
}

factory := interfaces.NewDefaultFactory[string](config)
channelRoot := factory.CreateChannelRoot("initial", 10)
wrapper := wrappers.NewChannelWrapper(channelRoot)
```

## Выбор типа Channel Variable

| Тип | Использование | Преимущества | Недостатки |
|-----|---------------|--------------|------------|
| BufferedChannel | Producer-consumer, async processing | Высокая производительность, неблокирующие операции | Может потреблять больше памяти |
| UnbufferedChannel | Строгая синхронизация | Точная синхронизация, низкое потребление памяти | Блокирующие операции |
| PriorityChannel | Обработка по приоритетам | Гибкое управление порядком обработки | Дополнительные накладные расходы |
| BroadcastChannel | Pub-Sub паттерны | Эффективная доставка множеству получателей | Сложность управления подписчиками |

## Лучшие практики

1. **Выбор размера буфера**: Учитывайте паттерны доступа и требования к памяти
2. **Обработка ошибок**: Всегда проверяйте возвращаемые значения неблокирующих операций
3. **Управление жизненным циклом**: Не забывайте закрывать каналы и отписываться от broadcast каналов
4. **Мониторинг**: Используйте метрики для отслеживания производительности

## Примеры паттернов

### Producer-Consumer
```go
ch := vars.NewBufferedChannelVar[Task](100, Task{})
wrapper := wrappers.NewChannelWrapper(ch)

// Producer
go func() {
    for task := range taskSource {
        wrapper.Send(task)
    }
}()

// Consumer
go func() {
    for {
        if task, ok := wrapper.Receive(); ok {
            processTask(task)
        }
    }
}()
```

### Priority Queue
```go
pq := vars.NewPriorityChannelVar[Job](1000, Job{})

// Добавление задач с разными приоритетами
pq.SendWithPriority(Job{Type: "urgent"}, 10)
pq.SendWithPriority(Job{Type: "normal"}, 5)
pq.SendWithPriority(Job{Type: "low"}, 1)

// Обработка в порядке приоритета
for {
    if job, ok := pq.Receive(); ok {
        processJob(job)
    }
}
```

### Event Broadcasting
```go
broadcaster := vars.NewBroadcastChannelVar[Event](10, Event{})

// Подписчики
sub1 := broadcaster.Subscribe()
sub2 := broadcaster.Subscribe()

// Обработчики событий
go handleEvents(sub1, "Handler1")
go handleEvents(sub2, "Handler2")

// Отправка событий
broadcaster.Send(Event{Type: "user_login", Data: userData})
```