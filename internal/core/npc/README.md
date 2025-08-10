# NPC пакет — Поведенческие деревья и сенсоры (документация на русском)

Этот пакет предоставляет удобный и расширяемый каркас для построения ИИ (NPC) на сервере.
Он сфокусирован на логике (без отрисовки), что подходит для серверов многопользовательских игр и симуляций.

Ключевые возможности:
- Поведенческие деревья (Behavior Trees): Sequence, Selector, Parallel, Decorators (Repeat, Timer, Probability, Inverter, Succeeder, Cooldown).
- Черная доска (Blackboard) — потокобезопасное хранилище общих данных ИИ.
- Память (Memory) — журнал решений с бинарной (gob) сериализацией.
- Сенсоры (Sensors) — модульные источники данных мира, обновляющие Blackboard.
- Agent — на каждой «тике» выполняет: сенсоры → дерево → запись решения.
- Конфигурации YAML/JSON через реестр (Registry): возможна сборка дерева без кода.

Содержание:
1. Базовые понятия
2. Быстрый старт (код)
3. Построение дерева через JSON/YAML
4. Декораторы и композиции
5. Сенсоры
6. Память и сохранение состояния
7. Расширение через Registry
8. Примеры и запуск (CLI и Web)

---

## 1. Базовые понятия

- Status: результат "тика" узла: Success, Failure, Running.
- BehaviorNode: интерфейс узла; Action, Condition, Decorator, Composite реализуют его.
- Blackboard: потокобезопасная карта ключ-значение; поддерживает пространства имён (Namespace) и сериализацию.
- Memory: хранит историю решений (DecisionRecord) и умеет сохраняться/восстанавливаться.
- Sensor: компонент, который на каждом шаге обновляет Blackboard на основе внешнего мира.
- DecisionTree: обёртка с корневым узлом и методом Tick.
- Agent: координирует сенсоры, дерево, память, события.

Основные интерфейсы смотрите в internal/core/npc/interfaces.go.

---

## 2. Быстрый старт (код)

Пример минимального агента с готовыми узлами (без мира):

```go
bb := npc.NewBlackboard()
mem := npc.NewMemory()
eb := npc.NewEventBus()

// Дерево: если ready → установить done=true
root := npc.NewSequence("Root",
    npc.ConditionFunc{Fn: func(t npc.TickContext) (bool, error) {
        v, _ := t.BB.Get("ready"); b, _ := v.(bool); return b, nil
    }},
    npc.ActionFunc{Fn: func(t npc.TickContext) (npc.Status, error) {
        t.BB.Set("done", true); return npc.StatusSuccess, nil
    }},
)

agent := npc.NewAgent(bb, mem, eb, npc.Tree{root: root}, nil)
ctx := context.Background()

bb.Set("ready", true)
_ , _ = agent.Step(ctx)
```

Смотрите также examples/npc/robot.go — более содержательный пример с миром и сенсорами.

---

## 3. Построение дерева через JSON/YAML

Пакет loader.go поддерживает схему конфигурации с Node Registry. Вы регистрируете типы узлов/сенсоров, затем собираете дерево из описания.

Пример JSON (internal/core/npc/examples_npc.json):
```json
{
  "root": "Root",
  "nodes": {
    "Root": {"type":"Selector", "children":["AttackIfPossible", "RunAway", "Idle"]},
    "AttackIfPossible": {"type":"Sequence", "children":["IsEnemyVisible", "IsStrongEnough", "Attack"]},
    "IsEnemyVisible": {"type":"Condition", "condition":"IsTrue", "params":{"key":"enemy_visible"}},
    "IsStrongEnough": {"type":"Condition", "condition":"IsTrue", "params":{"key":"strong_enough"}},
    "Attack": {"type":"Action", "action":"Noop"},
    "RunAway": {"type":"Action", "action":"SetBool", "params":{"key":"flee", "value":true}},
    "Idle": {"type":"Action", "action":"Noop"}
  },
  "sensors": [
    {"name":"Inv", "type":"InventorySensor", "params":{"key":"arrows", "out":"strong_enough", "threshold":5}}
  ]
}
```

Код сборки:
```go
r := npc.NewRegistry()
npc.RegisterBuiltins(r)
npc.RegisterSensors(r)

cfg, _ := npc.LoadJSON(bytes.NewReader(jsonBytes))
tree, sensors, _ := cfg.Build(r)
agent := npc.NewAgent(npc.NewBlackboard(), npc.NewMemory(), npc.NewEventBus(), tree, sensors)
```

Поддерживаемые типы узлов в конфиге:
- type: Sequence | Selector | Parallel | Decorator | Action | Condition
- Для Decorator используйте params.name для выбора конкретного декоратора (см. ниже).

---

## 4. Декораторы и композиции

Композиции:
- Sequence: идёт по детям до первого Failure/Running; Success когда все Success.
- Selector: идёт по детям до первого Success/Running; Failure когда все Failure.
- Parallel(policy): запускает всех детей и сводит статус по политике: RequireAllSuccess (по умолчанию) или RequireOneSuccess (policy: "any" в YAML/JSON).

Декораторы (встроенные):
- Repeat(times, stop_on_failure)
- Timer(ms)
- Probability(p)
- Inverter
- Succeeder
- Cooldown(ms, success_only)

В конфигурации указывайте:
```yaml
nodes:
  MyCooldown:
    type: Decorator
    child: SomeAction
    params:
      name: Cooldown
      ms: 200
      success_only: false
```

---

## 5. Сенсоры

Сенсор — это интерфейс:
```go
type Sensor interface {
  Name() string
  Update(ctx context.Context, bb Blackboard) error
}
```
Примеры встроенных сенсоров:
- DistanceSensor: пишет расстояние между точками (src_x, src_y, dst_x, dst_y → out).
- InventorySensor: проверяет порог и пишет bool.

Регистрация в реестре:
```go
r := npc.NewRegistry()
npc.RegisterSensors(r) // регистрирует встроенные
```

---

## 6. Память и сохранение состояния

Agent умеет сохранять и загружать состояние (Blackboard + Memory) в компактный бинарный формат:
```go
state, _ := agent.SaveState()
_ = agent.LoadState(state)
```

Memory хранит срез DecisionRecord с именем узла, статусом, длительностью и временем.

---

## 7. Расширение через Registry

Вы можете регистрировать собственные узлы/сенсоры:
```go
r.RegisterAction("MyAction", func(params map[string]any)(npc.Action, error){
    return npc.ActionFunc{Fn: func(t npc.TickContext)(npc.Status, error){
        // ...
        return npc.StatusSuccess, nil
    }}, nil
})
```
После регистрации использовать их можно в YAML/JSON конфиге или при программной сборке.

---

## 8. Примеры и запуск

1) Консольный пример робота:
- Файл: examples/npc/robot.go
- Запуск: `go run ./examples/npc/robot.go`

2) Веб‑демо с визуализацией в браузере (WebSocket, live‑тик, перезапуск):
- Сервер: examples/npc/webserver.go
- Статика: examples/npc/static
- Запуск: `go run ./examples/npc/webserver.go`
- Откройте в браузере: http://localhost:8080
- Кнопка «Перезапуск» создаёт новый случайный мир (расположение ловушек/артефактов/выхода меняется).

Примечания:
- Веб‑демо транслирует состояние по WebSocket в JSON (позиция, hp/energy, сетка и пр.).
- Примеры используют те же базовые узлы/сенсоры, что и ядро, демонстрируют адаптивное поведение через Blackboard и память.

---

Вопросы и улучшения:
- Добавьте собственные сенсоры и действия под вашу игру через Registry.
- Вынесите определение дерева в YAML/JSON, чтобы редактировать поведение без перекомпиляции.
- Возможные расширения: луч видимости (raycast) в сенсорах, A*‑поиск пути, более сложные политики Parallel, метрики и трейсинг.
