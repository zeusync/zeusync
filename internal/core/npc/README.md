# AI Agent Framework

A flexible, modular framework for building complex AI agents in Go. This framework provides a complete solution for creating intelligent agents that can adapt to various scenarios including games, business logic, simulations, and more.

## Features

- **Behavior Trees**: Dynamic, configurable behavior trees with support for sequences, selectors, parallel execution, decorators, and custom nodes
- **Blackboard System**: Centralized data storage for agent state and world information
- **Sensor System**: Modular perception components for gathering environmental data
- **Memory System**: Persistent memory with event storage, recall, and retention policies
- **Event System**: Reactive event handling with priority-based processing
- **Configuration-Driven**: JSON/YAML configuration support for non-programmers
- **State Persistence**: Save and restore agent states
- **Extensible Architecture**: Plugin-based system for custom behaviors, sensors, and handlers

## Quick Start

### Creating a Simple Agent

```go
package main

import (
    "context"
    "fmt"
    "time"
    
    "your-project/internal/core/npc"
)

func main() {
    // Create agent manager
    manager := npc.NewAIAgentManager()
    
    // Create a simple test agent
    config := npc.CreateSimpleTestConfig("agent_1", "Test Agent")
    agent, err := manager.CreateAgent(config)
    if err != nil {
        panic(err)
    }
    
    // Update the agent
    ctx := context.Background()
    for i := 0; i < 10; i++ {
        err := agent.Update(ctx, time.Millisecond*100)
        if err != nil {
            fmt.Printf("Update error: %v\n", err)
        }
        time.Sleep(time.Millisecond * 100)
    }
}
```

### Creating a Game NPC

```go
// Create a game NPC with combat and patrol behavior
config := npc.CreateGameNPCConfig("guard_1", "Castle Guard")
agent, err := manager.CreateAgent(config)

// Set initial position and health
bb := agent.GetBlackboard()
bb.Set("position", npc.Position{X: 10, Y: 5, Z: 0})
bb.Set("health", 100.0)
bb.Set("max_health", 100.0)

// Add damage event handler
damageHandler := npc.NewDamageEventHandler("damage_handler", "health", 8)
agent.AddEventHandler(damageHandler)
```

### Creating a Trading Agent

```go
// Create a trading agent that responds to market conditions
config := npc.CreateTradingAgentConfig("trader_1", "Market Trader")
agent, err := manager.CreateAgent(config)

// Set market conditions
bb := agent.GetBlackboard()
bb.Set("market_demand", 0.8) // High demand
bb.Set("inventory", map[string]int{
    "goods": 50,
    "materials": 20,
})
```

## Core Components

### Blackboard

The blackboard is a centralized data storage system where all agent modules can read and write data.

```go
bb := npc.NewBlackboard()

// Store different types of data
bb.Set("health", 100.0)
bb.Set("position", npc.Position{X: 0, Y: 0, Z: 0})
bb.Set("inventory", map[string]int{"gold": 50})

// Retrieve data with type safety
health, exists := bb.GetFloat("health")
position, exists := bb.Get("position")
hasGold := bb.Has("inventory")

// Export/import for persistence
data, err := bb.ToJSON()
err = bb.FromJSON(data)
```

### Behavior Trees

Behavior trees define the decision-making logic of agents through a hierarchical structure of nodes.

#### Node Types

- **Composite Nodes**:
  - `Sequence`: Executes children in order, fails on first failure
  - `Selector`: Executes children until one succeeds
  - `Parallel`: Executes all children simultaneously
  - `RandomSelector`: Randomly selects one child to execute

- **Decorator Nodes**:
  - `Inverter`: Inverts the result of its child
  - `Repeater`: Repeats its child a specified number of times

- **Leaf Nodes**:
  - `Action`: Performs an action
  - `Condition`: Checks a condition
  - `Wait`: Waits for a specified duration
  - `Log`: Logs a message

#### Configuration Example

```json
{
  "name": "Guard Behavior",
  "root": {
    "name": "MainSelector",
    "type": "Selector",
    "enabled": true,
    "children": [
      {
        "name": "CombatSequence",
        "type": "Sequence",
        "enabled": true,
        "children": [
          {
            "name": "IsEnemyNear",
            "type": "Condition",
            "enabled": true,
            "parameters": {
              "condition_type": "compare",
              "key": "distance_to_enemy",
              "operator": "<=",
              "value": 5.0
            }
          },
          {
            "name": "AttackEnemy",
            "type": "Action",
            "enabled": true,
            "parameters": {
              "action_type": "custom",
              "message": "Attacking enemy!"
            }
          }
        ]
      },
      {
        "name": "PatrolAction",
        "type": "Action",
        "enabled": true,
        "parameters": {
          "action_type": "set_blackboard",
          "key": "state",
          "value": "patrolling"
        }
      }
    ]
  }
}
```

### Sensors

Sensors gather information from the environment and update the blackboard.

#### Built-in Sensors

- **DistanceSensor**: Measures distance to targets
- **HealthSensor**: Monitors agent health status
- **InventorySensor**: Tracks inventory items
- **TimerSensor**: Manages time-based conditions
- **EnvironmentSensor**: Monitors environmental conditions
- **ProximitySensor**: Detects nearby entities

#### Custom Sensor Example

```go
type CustomSensor struct {
    *npc.BaseSensor
    // Custom fields
}

func (cs *CustomSensor) Update(ctx *npc.ExecutionContext) error {
    // Custom sensor logic
    ctx.Blackboard.Set("custom_data", "sensor_value")
    return nil
}
```

### Memory System

The memory system stores and recalls experiences and events.

```go
memory := npc.NewBasicMemory()

// Store simple data
memory.Store("important_location", npc.Position{X: 100, Y: 50, Z: 0})

// Remember events
event := npc.MemoryEvent{
    Type: "combat_encounter",
    Timestamp: time.Now(),
    Data: map[string]interface{}{
        "enemy_type": "orc",
        "location": npc.Position{X: 10, Y: 20, Z: 0},
    },
    Importance: 0.8,
    Tags: []string{"combat", "enemy"},
}
memory.Remember(event)

// Recall events
criteria := npc.MemoryCriteria{
    Type: "combat_encounter",
    Tags: []string{"combat"},
    Limit: 10,
}
events, err := memory.Recall(criteria)
```

### Event System

The event system enables reactive behavior through event handling.

```go
// Create events
damageEvent := npc.CreateDamageEvent("enemy_1", "agent_1", 25.0)
healEvent := npc.CreateHealEvent("potion", "agent_1", 50.0)
itemEvent := npc.CreateItemEvent("chest", "agent_1", "pickup", "sword", 1)

// Handle events
agent.HandleEvent(damageEvent)

// Custom event handlers
handler := npc.NewBasicEventHandler("custom_handler", "custom_event", 5, 
    func(ctx *npc.ExecutionContext, event npc.Event) error {
        // Custom handling logic
        return nil
    })
agent.AddEventHandler(handler)
```

## Configuration

### Agent Configuration

```yaml
id: "guard_1"
name: "Castle Guard"
description: "A guard that patrols and fights intruders"
active: true
update_rate: "100ms"

behavior_tree:
  name: "Guard Behavior"
  root:
    name: "MainSelector"
    type: "Selector"
    enabled: true
    children:
      - name: "CombatBehavior"
        type: "Sequence"
        enabled: true
        # ... behavior tree definition

sensors:
  - name: "ProximitySensor"
    type: "Proximity"
    enabled: true
    update_interval: "100ms"
    parameters:
      position_key: "position"
      entities_key: "world_entities"
      output_key: "proximity"
      max_range: 10.0
      entity_types: ["player", "intruder"]

event_handlers:
  - name: "DamageHandler"
    type: "damage"
    event_type: "damage"
    priority: 8
    enabled: true

memory:
  type: "basic"
  max_events: 1000
  retention_time: "24h"

initial_data:
  health: 100.0
  max_health: 100.0
  position:
    x: 0
    y: 0
    z: 0
  patrol_waypoints:
    - {x: 0, y: 0, z: 0}
    - {x: 10, y: 0, z: 0}
    - {x: 10, y: 10, z: 0}
    - {x: 0, y: 10, z: 0}
```

## Advanced Usage

### Custom Node Types

```go
// Register custom node factory
type CustomActionFactory struct{}

func (f *CustomActionFactory) CreateNode(config *npc.NodeConfig) (npc.Node, error) {
    return &CustomActionNode{
        BaseNode: npc.NewBaseNode(config.Name, "CustomAction"),
        // Custom parameters
    }, nil
}

func (f *CustomActionFactory) GetNodeType() string {
    return "CustomAction"
}

// Register with builder
builder := npc.NewBehaviorTreeBuilder()
builder.RegisterNodeType("CustomAction", &CustomActionFactory{})
```

### Custom Sensors

```go
// Implement sensor factory
type CustomSensorFactory struct{}

func (f *CustomSensorFactory) CreateSensor(config *npc.SensorConfig) (npc.Sensor, error) {
    return &CustomSensor{
        BaseSensor: npc.NewBaseSensor(config.Name, "Custom", config.UpdateInterval),
        // Custom parameters from config.Parameters
    }, nil
}

// Register with manager
manager.RegisterSensorFactory("Custom", &CustomSensorFactory{})
```

### State Persistence

```go
// Save agent state
data, err := agent.Save()
if err != nil {
    log.Printf("Failed to save agent: %v", err)
}

// Save to file
err = ioutil.WriteFile("agent_state.json", data, 0644)

// Load agent state
data, err := ioutil.ReadFile("agent_state.json")
if err != nil {
    log.Printf("Failed to read agent state: %v", err)
}

err = agent.Load(data)
if err != nil {
    log.Printf("Failed to load agent: %v", err)
}
```

### Manager Auto-Update

```go
// Start automatic updates
ctx := context.Background()
manager.StartAutoUpdate(ctx)

// Stop automatic updates
manager.StopAutoUpdate()
```

## Examples

The framework includes several example configurations:

- **Game NPC**: Combat and patrol behavior for game characters
- **Trading Agent**: Market-responsive trading logic
- **Patrol Guard**: Security guard with investigation behavior
- **Simple Test**: Basic testing configuration
- **Repeating Behavior**: Demonstration of repeater nodes
- **Parallel Behavior**: Demonstration of parallel execution

## Testing

Run the comprehensive test suite:

```bash
go test ./internal/core/npc -v
```

Run benchmarks:

```bash
go test ./internal/core/npc -bench=.
```

## Performance Considerations

- **Update Rates**: Configure appropriate update intervals for agents and sensors
- **Memory Management**: Set reasonable retention policies for memory systems
- **Event Processing**: Use priority-based event handling for performance
- **Blackboard Access**: Minimize unnecessary blackboard operations
- **Sensor Optimization**: Disable unused sensors and optimize update intervals

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Agent Manager │    │   Behavior Tree │    │   Blackboard    │
│                 │    │     Builder     │    │                 │
│ - Agent Lifecycle│    │ - Node Factories│    │ - Data Storage  │
│ - Event Routing │    │ - Config Parser │    │ - Type Safety   │
│ - Auto Updates  │    │ - Tree Building │    │ - Persistence   │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         └───────────────────────┼───────────────────────┘
                                 │
                    ┌─────────────────┐
                    │    AI Agent     │
                    │                 │
                    │ - Behavior Tree │
                    │ - Sensors       │
                    │ - Memory        │
                    │ - Event Handlers│
                    │ - State Mgmt    │
                    └─────────────────┘
                             │
        ┌────────────────────┼────────────────────┐
        │                    │                    │
┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐
│    Sensors      │ │     Memory      │ │ Event Handlers  │
│                 │ │                 │ │                 │
│ - Distance      │ │ - Event Storage │ │ - Damage        │
│ - Health        │ │ - Recall System │ │ - Combat        │
│ - Proximity     │ │ - Retention     �� │ - Items         │
│ - Environment   │ │ - Persistence   │ │ - Environment   │
└─────────────────┘ └─────────────────┘ └─────────────────┘
```

## Contributing

To extend the framework:

1. Implement the appropriate interfaces (`Node`, `Sensor`, `EventHandler`, etc.)
2. Create factory classes for configuration-based instantiation
3. Register your custom types with the appropriate managers
4. Add comprehensive tests for your extensions
5. Update documentation with usage examples

## License

This framework is part of the ZeuSync project and follows the project's licensing terms.