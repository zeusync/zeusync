package npc

import (
	"context"
	"fmt"
	"time"
)

// RunDemo demonstrates the AI agent framework capabilities
func RunDemo() {
	fmt.Println("=== AI Agent Framework Demo ===\n")

	// Create agent manager
	manager := NewAIAgentManager()

	// Demo 1: Simple Test Agent
	fmt.Println("1. Creating Simple Test Agent...")
	runSimpleTestDemo(manager)

	// Demo 2: Game NPC
	fmt.Println("\n2. Creating Game NPC...")
	runGameNPCDemo(manager)

	// Demo 3: Trading Agent
	fmt.Println("\n3. Creating Trading Agent...")
	runTradingAgentDemo(manager)

	// Demo 4: Event System
	fmt.Println("\n4. Demonstrating Event System...")
	runEventSystemDemo(manager)

	// Demo 5: Memory System
	fmt.Println("\n5. Demonstrating Memory System...")
	runMemorySystemDemo()

	// Demo 6: Parallel Behavior
	fmt.Println("\n6. Demonstrating Parallel Behavior...")
	runParallelBehaviorDemo(manager)

	fmt.Println("\n=== Demo Complete ===")
}

func runSimpleTestDemo(manager *AIAgentManager) {
	config := CreateSimpleTestConfig("test_agent", "Simple Test Agent")
	agent, err := manager.CreateAgent(config)
	if err != nil {
		fmt.Printf("Error creating agent: %v\n", err)
		return
	}

	fmt.Printf("Created agent: %s (%s)\n", agent.GetName(), agent.GetID())

	// Run a few update cycles
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		err := agent.Update(ctx, time.Millisecond*100)
		if err != nil {
			fmt.Printf("Update error: %v\n", err)
		}
		time.Sleep(time.Millisecond * 200)
	}

	// Check blackboard state
	bb := agent.GetBlackboard()
	counter, exists := bb.GetInt("counter")
	if exists {
		fmt.Printf("Counter value: %d\n", counter)
	}

	testValue, exists := bb.GetString("test_value")
	if exists {
		fmt.Printf("Test value: %s\n", testValue)
	}
}

func runGameNPCDemo(manager *AIAgentManager) {
	config := CreateGameNPCConfig("guard_1", "Castle Guard")
	agent, err := manager.CreateAgent(config)
	if err != nil {
		fmt.Printf("Error creating NPC: %v\n", err)
		return
	}

	fmt.Printf("Created NPC: %s (%s)\n", agent.GetName(), agent.GetID())

	// Set up initial game state
	bb := agent.GetBlackboard()
	bb.Set("position", Position{X: 10, Y: 5, Z: 0})
	bb.Set("nearby_entities", []interface{}{
		map[string]interface{}{
			"type":     "player",
			"position": Position{X: 15, Y: 8, Z: 0},
			"id":       "player_1",
		},
	})

	ctx := context.Background()

	// Simulate normal patrol
	fmt.Println("  - Guard patrolling (no threats detected)")
	for i := 0; i < 2; i++ {
		agent.Update(ctx, time.Millisecond*100)
		time.Sleep(time.Millisecond * 150)
	}

	// Simulate combat situation
	fmt.Println("  - Enemy detected! Entering combat mode")
	bb.Set("in_combat", true)
	bb.Set("distance_to_enemy", 8.0)

	for i := 0; i < 2; i++ {
		agent.Update(ctx, time.Millisecond*100)
		time.Sleep(time.Millisecond * 150)
	}

	// Send damage event
	damageEvent := CreateDamageEvent("enemy_orc", agent.GetID(), 30.0)
	agent.HandleEvent(damageEvent)
	agent.Update(ctx, time.Millisecond*100)

	health, exists := bb.GetFloat("health")
	if exists {
		fmt.Printf("  - Guard health after damage: %.1f\n", health)
	}
}

func runTradingAgentDemo(manager *AIAgentManager) {
	config := CreateTradingAgentConfig("trader_1", "Market Trader")
	agent, err := manager.CreateAgent(config)
	if err != nil {
		fmt.Printf("Error creating trader: %v\n", err)
		return
	}

	fmt.Printf("Created trader: %s (%s)\n", agent.GetName(), agent.GetID())

	bb := agent.GetBlackboard()
	ctx := context.Background()

	// Simulate high demand market
	fmt.Println("  - High demand market conditions")
	bb.Set("market_demand", 0.9)

	for i := 0; i < 2; i++ {
		agent.Update(ctx, time.Millisecond*100)
		time.Sleep(time.Millisecond * 100)
	}

	priceMultiplier, exists := bb.GetFloat("price_multiplier")
	if exists {
		fmt.Printf("  - Price multiplier adjusted to: %.2f\n", priceMultiplier)
	}

	// Simulate low demand market
	fmt.Println("  - Low demand market conditions")
	bb.Set("market_demand", 0.2)

	for i := 0; i < 2; i++ {
		agent.Update(ctx, time.Millisecond*100)
		time.Sleep(time.Millisecond * 100)
	}

	priceMultiplier, exists = bb.GetFloat("price_multiplier")
	if exists {
		fmt.Printf("  - Price multiplier adjusted to: %.2f\n", priceMultiplier)
	}

	// Simulate item transaction
	itemEvent := CreateItemEvent("customer", agent.GetID(), "pickup", "goods", 3)
	agent.HandleEvent(itemEvent)
	agent.Update(ctx, time.Millisecond*100)

	inventory, exists := bb.Get("inventory")
	if exists {
		fmt.Printf("  - Inventory after sale: %v\n", inventory)
	}
}

func runEventSystemDemo(manager *AIAgentManager) {
	config := CreateSimpleTestConfig("event_agent", "Event Test Agent")
	agent, err := manager.CreateAgent(config)
	if err != nil {
		fmt.Printf("Error creating agent: %v\n", err)
		return
	}

	fmt.Printf("Created agent for event testing: %s\n", agent.GetName())

	// Add event handlers
	damageHandler := NewDamageEventHandler("damage_handler", "health", 8)
	healHandler := NewHealEventHandler("heal_handler", "health", "max_health", 6)
	itemHandler := NewItemEventHandler("item_handler", "inventory", 4)

	agent.AddEventHandler(damageHandler)
	agent.AddEventHandler(healHandler)
	agent.AddEventHandler(itemHandler)

	// Set initial state
	bb := agent.GetBlackboard()
	bb.Set("health", 100.0)
	bb.Set("max_health", 100.0)
	bb.Set("inventory", map[string]int{"potions": 5})

	ctx := context.Background()

	// Send various events
	fmt.Println("  - Sending damage event")
	damageEvent := CreateDamageEvent("trap", agent.GetID(), 25.0)
	agent.HandleEvent(damageEvent)
	agent.Update(ctx, time.Millisecond*100)

	health, _ := bb.GetFloat("health")
	fmt.Printf("  - Health after damage: %.1f\n", health)

	fmt.Println("  - Sending heal event")
	healEvent := CreateHealEvent("potion", agent.GetID(), 15.0)
	agent.HandleEvent(healEvent)
	agent.Update(ctx, time.Millisecond*100)

	health, _ = bb.GetFloat("health")
	fmt.Printf("  - Health after healing: %.1f\n", health)

	fmt.Println("  - Sending item pickup event")
	itemEvent := CreateItemEvent("chest", agent.GetID(), "pickup", "sword", 1)
	agent.HandleEvent(itemEvent)
	agent.Update(ctx, time.Millisecond*100)

	inventory, _ := bb.Get("inventory")
	fmt.Printf("  - Inventory after pickup: %v\n", inventory)
}

func runMemorySystemDemo() {
	fmt.Println("Creating memory system...")
	memory := NewBasicMemory()

	// Store some basic data
	memory.Store("home_location", Position{X: 0, Y: 0, Z: 0})
	memory.Store("favorite_weapon", "sword")

	// Remember some events
	events := []MemoryEvent{
		{
			Type:      "combat",
			Timestamp: time.Now().Add(-time.Hour),
			Data: map[string]interface{}{
				"enemy":    "orc",
				"outcome":  "victory",
				"location": Position{X: 10, Y: 15, Z: 0},
			},
			Importance: 0.8,
			Tags:       []string{"combat", "victory"},
		},
		{
			Type:      "trade",
			Timestamp: time.Now().Add(-time.Minute * 30),
			Data: map[string]interface{}{
				"item":     "potion",
				"price":    50,
				"merchant": "trader_bob",
			},
			Importance: 0.3,
			Tags:       []string{"trade", "potion"},
		},
		{
			Type:      "discovery",
			Timestamp: time.Now().Add(-time.Minute * 10),
			Data: map[string]interface{}{
				"item":     "treasure_chest",
				"location": Position{X: 25, Y: 30, Z: 0},
			},
			Importance: 0.9,
			Tags:       []string{"discovery", "treasure"},
		},
	}

	for _, event := range events {
		memory.Remember(event)
	}

	// Demonstrate recall
	fmt.Println("  - Recalling combat events:")
	combatEvents, _ := memory.Recall(MemoryCriteria{Type: "combat"})
	for _, event := range combatEvents {
		fmt.Printf("    Combat vs %s at %v (importance: %.1f)\n",
			event.Data["enemy"], event.Data["location"], event.Importance)
	}

	fmt.Println("  - Recalling important events (>0.5):")
	importance := 0.5
	importantEvents, _ := memory.Recall(MemoryCriteria{Importance: &importance})
	for _, event := range importantEvents {
		fmt.Printf("    %s event (importance: %.1f)\n", event.Type, event.Importance)
	}

	fmt.Println("  - Recalling recent events (last 2):")
	recentEvents, _ := memory.Recall(MemoryCriteria{Limit: 2})
	for _, event := range recentEvents {
		fmt.Printf("    %s event at %s\n", event.Type, event.Timestamp.Format("15:04:05"))
	}

	// Show memory stats
	stats := memory.GetStats()
	fmt.Printf("  - Memory stats: %v\n", stats)
}

func runParallelBehaviorDemo(manager *AIAgentManager) {
	config := CreateParallelBehaviorConfig("parallel_agent", "Parallel Test Agent")
	agent, err := manager.CreateAgent(config)
	if err != nil {
		fmt.Printf("Error creating parallel agent: %v\n", err)
		return
	}

	fmt.Printf("Created parallel agent: %s\n", agent.GetName())
	fmt.Println("  - Executing parallel tasks...")

	ctx := context.Background()

	// Run the parallel behavior
	for i := 0; i < 10; i++ {
		err := agent.Update(ctx, time.Millisecond*100)
		if err != nil {
			fmt.Printf("Update error: %v\n", err)
		}
		time.Sleep(time.Millisecond * 200)

		// Check behavior tree status
		bb := agent.GetBlackboard()
		if status, exists := bb.GetString("last_behavior_status"); exists {
			if status == "Success" {
				fmt.Println("  - All parallel tasks completed successfully!")
				break
			}
		}
	}
}

// DemoCustomNode shows how to create and use custom nodes
func DemoCustomNode() {
	fmt.Println("\n=== Custom Node Demo ===")

	// Create a custom action node
	customAction := NewBasicActionNode("custom_demo", func(ctx *ExecutionContext) NodeStatus {
		fmt.Println("  - Custom action executed!")
		ctx.Blackboard.Set("custom_executed", true)
		return StatusSuccess
	})

	// Create a simple tree with the custom node
	sequence := NewSequenceNode("custom_sequence")
	sequence.AddChild(customAction)

	// Execute the tree
	bb := NewBlackboard()
	execCtx := &ExecutionContext{
		Context:    context.Background(),
		Blackboard: bb,
		DeltaTime:  time.Millisecond * 100,
	}

	status := sequence.Execute(execCtx)
	fmt.Printf("  - Execution status: %s\n", status)

	executed, exists := bb.GetBool("custom_executed")
	if exists && executed {
		fmt.Println("  - Custom node successfully executed!")
	}
}

// DemoSensorSystem shows sensor functionality
func DemoSensorSystem() {
	fmt.Println("\n=== Sensor System Demo ===")

	bb := NewBlackboard()

	// Set up test data
	bb.Set("position", Position{X: 0, Y: 0, Z: 0})
	bb.Set("target_position", Position{X: 3, Y: 4, Z: 0})
	bb.Set("health", 75.0)
	bb.Set("max_health", 100.0)
	bb.Set("inventory", map[string]int{
		"potions": 3,
		"weapons": 1,
		"gold":    150,
	})

	// Create sensors
	distanceSensor := NewDistanceSensor("distance_test", 10.0, "target_position", "position", "distance")
	healthSensor := NewHealthSensor("health_test", "health", "max_health", "health_status", 0.8)
	inventorySensor := NewInventorySensor("inventory_test", "inventory", "inventory_status",
		[]string{"potions", "weapons", "gold"})

	execCtx := &ExecutionContext{
		Blackboard: bb,
		DeltaTime:  time.Millisecond * 100,
	}

	// Update sensors
	fmt.Println("  - Updating sensors...")

	distanceSensor.Update(execCtx)
	distance, _ := bb.GetFloat("distance")
	fmt.Printf("    Distance to target: %.1f\n", distance)

	healthSensor.Update(execCtx)
	healthPercentage, _ := bb.GetFloat("health_status_percentage")
	isLow, _ := bb.GetBool("health_status_is_low")
	fmt.Printf("    Health: %.1f%% (low: %v)\n", healthPercentage*100, isLow)

	inventorySensor.Update(execCtx)
	potionCount, _ := bb.GetInt("inventory_status_potions_count")
	totalItems, _ := bb.GetInt("inventory_status_total")
	fmt.Printf("    Inventory: %d potions, %d total items\n", potionCount, totalItems)
}

// RunFullDemo runs all demonstrations
func RunFullDemo() {
	RunDemo()
	DemoCustomNode()
	DemoSensorSystem()
}
