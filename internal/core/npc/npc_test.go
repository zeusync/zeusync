package npc

import (
	"context"
	"testing"
	"time"
)

func TestBlackboard(t *testing.T) {
	bb := NewBlackboard()

	// Test basic operations
	bb.Set("test_key", "test_value")

	value, exists := bb.Get("test_key")
	if !exists {
		t.Error("Expected key to exist")
	}

	if value != "test_value" {
		t.Errorf("Expected 'test_value', got %v", value)
	}

	// Test typed getters
	bb.Set("int_key", 42)
	intVal, ok := bb.GetInt("int_key")
	if !ok || intVal != 42 {
		t.Errorf("Expected 42, got %d", intVal)
	}

	bb.Set("bool_key", true)
	boolVal, ok := bb.GetBool("bool_key")
	if !ok || !boolVal {
		t.Errorf("Expected true, got %v", boolVal)
	}

	// Test JSON export/import
	data, err := bb.ToJSON()
	if err != nil {
		t.Errorf("Failed to export to JSON: %v", err)
	}

	bb2 := NewBlackboard()
	err = bb2.FromJSON(data)
	if err != nil {
		t.Errorf("Failed to import from JSON: %v", err)
	}

	value2, exists := bb2.Get("test_key")
	if !exists || value2 != "test_value" {
		t.Error("JSON import/export failed")
	}
}

func TestBasicMemory(t *testing.T) {
	memory := NewBasicMemory()

	// Test basic storage
	err := memory.Store("test_key", "test_value")
	if err != nil {
		t.Errorf("Failed to store value: %v", err)
	}

	value, exists := memory.Retrieve("test_key")
	if !exists {
		t.Error("Expected key to exist")
	}

	if value != "test_value" {
		t.Errorf("Expected 'test_value', got %v", value)
	}

	// Test memory events
	event := MemoryEvent{
		Type:      "test_event",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"test": "data",
		},
		Importance: 0.5,
		Tags:       []string{"test"},
	}

	err = memory.Remember(event)
	if err != nil {
		t.Errorf("Failed to remember event: %v", err)
	}

	// Test recall
	criteria := MemoryCriteria{
		Type: "test_event",
	}

	events, err := memory.Recall(criteria)
	if err != nil {
		t.Errorf("Failed to recall events: %v", err)
	}

	if len(events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(events))
	}

	if events[0].Type != "test_event" {
		t.Errorf("Expected 'test_event', got %s", events[0].Type)
	}
}

func TestBehaviorTreeBuilder(t *testing.T) {
	builder := NewBehaviorTreeBuilder()

	// Test simple sequence
	config := &BehaviorTreeConfig{
		Name: "Test Tree",
		Root: &NodeConfig{
			Name:    "TestSequence",
			Type:    "Sequence",
			Enabled: true,
			Children: []*NodeConfig{
				{
					Name:    "TestAction1",
					Type:    "Action",
					Enabled: true,
					Parameters: map[string]interface{}{
						"action_type": "set_blackboard",
						"key":         "test",
						"value":       "success",
					},
				},
				{
					Name:    "TestAction2",
					Type:    "Log",
					Enabled: true,
					Parameters: map[string]interface{}{
						"message": "Test completed",
					},
				},
			},
		},
	}

	tree, err := builder.BuildFromConfig(config)
	if err != nil {
		t.Errorf("Failed to build tree: %v", err)
	}

	if tree == nil {
		t.Error("Expected tree to be created")
	}

	if tree.GetType() != "Sequence" {
		t.Errorf("Expected 'Sequence', got %s", tree.GetType())
	}
}

func TestAIAgent(t *testing.T) {
	agent := NewAIAgent("test_agent", "Test Agent")

	// Test basic properties
	if agent.GetID() != "test_agent" {
		t.Errorf("Expected 'test_agent', got %s", agent.GetID())
	}

	if agent.GetName() != "Test Agent" {
		t.Errorf("Expected 'Test Agent', got %s", agent.GetName())
	}

	// Test blackboard access
	bb := agent.GetBlackboard()
	bb.Set("test", "value")

	value, exists := bb.Get("test")
	if !exists || value != "value" {
		t.Error("Blackboard access failed")
	}

	// Test sensor management
	sensor := NewTimerSensor("test_timer", "timer")
	agent.AddSensor(sensor)

	sensors := agent.GetSensors()
	if len(sensors) != 1 {
		t.Errorf("Expected 1 sensor, got %d", len(sensors))
	}

	// Test event handling
	event := CreateEvent("test", "source", agent.GetID(), map[string]interface{}{
		"test": "data",
	}, 5)

	err := agent.HandleEvent(event)
	if err != nil {
		t.Errorf("Failed to handle event: %v", err)
	}

	// Test save/load
	data, err := agent.Save()
	if err != nil {
		t.Errorf("Failed to save agent: %v", err)
	}

	agent2 := NewAIAgent("test_agent2", "Test Agent 2")
	err = agent2.Load(data)
	if err != nil {
		t.Errorf("Failed to load agent: %v", err)
	}

	if agent2.GetID() != "test_agent" {
		t.Error("Agent load failed")
	}
}

func TestAgentManager(t *testing.T) {
	manager := NewAIAgentManager()

	// Test agent creation
	config := CreateSimpleTestConfig("test_agent", "Test Agent")
	agent, err := manager.CreateAgent(config)
	if err != nil {
		t.Errorf("Failed to create agent: %v", err)
	}

	if agent == nil {
		t.Error("Expected agent to be created")
	}

	// Test agent retrieval
	retrievedAgent, exists := manager.GetAgent("test_agent")
	if !exists {
		t.Error("Expected agent to exist")
	}

	if retrievedAgent.GetID() != "test_agent" {
		t.Error("Retrieved wrong agent")
	}

	// Test agent update
	ctx := context.Background()
	err = manager.Update(ctx, time.Millisecond*100)
	if err != nil {
		t.Errorf("Failed to update agents: %v", err)
	}

	// Test event broadcasting
	event := CreateEvent("test", "manager", "*", map[string]interface{}{
		"broadcast": true,
	}, 5)

	err = manager.BroadcastEvent(event)
	if err != nil {
		t.Errorf("Failed to broadcast event: %v", err)
	}

	// Test agent removal
	removed := manager.RemoveAgent("test_agent")
	if !removed {
		t.Error("Expected agent to be removed")
	}

	_, exists = manager.GetAgent("test_agent")
	if exists {
		t.Error("Agent should have been removed")
	}
}

func TestSensors(t *testing.T) {
	// Test Distance Sensor
	bb := NewBlackboard()
	bb.Set("position", Position{X: 0, Y: 0, Z: 0})
	bb.Set("target_position", Position{X: 3, Y: 4, Z: 0})

	sensor := NewDistanceSensor("distance_test", 10.0, "target_position", "position", "distance")

	ctx := &ExecutionContext{
		Blackboard: bb,
	}

	err := sensor.Update(ctx)
	if err != nil {
		t.Errorf("Failed to update distance sensor: %v", err)
	}

	distance, exists := bb.GetFloat("distance")
	if !exists {
		t.Error("Expected distance to be calculated")
	}

	expectedDistance := 5.0 // sqrt(3^2 + 4^2)
	if distance != expectedDistance {
		t.Errorf("Expected distance %f, got %f", expectedDistance, distance)
	}

	// Test Health Sensor
	bb.Set("health", 30.0)
	bb.Set("max_health", 100.0)

	healthSensor := NewHealthSensor("health_test", "health", "max_health", "health_status", 0.5)

	err = healthSensor.Update(ctx)
	if err != nil {
		t.Errorf("Failed to update health sensor: %v", err)
	}

	isLow, exists := bb.GetBool("health_status_is_low")
	if !exists || !isLow {
		t.Error("Expected health to be marked as low")
	}
}

func TestEventHandlers(t *testing.T) {
	agent := NewAIAgent("test_agent", "Test Agent")
	bb := agent.GetBlackboard()
	bb.Set("health", 100.0)

	// Test damage handler
	damageHandler := NewDamageEventHandler("damage_test", "health", 8)
	agent.AddEventHandler(damageHandler)

	damageEvent := CreateDamageEvent("enemy", agent.GetID(), 25.0)
	err := agent.HandleEvent(damageEvent)
	if err != nil {
		t.Errorf("Failed to handle damage event: %v", err)
	}

	// Process the event
	ctx := context.Background()
	err = agent.Update(ctx, time.Millisecond*100)
	if err != nil {
		t.Errorf("Failed to update agent: %v", err)
	}

	health, exists := bb.GetFloat("health")
	if !exists {
		t.Error("Expected health to exist")
	}

	expectedHealth := 75.0
	if health != expectedHealth {
		t.Errorf("Expected health %f, got %f", expectedHealth, health)
	}
}

func TestComplexBehaviorTree(t *testing.T) {
	manager := NewAIAgentManager()

	// Create a game NPC
	config := CreateGameNPCConfig("npc_1", "Guard NPC")
	agent, err := manager.CreateAgent(config)
	if err != nil {
		t.Errorf("Failed to create NPC: %v", err)
	}

	// Set up initial state
	bb := agent.GetBlackboard()
	bb.Set("distance_to_enemy", 15.0) // Out of range initially

	// Update the agent
	ctx := context.Background()
	err = agent.Update(ctx, time.Millisecond*100)
	if err != nil {
		t.Errorf("Failed to update agent: %v", err)
	}

	// Check that agent is in idle state (no combat)
	state, exists := bb.Get("state")
	if !exists {
		t.Error("Expected state to be set")
	}

	if state != "idle" {
		t.Errorf("Expected 'idle', got %v", state)
	}

	// Simulate enemy approaching
	bb.Set("distance_to_enemy", 8.0)
	bb.Set("in_combat", true)

	// Update again
	err = agent.Update(ctx, time.Millisecond*100)
	if err != nil {
		t.Errorf("Failed to update agent: %v", err)
	}

	// The behavior tree should have executed the combat sequence
	// (This would be more thoroughly tested with custom action implementations)
}

func TestConfigurationValidation(t *testing.T) {
	// Test invalid config
	invalidConfig := &BehaviorTreeConfig{
		Name: "",  // Invalid: empty name
		Root: nil, // Invalid: no root
	}

	err := invalidConfig.Validate()
	if err == nil {
		t.Error("Expected validation to fail")
	}

	// Test valid config
	validConfig := &BehaviorTreeConfig{
		Name: "Valid Tree",
		Root: &NodeConfig{
			Name:    "Root",
			Type:    "Sequence",
			Enabled: true,
		},
	}

	err = validConfig.Validate()
	if err != nil {
		t.Errorf("Expected validation to pass: %v", err)
	}
}

func TestMemoryRetention(t *testing.T) {
	// Create memory with short retention
	memory := NewBasicMemoryWithConfig(5, time.Millisecond*100)

	// Add some events
	for i := 0; i < 3; i++ {
		event := MemoryEvent{
			Type:      "test",
			Timestamp: time.Now(),
			Data:      map[string]interface{}{"index": i},
			Tags:      []string{"test"},
		}
		memory.Remember(event)
	}

	// Wait for retention period
	time.Sleep(time.Millisecond * 150)

	// Add a new event (this should trigger cleanup)
	newEvent := MemoryEvent{
		Type:      "test",
		Timestamp: time.Now(),
		Data:      map[string]interface{}{"index": "new"},
		Tags:      []string{"test"},
	}
	memory.Remember(newEvent)

	// Check that old events were cleaned up
	events, err := memory.Recall(MemoryCriteria{Type: "test"})
	if err != nil {
		t.Errorf("Failed to recall events: %v", err)
	}

	// Should only have the new event (old ones expired)
	if len(events) != 1 {
		t.Errorf("Expected 1 event after cleanup, got %d", len(events))
	}

	if events[0].Data["index"] != "new" {
		t.Error("Wrong event remained after cleanup")
	}
}

func BenchmarkBlackboardOperations(b *testing.B) {
	bb := NewBlackboard()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "test_key"
		bb.Set(key, i)
		_, _ = bb.Get(key)
	}
}

func BenchmarkAgentUpdate(b *testing.B) {
	manager := NewAIAgentManager()
	config := CreateSimpleTestConfig("bench_agent", "Benchmark Agent")
	agent, _ := manager.CreateAgent(config)

	ctx := context.Background()
	deltaTime := time.Millisecond * 16 // ~60 FPS

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		agent.Update(ctx, deltaTime)
	}
}

func BenchmarkBehaviorTreeExecution(b *testing.B) {
	builder := NewBehaviorTreeBuilder()
	config := &BehaviorTreeConfig{
		Name: "Benchmark Tree",
		Root: &NodeConfig{
			Name:    "BenchSequence",
			Type:    "Sequence",
			Enabled: true,
			Children: []*NodeConfig{
				{
					Name:    "Action1",
					Type:    "Action",
					Enabled: true,
					Parameters: map[string]interface{}{
						"action_type": "set_blackboard",
						"key":         "test",
						"value":       "value",
					},
				},
				{
					Name:    "Action2",
					Type:    "Action",
					Enabled: true,
					Parameters: map[string]interface{}{
						"action_type": "increment",
						"key":         "counter",
						"increment":   1,
					},
				},
			},
		},
	}

	tree, _ := builder.BuildFromConfig(config)
	bb := NewBlackboard()
	bb.Set("counter", 0)

	ctx := &ExecutionContext{
		Context:    context.Background(),
		Blackboard: bb,
		DeltaTime:  time.Millisecond * 16,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree.Execute(ctx)
		tree.Reset()
	}
}
