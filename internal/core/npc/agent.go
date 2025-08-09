package npc

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"
)

// AIAgent represents a complete AI agent implementation
type AIAgent struct {
	mu            sync.RWMutex
	id            string
	name          string
	blackboard    *Blackboard
	memory        Memory
	behaviorTree  Node
	sensors       map[string]Sensor
	eventHandlers map[string][]EventHandler

	// State management
	active     bool
	lastUpdate time.Time
	updateRate time.Duration

	// Event queue
	eventQueue []Event
	eventMu    sync.Mutex
}

// NewAIAgent creates a new AI agent
func NewAIAgent(id, name string) *AIAgent {
	return &AIAgent{
		id:            id,
		name:          name,
		blackboard:    NewBlackboard(),
		memory:        NewBasicMemory(),
		sensors:       make(map[string]Sensor),
		eventHandlers: make(map[string][]EventHandler),
		active:        true,
		updateRate:    time.Millisecond * 100, // Default 10 FPS
		eventQueue:    make([]Event, 0),
	}
}

// GetID returns the agent's unique identifier
func (a *AIAgent) GetID() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.id
}

// GetName returns the agent's name
func (a *AIAgent) GetName() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.name
}

// GetBlackboard returns the agent's blackboard
func (a *AIAgent) GetBlackboard() *Blackboard {
	return a.blackboard
}

// GetMemory returns the agent's memory system
func (a *AIAgent) GetMemory() Memory {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.memory
}

// SetMemory sets the agent's memory system
func (a *AIAgent) SetMemory(memory Memory) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.memory = memory
}

// Update updates the agent (called each frame/tick)
func (a *AIAgent) Update(ctx context.Context, deltaTime time.Duration) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.active {
		return nil
	}

	// Check if enough time has passed since last update
	if time.Since(a.lastUpdate) < a.updateRate {
		return nil
	}

	a.lastUpdate = time.Now()

	// Process events
	if err := a.processEvents(ctx); err != nil {
		return fmt.Errorf("failed to process events: %w", err)
	}

	// Update sensors
	if err := a.updateSensors(ctx, deltaTime); err != nil {
		return fmt.Errorf("failed to update sensors: %w", err)
	}

	// Execute behavior tree
	if a.behaviorTree != nil {
		execCtx := &ExecutionContext{
			Context:    ctx,
			Blackboard: a.blackboard,
			Agent:      a,
			DeltaTime:  deltaTime,
		}

		status := a.behaviorTree.Execute(execCtx)
		a.blackboard.Set("last_behavior_status", status.String())
		a.blackboard.Set("last_update_time", time.Now())
	}

	return nil
}

// processEvents processes all queued events
func (a *AIAgent) processEvents(ctx context.Context) error {
	a.eventMu.Lock()
	events := make([]Event, len(a.eventQueue))
	copy(events, a.eventQueue)
	a.eventQueue = a.eventQueue[:0] // Clear the queue
	a.eventMu.Unlock()

	for _, event := range events {
		if handlers, exists := a.eventHandlers[event.Type]; exists {
			// Sort handlers by priority (higher priority first)
			sort.Slice(handlers, func(i, j int) bool {
				return handlers[i].GetPriority() > handlers[j].GetPriority()
			})

			for _, handler := range handlers {
				execCtx := &ExecutionContext{
					Context:    ctx,
					Blackboard: a.blackboard,
					Agent:      a,
				}

				if err := handler.Handle(execCtx, event); err != nil {
					return fmt.Errorf("event handler %s failed: %w", handler.GetEventType(), err)
				}
			}
		}

		// Store event in memory
		if a.memory != nil {
			memEvent := MemoryEvent{
				ID:         event.ID,
				Type:       event.Type,
				Timestamp:  event.Timestamp,
				Data:       event.Data,
				Importance: float64(event.Priority) / 10.0, // Convert priority to importance
				Tags:       []string{"event", event.Type},
			}

			a.memory.Remember(memEvent)
		}
	}

	return nil
}

// updateSensors updates all active sensors
func (a *AIAgent) updateSensors(ctx context.Context, deltaTime time.Duration) error {
	execCtx := &ExecutionContext{
		Context:    ctx,
		Blackboard: a.blackboard,
		Agent:      a,
		DeltaTime:  deltaTime,
	}

	for _, sensor := range a.sensors {
		if sensor.IsEnabled() {
			if err := sensor.Update(execCtx); err != nil {
				return fmt.Errorf("sensor %s update failed: %w", sensor.GetName(), err)
			}
		}
	}

	return nil
}

// HandleEvent processes an event
func (a *AIAgent) HandleEvent(event Event) error {
	a.eventMu.Lock()
	defer a.eventMu.Unlock()

	// Add event to queue
	a.eventQueue = append(a.eventQueue, event)

	return nil
}

// AddSensor adds a sensor to the agent
func (a *AIAgent) AddSensor(sensor Sensor) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.sensors[sensor.GetName()] = sensor
}

// RemoveSensor removes a sensor from the agent
func (a *AIAgent) RemoveSensor(sensorName string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, exists := a.sensors[sensorName]; exists {
		delete(a.sensors, sensorName)
		return true
	}
	return false
}

// GetSensors returns all sensors
func (a *AIAgent) GetSensors() []Sensor {
	a.mu.RLock()
	defer a.mu.RUnlock()

	sensors := make([]Sensor, 0, len(a.sensors))
	for _, sensor := range a.sensors {
		sensors = append(sensors, sensor)
	}
	return sensors
}

// AddEventHandler adds an event handler
func (a *AIAgent) AddEventHandler(handler EventHandler) {
	a.mu.Lock()
	defer a.mu.Unlock()

	eventType := handler.GetEventType()
	if a.eventHandlers[eventType] == nil {
		a.eventHandlers[eventType] = make([]EventHandler, 0)
	}

	a.eventHandlers[eventType] = append(a.eventHandlers[eventType], handler)
}

// RemoveEventHandler removes an event handler
func (a *AIAgent) RemoveEventHandler(eventType string, handler EventHandler) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	handlers, exists := a.eventHandlers[eventType]
	if !exists {
		return false
	}

	for i, h := range handlers {
		if h == handler {
			a.eventHandlers[eventType] = append(handlers[:i], handlers[i+1:]...)
			return true
		}
	}

	return false
}

// SetBehaviorTree sets the agent's behavior tree
func (a *AIAgent) SetBehaviorTree(root Node) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.behaviorTree = root
}

// GetBehaviorTree returns the agent's behavior tree
func (a *AIAgent) GetBehaviorTree() Node {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.behaviorTree
}

// SetUpdateRate sets the agent's update rate
func (a *AIAgent) SetUpdateRate(rate time.Duration) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.updateRate = rate
}

// GetUpdateRate returns the agent's update rate
func (a *AIAgent) GetUpdateRate() time.Duration {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.updateRate
}

// Save exports the agent's state
func (a *AIAgent) Save() ([]byte, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// Export blackboard
	blackboardData, err := a.blackboard.ToJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to export blackboard: %w", err)
	}

	// Export memory
	var memoryData []byte
	if a.memory != nil {
		memoryData, err = a.memory.Save()
		if err != nil {
			return nil, fmt.Errorf("failed to export memory: %w", err)
		}
	}

	// Create agent state
	state := struct {
		ID         string          `json:"id"`
		Name       string          `json:"name"`
		Active     bool            `json:"active"`
		UpdateRate time.Duration   `json:"update_rate"`
		LastUpdate time.Time       `json:"last_update"`
		Blackboard json.RawMessage `json:"blackboard"`
		Memory     json.RawMessage `json:"memory,omitempty"`
	}{
		ID:         a.id,
		Name:       a.name,
		Active:     a.active,
		UpdateRate: a.updateRate,
		LastUpdate: a.lastUpdate,
		Blackboard: blackboardData,
		Memory:     memoryData,
	}

	return json.MarshalIndent(state, "", "  ")
}

// Load imports the agent's state
func (a *AIAgent) Load(data []byte) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	var state struct {
		ID         string          `json:"id"`
		Name       string          `json:"name"`
		Active     bool            `json:"active"`
		UpdateRate time.Duration   `json:"update_rate"`
		LastUpdate time.Time       `json:"last_update"`
		Blackboard json.RawMessage `json:"blackboard"`
		Memory     json.RawMessage `json:"memory,omitempty"`
	}

	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to unmarshal agent state: %w", err)
	}

	// Load basic properties
	a.id = state.ID
	a.name = state.Name
	a.active = state.Active
	a.updateRate = state.UpdateRate
	a.lastUpdate = state.LastUpdate

	// Load blackboard
	if err := a.blackboard.FromJSON(state.Blackboard); err != nil {
		return fmt.Errorf("failed to load blackboard: %w", err)
	}

	// Load memory
	if len(state.Memory) > 0 && a.memory != nil {
		if err := a.memory.Load(state.Memory); err != nil {
			return fmt.Errorf("failed to load memory: %w", err)
		}
	}

	return nil
}

// Reset resets the agent to initial state
func (a *AIAgent) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Clear blackboard
	a.blackboard.Clear()

	// Clear memory
	if a.memory != nil {
		a.memory.Clear()
	}

	// Reset behavior tree
	if a.behaviorTree != nil {
		a.behaviorTree.Reset()
	}

	// Clear event queue
	a.eventMu.Lock()
	a.eventQueue = a.eventQueue[:0]
	a.eventMu.Unlock()

	// Reset timestamps
	a.lastUpdate = time.Time{}
}

// IsActive returns whether the agent is active
func (a *AIAgent) IsActive() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.active
}

// SetActive sets the agent's active state
func (a *AIAgent) SetActive(active bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.active = active
}

// GetStats returns agent statistics
func (a *AIAgent) GetStats() map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	stats := map[string]interface{}{
		"id":                 a.id,
		"name":               a.name,
		"active":             a.active,
		"update_rate":        a.updateRate.String(),
		"last_update":        a.lastUpdate,
		"sensor_count":       len(a.sensors),
		"handler_count":      len(a.eventHandlers),
		"blackboard_keys":    len(a.blackboard.Keys()),
		"blackboard_version": a.blackboard.GetVersion(),
	}

	// Add event queue stats
	a.eventMu.Lock()
	stats["queued_events"] = len(a.eventQueue)
	a.eventMu.Unlock()

	return stats
}
