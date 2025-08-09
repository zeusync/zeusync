package npc

import (
	"fmt"
	"math"
	"time"
)

// BaseSensor provides common functionality for all sensors
type BaseSensor struct {
	name           string
	sensorType     string
	enabled        bool
	updateInterval time.Duration
	lastUpdate     time.Time
}

// NewBaseSensor creates a new base sensor
func NewBaseSensor(name, sensorType string, updateInterval time.Duration) *BaseSensor {
	return &BaseSensor{
		name:           name,
		sensorType:     sensorType,
		enabled:        true,
		updateInterval: updateInterval,
		lastUpdate:     time.Time{},
	}
}

// GetName returns the sensor name
func (bs *BaseSensor) GetName() string {
	return bs.name
}

// GetType returns the sensor type
func (bs *BaseSensor) GetType() string {
	return bs.sensorType
}

// IsEnabled returns whether the sensor is active
func (bs *BaseSensor) IsEnabled() bool {
	return bs.enabled
}

// SetEnabled enables or disables the sensor
func (bs *BaseSensor) SetEnabled(enabled bool) {
	bs.enabled = enabled
}

// GetUpdateInterval returns how often the sensor should update
func (bs *BaseSensor) GetUpdateInterval() time.Duration {
	return bs.updateInterval
}

// SetUpdateInterval sets the sensor update interval
func (bs *BaseSensor) SetUpdateInterval(interval time.Duration) {
	bs.updateInterval = interval
}

// ShouldUpdate checks if the sensor should update based on its interval
func (bs *BaseSensor) ShouldUpdate() bool {
	return time.Since(bs.lastUpdate) >= bs.updateInterval
}

// MarkUpdated marks the sensor as updated
func (bs *BaseSensor) MarkUpdated() {
	bs.lastUpdate = time.Now()
}

// DistanceSensor measures distance to targets
type DistanceSensor struct {
	*BaseSensor
	maxRange    float64
	targetKey   string
	positionKey string
	outputKey   string
}

// NewDistanceSensor creates a new distance sensor
func NewDistanceSensor(name string, maxRange float64, targetKey, positionKey, outputKey string) *DistanceSensor {
	return &DistanceSensor{
		BaseSensor:  NewBaseSensor(name, "Distance", time.Millisecond*100),
		maxRange:    maxRange,
		targetKey:   targetKey,
		positionKey: positionKey,
		outputKey:   outputKey,
	}
}

// Update updates the distance sensor
func (ds *DistanceSensor) Update(ctx *ExecutionContext) error {
	if !ds.ShouldUpdate() {
		return nil
	}

	defer ds.MarkUpdated()

	// Get agent position
	agentPos, exists := ctx.Blackboard.Get(ds.positionKey)
	if !exists {
		return fmt.Errorf("agent position not found in blackboard key: %s", ds.positionKey)
	}

	agentPosition, ok := agentPos.(Position)
	if !ok {
		return fmt.Errorf("invalid agent position type")
	}

	// Get target position
	targetPos, exists := ctx.Blackboard.Get(ds.targetKey)
	if !exists {
		ctx.Blackboard.Set(ds.outputKey, -1.0) // No target
		return nil
	}

	targetPosition, ok := targetPos.(Position)
	if !ok {
		return fmt.Errorf("invalid target position type")
	}

	// Calculate distance
	distance := calculateDistance(agentPosition, targetPosition)

	// Check if within range
	if distance <= ds.maxRange {
		ctx.Blackboard.Set(ds.outputKey, distance)
		ctx.Blackboard.Set(ds.outputKey+"_in_range", true)
	} else {
		ctx.Blackboard.Set(ds.outputKey, -1.0) // Out of range
		ctx.Blackboard.Set(ds.outputKey+"_in_range", false)
	}

	return nil
}

// HealthSensor monitors agent health
type HealthSensor struct {
	*BaseSensor
	healthKey    string
	maxHealthKey string
	outputKey    string
	lowThreshold float64
}

// NewHealthSensor creates a new health sensor
func NewHealthSensor(name, healthKey, maxHealthKey, outputKey string, lowThreshold float64) *HealthSensor {
	return &HealthSensor{
		BaseSensor:   NewBaseSensor(name, "Health", time.Millisecond*50),
		healthKey:    healthKey,
		maxHealthKey: maxHealthKey,
		outputKey:    outputKey,
		lowThreshold: lowThreshold,
	}
}

// Update updates the health sensor
func (hs *HealthSensor) Update(ctx *ExecutionContext) error {
	if !hs.ShouldUpdate() {
		return nil
	}

	defer hs.MarkUpdated()

	// Get current health
	currentHealth, exists := ctx.Blackboard.GetFloat(hs.healthKey)
	if !exists {
		return fmt.Errorf("current health not found in blackboard key: %s", hs.healthKey)
	}

	// Get max health
	maxHealth, exists := ctx.Blackboard.GetFloat(hs.maxHealthKey)
	if !exists {
		maxHealth = 100.0 // Default max health
	}

	// Calculate health percentage
	healthPercentage := currentHealth / maxHealth

	// Update blackboard
	ctx.Blackboard.Set(hs.outputKey+"_percentage", healthPercentage)
	ctx.Blackboard.Set(hs.outputKey+"_is_low", healthPercentage < hs.lowThreshold)
	ctx.Blackboard.Set(hs.outputKey+"_is_critical", healthPercentage < 0.1)
	ctx.Blackboard.Set(hs.outputKey+"_is_full", healthPercentage >= 1.0)

	return nil
}

// InventorySensor monitors inventory status
type InventorySensor struct {
	*BaseSensor
	inventoryKey string
	outputKey    string
	itemTypes    []string
}

// NewInventorySensor creates a new inventory sensor
func NewInventorySensor(name, inventoryKey, outputKey string, itemTypes []string) *InventorySensor {
	return &InventorySensor{
		BaseSensor:   NewBaseSensor(name, "Inventory", time.Millisecond*200),
		inventoryKey: inventoryKey,
		outputKey:    outputKey,
		itemTypes:    itemTypes,
	}
}

// Update updates the inventory sensor
func (is *InventorySensor) Update(ctx *ExecutionContext) error {
	if !is.ShouldUpdate() {
		return nil
	}

	defer is.MarkUpdated()

	// Get inventory data
	inventoryData, exists := ctx.Blackboard.Get(is.inventoryKey)
	if !exists {
		return fmt.Errorf("inventory not found in blackboard key: %s", is.inventoryKey)
	}

	inventory, ok := inventoryData.(map[string]int)
	if !ok {
		return fmt.Errorf("invalid inventory type")
	}

	// Check each item type
	for _, itemType := range is.itemTypes {
		count, exists := inventory[itemType]
		if !exists {
			count = 0
		}

		ctx.Blackboard.Set(fmt.Sprintf("%s_%s_count", is.outputKey, itemType), count)
		ctx.Blackboard.Set(fmt.Sprintf("%s_%s_available", is.outputKey, itemType), count > 0)
	}

	// Calculate total items
	totalItems := 0
	for _, count := range inventory {
		totalItems += count
	}

	ctx.Blackboard.Set(is.outputKey+"_total", totalItems)
	ctx.Blackboard.Set(is.outputKey+"_empty", totalItems == 0)

	return nil
}

// TimerSensor tracks time-based conditions
type TimerSensor struct {
	*BaseSensor
	timers    map[string]time.Time
	outputKey string
}

// NewTimerSensor creates a new timer sensor
func NewTimerSensor(name, outputKey string) *TimerSensor {
	return &TimerSensor{
		BaseSensor: NewBaseSensor(name, "Timer", time.Millisecond*10),
		timers:     make(map[string]time.Time),
		outputKey:  outputKey,
	}
}

// Update updates the timer sensor
func (ts *TimerSensor) Update(ctx *ExecutionContext) error {
	if !ts.ShouldUpdate() {
		return nil
	}

	defer ts.MarkUpdated()

	now := time.Now()

	// Update all timers
	for timerName, startTime := range ts.timers {
		elapsed := now.Sub(startTime)
		ctx.Blackboard.Set(fmt.Sprintf("%s_%s_elapsed", ts.outputKey, timerName), elapsed)
		ctx.Blackboard.Set(fmt.Sprintf("%s_%s_elapsed_seconds", ts.outputKey, timerName), elapsed.Seconds())
	}

	return nil
}

// StartTimer starts a named timer
func (ts *TimerSensor) StartTimer(name string) {
	ts.timers[name] = time.Now()
}

// StopTimer stops and removes a named timer
func (ts *TimerSensor) StopTimer(name string) time.Duration {
	if startTime, exists := ts.timers[name]; exists {
		elapsed := time.Since(startTime)
		delete(ts.timers, name)
		return elapsed
	}
	return 0
}

// GetElapsed returns the elapsed time for a timer
func (ts *TimerSensor) GetElapsed(name string) time.Duration {
	if startTime, exists := ts.timers[name]; exists {
		return time.Since(startTime)
	}
	return 0
}

// EnvironmentSensor monitors environmental conditions
type EnvironmentSensor struct {
	*BaseSensor
	environmentKey string
	outputKey      string
	conditions     []string
}

// NewEnvironmentSensor creates a new environment sensor
func NewEnvironmentSensor(name, environmentKey, outputKey string, conditions []string) *EnvironmentSensor {
	return &EnvironmentSensor{
		BaseSensor:     NewBaseSensor(name, "Environment", time.Millisecond*500),
		environmentKey: environmentKey,
		outputKey:      outputKey,
		conditions:     conditions,
	}
}

// Update updates the environment sensor
func (es *EnvironmentSensor) Update(ctx *ExecutionContext) error {
	if !es.ShouldUpdate() {
		return nil
	}

	defer es.MarkUpdated()

	// Get environment data
	envData, exists := ctx.Blackboard.Get(es.environmentKey)
	if !exists {
		return fmt.Errorf("environment data not found in blackboard key: %s", es.environmentKey)
	}

	environment, ok := envData.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid environment data type")
	}

	// Check each condition
	for _, condition := range es.conditions {
		value, exists := environment[condition]
		if exists {
			ctx.Blackboard.Set(fmt.Sprintf("%s_%s", es.outputKey, condition), value)
		}
	}

	return nil
}

// Position represents a 2D or 3D position
type Position struct {
	X, Y, Z float64
}

// calculateDistance calculates the Euclidean distance between two positions
func calculateDistance(pos1, pos2 Position) float64 {
	dx := pos1.X - pos2.X
	dy := pos1.Y - pos2.Y
	dz := pos1.Z - pos2.Z

	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

// ProximitySensor detects nearby entities
type ProximitySensor struct {
	*BaseSensor
	positionKey string
	entitiesKey string
	outputKey   string
	maxRange    float64
	entityTypes []string
}

// NewProximitySensor creates a new proximity sensor
func NewProximitySensor(name, positionKey, entitiesKey, outputKey string, maxRange float64, entityTypes []string) *ProximitySensor {
	return &ProximitySensor{
		BaseSensor:  NewBaseSensor(name, "Proximity", time.Millisecond*100),
		positionKey: positionKey,
		entitiesKey: entitiesKey,
		outputKey:   outputKey,
		maxRange:    maxRange,
		entityTypes: entityTypes,
	}
}

// Update updates the proximity sensor
func (ps *ProximitySensor) Update(ctx *ExecutionContext) error {
	if !ps.ShouldUpdate() {
		return nil
	}

	defer ps.MarkUpdated()

	// Get agent position
	agentPos, exists := ctx.Blackboard.Get(ps.positionKey)
	if !exists {
		return fmt.Errorf("agent position not found in blackboard key: %s", ps.positionKey)
	}

	agentPosition, ok := agentPos.(Position)
	if !ok {
		return fmt.Errorf("invalid agent position type")
	}

	// Get entities
	entitiesData, exists := ctx.Blackboard.Get(ps.entitiesKey)
	if !exists {
		ctx.Blackboard.Set(ps.outputKey+"_nearby_count", 0)
		ctx.Blackboard.Set(ps.outputKey+"_nearby_entities", []interface{}{})
		return nil
	}

	entities, ok := entitiesData.([]interface{})
	if !ok {
		return fmt.Errorf("invalid entities data type")
	}

	nearbyEntities := make([]interface{}, 0)
	entityCounts := make(map[string]int)

	// Check each entity
	for _, entityData := range entities {
		entity, ok := entityData.(map[string]interface{})
		if !ok {
			continue
		}

		// Get entity position
		entityPosData, exists := entity["position"]
		if !exists {
			continue
		}

		entityPosition, ok := entityPosData.(Position)
		if !ok {
			continue
		}

		// Calculate distance
		distance := calculateDistance(agentPosition, entityPosition)

		if distance <= ps.maxRange {
			nearbyEntities = append(nearbyEntities, entity)

			// Count by type if specified
			if len(ps.entityTypes) > 0 {
				if entityType, exists := entity["type"].(string); exists {
					for _, targetType := range ps.entityTypes {
						if entityType == targetType {
							entityCounts[targetType]++
							break
						}
					}
				}
			}
		}
	}

	// Update blackboard
	ctx.Blackboard.Set(ps.outputKey+"_nearby_count", len(nearbyEntities))
	ctx.Blackboard.Set(ps.outputKey+"_nearby_entities", nearbyEntities)

	// Update type-specific counts
	for _, entityType := range ps.entityTypes {
		count := entityCounts[entityType]
		ctx.Blackboard.Set(fmt.Sprintf("%s_%s_count", ps.outputKey, entityType), count)
		ctx.Blackboard.Set(fmt.Sprintf("%s_%s_detected", ps.outputKey, entityType), count > 0)
	}

	return nil
}
