package npc

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// AIAgentManager manages multiple AI agents
type AIAgentManager struct {
	mu              sync.RWMutex
	agents          map[string]Agent
	behaviorBuilder BehaviorTreeBuilder
	sensorFactories map[string]SensorFactory
	eventHandlers   map[string][]EventHandler

	// Global event queue
	globalEvents []Event
	eventMu      sync.Mutex

	// Update control
	updateInterval time.Duration
	lastUpdate     time.Time
	isRunning      bool
	stopChan       chan struct{}
}

// NewAIAgentManager creates a new agent manager
func NewAIAgentManager() *AIAgentManager {
	manager := &AIAgentManager{
		agents:          make(map[string]Agent),
		behaviorBuilder: NewBehaviorTreeBuilder(),
		sensorFactories: make(map[string]SensorFactory),
		eventHandlers:   make(map[string][]EventHandler),
		globalEvents:    make([]Event, 0),
		updateInterval:  time.Millisecond * 50, // 20 FPS default
		stopChan:        make(chan struct{}),
	}

	// Register default sensor factories
	manager.registerDefaultSensorFactories()

	return manager
}

// registerDefaultSensorFactories registers built-in sensor factories
func (am *AIAgentManager) registerDefaultSensorFactories() {
	am.sensorFactories["Distance"] = &DistanceSensorFactory{}
	am.sensorFactories["Health"] = &HealthSensorFactory{}
	am.sensorFactories["Inventory"] = &InventorySensorFactory{}
	am.sensorFactories["Timer"] = &TimerSensorFactory{}
	am.sensorFactories["Environment"] = &EnvironmentSensorFactory{}
	am.sensorFactories["Proximity"] = &ProximitySensorFactory{}
}

// CreateAgent creates a new agent from configuration
func (am *AIAgentManager) CreateAgent(config *AgentConfig) (Agent, error) {
	if config == nil {
		return nil, fmt.Errorf("agent config cannot be nil")
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid agent config: %w", err)
	}

	am.mu.Lock()
	defer am.mu.Unlock()

	// Check if agent already exists
	if _, exists := am.agents[config.ID]; exists {
		return nil, fmt.Errorf("agent with ID %s already exists", config.ID)
	}

	// Create agent
	agent := NewAIAgent(config.ID, config.Name)

	// Set update rate
	if config.UpdateRate > 0 {
		agent.SetUpdateRate(config.UpdateRate)
	}

	// Set initial blackboard data
	if config.InitialData != nil {
		for key, value := range config.InitialData {
			agent.GetBlackboard().Set(key, value)
		}
	}

	// Configure memory
	if config.Memory != nil {
		memory, err := am.createMemory(config.Memory)
		if err != nil {
			return nil, fmt.Errorf("failed to create memory: %w", err)
		}
		agent.SetMemory(memory)
	}

	// Create and set behavior tree
	if config.BehaviorTree != nil {
		behaviorTree, err := am.behaviorBuilder.BuildFromConfig(config.BehaviorTree)
		if err != nil {
			return nil, fmt.Errorf("failed to build behavior tree: %w", err)
		}
		agent.SetBehaviorTree(behaviorTree)
	}

	// Create and add sensors
	for _, sensorConfig := range config.Sensors {
		sensor, err := am.createSensor(sensorConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create sensor %s: %w", sensorConfig.Name, err)
		}
		agent.AddSensor(sensor)
	}

	// Add event handlers
	for _, handlerConfig := range config.EventHandlers {
		handler, err := am.createEventHandler(handlerConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create event handler %s: %w", handlerConfig.Name, err)
		}
		agent.AddEventHandler(handler)
	}

	// Set active state
	agent.SetActive(config.Active)

	// Store agent
	am.agents[config.ID] = agent

	return agent, nil
}

// createMemory creates a memory instance from configuration
func (am *AIAgentManager) createMemory(config *MemoryConfig) (Memory, error) {
	switch config.Type {
	case "basic", "":
		maxEvents := config.MaxEvents
		if maxEvents <= 0 {
			maxEvents = 1000
		}

		retention := config.RetentionTime
		if retention <= 0 {
			retention = time.Hour * 24
		}

		return NewBasicMemoryWithConfig(maxEvents, retention), nil
	default:
		return nil, fmt.Errorf("unknown memory type: %s", config.Type)
	}
}

// createSensor creates a sensor instance from configuration
func (am *AIAgentManager) createSensor(config *SensorConfig) (Sensor, error) {
	factory, exists := am.sensorFactories[config.Type]
	if !exists {
		return nil, fmt.Errorf("unknown sensor type: %s", config.Type)
	}

	return factory.CreateSensor(config)
}

// createEventHandler creates an event handler from configuration
func (am *AIAgentManager) createEventHandler(config *EventHandlerConfig) (EventHandler, error) {
	// This would be extended to support different event handler types
	return &BasicEventHandler{
		name:      config.Name,
		eventType: config.EventType,
		priority:  config.Priority,
		enabled:   config.Enabled,
	}, nil
}

// GetAgent retrieves an agent by ID
func (am *AIAgentManager) GetAgent(id string) (Agent, bool) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	agent, exists := am.agents[id]
	return agent, exists
}

// RemoveAgent removes an agent
func (am *AIAgentManager) RemoveAgent(id string) bool {
	am.mu.Lock()
	defer am.mu.Unlock()

	if _, exists := am.agents[id]; exists {
		delete(am.agents, id)
		return true
	}
	return false
}

// GetAllAgents returns all agents
func (am *AIAgentManager) GetAllAgents() []Agent {
	am.mu.RLock()
	defer am.mu.RUnlock()

	agents := make([]Agent, 0, len(am.agents))
	for _, agent := range am.agents {
		agents = append(agents, agent)
	}
	return agents
}

// Update updates all agents
func (am *AIAgentManager) Update(ctx context.Context, deltaTime time.Duration) error {
	am.mu.RLock()
	agents := make([]Agent, 0, len(am.agents))
	for _, agent := range am.agents {
		agents = append(agents, agent)
	}
	am.mu.RUnlock()

	// Process global events first
	if err := am.processGlobalEvents(ctx); err != nil {
		return fmt.Errorf("failed to process global events: %w", err)
	}

	// Update all agents
	for _, agent := range agents {
		if err := agent.Update(ctx, deltaTime); err != nil {
			return fmt.Errorf("failed to update agent %s: %w", agent.GetID(), err)
		}
	}

	return nil
}

// processGlobalEvents processes events that affect multiple agents
func (am *AIAgentManager) processGlobalEvents(ctx context.Context) error {
	am.eventMu.Lock()
	events := make([]Event, len(am.globalEvents))
	copy(events, am.globalEvents)
	am.globalEvents = am.globalEvents[:0] // Clear the queue
	am.eventMu.Unlock()

	for _, event := range events {
		// Send event to all agents or specific targets
		if event.Target == "" || event.Target == "*" {
			// Broadcast to all agents
			if err := am.BroadcastEvent(event); err != nil {
				return fmt.Errorf("failed to broadcast event %s: %w", event.ID, err)
			}
		} else {
			// Send to specific agent
			if err := am.SendEvent(event.Target, event); err != nil {
				return fmt.Errorf("failed to send event %s to %s: %w", event.ID, event.Target, err)
			}
		}
	}

	return nil
}

// BroadcastEvent sends an event to all agents
func (am *AIAgentManager) BroadcastEvent(event Event) error {
	am.mu.RLock()
	defer am.mu.RUnlock()

	for _, agent := range am.agents {
		if err := agent.HandleEvent(event); err != nil {
			return fmt.Errorf("agent %s failed to handle event: %w", agent.GetID(), err)
		}
	}

	return nil
}

// SendEvent sends an event to a specific agent
func (am *AIAgentManager) SendEvent(agentID string, event Event) error {
	agent, exists := am.GetAgent(agentID)
	if !exists {
		return fmt.Errorf("agent %s not found", agentID)
	}

	return agent.HandleEvent(event)
}

// QueueGlobalEvent queues an event for global processing
func (am *AIAgentManager) QueueGlobalEvent(event Event) {
	am.eventMu.Lock()
	defer am.eventMu.Unlock()

	am.globalEvents = append(am.globalEvents, event)
}

// StartAutoUpdate starts automatic agent updates
func (am *AIAgentManager) StartAutoUpdate(ctx context.Context) {
	am.mu.Lock()
	if am.isRunning {
		am.mu.Unlock()
		return
	}
	am.isRunning = true
	am.mu.Unlock()

	go am.updateLoop(ctx)
}

// StopAutoUpdate stops automatic agent updates
func (am *AIAgentManager) StopAutoUpdate() {
	am.mu.Lock()
	defer am.mu.Unlock()

	if am.isRunning {
		am.isRunning = false
		close(am.stopChan)
		am.stopChan = make(chan struct{})
	}
}

// updateLoop runs the automatic update loop
func (am *AIAgentManager) updateLoop(ctx context.Context) {
	ticker := time.NewTicker(am.updateInterval)
	defer ticker.Stop()

	lastTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			return
		case <-am.stopChan:
			return
		case now := <-ticker.C:
			deltaTime := now.Sub(lastTime)
			lastTime = now

			if err := am.Update(ctx, deltaTime); err != nil {
				fmt.Printf("Agent manager update error: %v\n", err)
			}
		}
	}
}

// SaveAll saves all agents' states
func (am *AIAgentManager) SaveAll() (map[string][]byte, error) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	states := make(map[string][]byte)

	for id, agent := range am.agents {
		data, err := agent.Save()
		if err != nil {
			return nil, fmt.Errorf("failed to save agent %s: %w", id, err)
		}
		states[id] = data
	}

	return states, nil
}

// LoadAll loads all agents' states
func (am *AIAgentManager) LoadAll(data map[string][]byte) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	for id, agentData := range data {
		if agent, exists := am.agents[id]; exists {
			if err := agent.Load(agentData); err != nil {
				return fmt.Errorf("failed to load agent %s: %w", id, err)
			}
		}
	}

	return nil
}

// RegisterSensorFactory registers a custom sensor factory
func (am *AIAgentManager) RegisterSensorFactory(sensorType string, factory SensorFactory) {
	am.mu.Lock()
	defer am.mu.Unlock()

	am.sensorFactories[sensorType] = factory
}

// GetStats returns manager statistics
func (am *AIAgentManager) GetStats() map[string]interface{} {
	am.mu.RLock()
	defer am.mu.RUnlock()

	activeAgents := 0
	for _, agent := range am.agents {
		if agent.IsActive() {
			activeAgents++
		}
	}

	am.eventMu.Lock()
	queuedEvents := len(am.globalEvents)
	am.eventMu.Unlock()

	return map[string]interface{}{
		"total_agents":       len(am.agents),
		"active_agents":      activeAgents,
		"queued_events":      queuedEvents,
		"update_interval":    am.updateInterval.String(),
		"is_running":         am.isRunning,
		"last_update":        am.lastUpdate,
		"registered_sensors": len(am.sensorFactories),
	}
}

// Sensor Factory Interfaces and Implementations

// SensorFactory creates sensors of a specific type
type SensorFactory interface {
	CreateSensor(config *SensorConfig) (Sensor, error)
	GetSensorType() string
}

// DistanceSensorFactory creates distance sensors
type DistanceSensorFactory struct{}

func (f *DistanceSensorFactory) CreateSensor(config *SensorConfig) (Sensor, error) {
	maxRange, _ := getFloatParameter(config.Parameters, "max_range", 10.0)
	targetKey, _ := getStringParameter(config.Parameters, "target_key", "target_position")
	positionKey, _ := getStringParameter(config.Parameters, "position_key", "position")
	outputKey, _ := getStringParameter(config.Parameters, "output_key", "distance")

	sensor := NewDistanceSensor(config.Name, maxRange, targetKey, positionKey, outputKey)
	sensor.SetEnabled(config.Enabled)

	if config.UpdateInterval > 0 {
		sensor.SetUpdateInterval(config.UpdateInterval)
	}

	return sensor, nil
}

func (f *DistanceSensorFactory) GetSensorType() string {
	return "Distance"
}

// HealthSensorFactory creates health sensors
type HealthSensorFactory struct{}

func (f *HealthSensorFactory) CreateSensor(config *SensorConfig) (Sensor, error) {
	healthKey, _ := getStringParameter(config.Parameters, "health_key", "health")
	maxHealthKey, _ := getStringParameter(config.Parameters, "max_health_key", "max_health")
	outputKey, _ := getStringParameter(config.Parameters, "output_key", "health_status")
	lowThreshold, _ := getFloatParameter(config.Parameters, "low_threshold", 0.3)

	sensor := NewHealthSensor(config.Name, healthKey, maxHealthKey, outputKey, lowThreshold)
	sensor.SetEnabled(config.Enabled)

	if config.UpdateInterval > 0 {
		sensor.SetUpdateInterval(config.UpdateInterval)
	}

	return sensor, nil
}

func (f *HealthSensorFactory) GetSensorType() string {
	return "Health"
}

// Additional sensor factories would be implemented similarly...

// InventorySensorFactory creates inventory sensors
type InventorySensorFactory struct{}

func (f *InventorySensorFactory) CreateSensor(config *SensorConfig) (Sensor, error) {
	inventoryKey, _ := getStringParameter(config.Parameters, "inventory_key", "inventory")
	outputKey, _ := getStringParameter(config.Parameters, "output_key", "inventory_status")
	itemTypes, _ := getStringSliceParameter(config.Parameters, "item_types", []string{})

	sensor := NewInventorySensor(config.Name, inventoryKey, outputKey, itemTypes)
	sensor.SetEnabled(config.Enabled)

	if config.UpdateInterval > 0 {
		sensor.SetUpdateInterval(config.UpdateInterval)
	}

	return sensor, nil
}

func (f *InventorySensorFactory) GetSensorType() string {
	return "Inventory"
}

// TimerSensorFactory creates timer sensors
type TimerSensorFactory struct{}

func (f *TimerSensorFactory) CreateSensor(config *SensorConfig) (Sensor, error) {
	outputKey, _ := getStringParameter(config.Parameters, "output_key", "timer")

	sensor := NewTimerSensor(config.Name, outputKey)
	sensor.SetEnabled(config.Enabled)

	if config.UpdateInterval > 0 {
		sensor.SetUpdateInterval(config.UpdateInterval)
	}

	return sensor, nil
}

func (f *TimerSensorFactory) GetSensorType() string {
	return "Timer"
}

// EnvironmentSensorFactory creates environment sensors
type EnvironmentSensorFactory struct{}

func (f *EnvironmentSensorFactory) CreateSensor(config *SensorConfig) (Sensor, error) {
	environmentKey, _ := getStringParameter(config.Parameters, "environment_key", "environment")
	outputKey, _ := getStringParameter(config.Parameters, "output_key", "environment_status")
	conditions, _ := getStringSliceParameter(config.Parameters, "conditions", []string{})

	sensor := NewEnvironmentSensor(config.Name, environmentKey, outputKey, conditions)
	sensor.SetEnabled(config.Enabled)

	if config.UpdateInterval > 0 {
		sensor.SetUpdateInterval(config.UpdateInterval)
	}

	return sensor, nil
}

func (f *EnvironmentSensorFactory) GetSensorType() string {
	return "Environment"
}

// ProximitySensorFactory creates proximity sensors
type ProximitySensorFactory struct{}

func (f *ProximitySensorFactory) CreateSensor(config *SensorConfig) (Sensor, error) {
	positionKey, _ := getStringParameter(config.Parameters, "position_key", "position")
	entitiesKey, _ := getStringParameter(config.Parameters, "entities_key", "entities")
	outputKey, _ := getStringParameter(config.Parameters, "output_key", "proximity")
	maxRange, _ := getFloatParameter(config.Parameters, "max_range", 5.0)
	entityTypes, _ := getStringSliceParameter(config.Parameters, "entity_types", []string{})

	sensor := NewProximitySensor(config.Name, positionKey, entitiesKey, outputKey, maxRange, entityTypes)
	sensor.SetEnabled(config.Enabled)

	if config.UpdateInterval > 0 {
		sensor.SetUpdateInterval(config.UpdateInterval)
	}

	return sensor, nil
}

func (f *ProximitySensorFactory) GetSensorType() string {
	return "Proximity"
}

// Helper functions for parameter extraction
func getStringParameter(params map[string]interface{}, key, defaultValue string) (string, bool) {
	if params == nil {
		return defaultValue, false
	}

	if value, exists := params[key]; exists {
		if str, ok := value.(string); ok {
			return str, true
		}
	}

	return defaultValue, false
}

func getFloatParameter(params map[string]interface{}, key string, defaultValue float64) (float64, bool) {
	if params == nil {
		return defaultValue, false
	}

	if value, exists := params[key]; exists {
		switch v := value.(type) {
		case float64:
			return v, true
		case int:
			return float64(v), true
		}
	}

	return defaultValue, false
}

func getStringSliceParameter(params map[string]interface{}, key string, defaultValue []string) ([]string, bool) {
	if params == nil {
		return defaultValue, false
	}

	if value, exists := params[key]; exists {
		if slice, ok := value.([]interface{}); ok {
			result := make([]string, 0, len(slice))
			for _, item := range slice {
				if str, ok := item.(string); ok {
					result = append(result, str)
				}
			}
			return result, true
		}
	}

	return defaultValue, false
}
