package npc

import (
	"fmt"
	"time"
)

// BasicEventHandler provides a basic implementation of EventHandler
type BasicEventHandler struct {
	name      string
	eventType string
	priority  int
	enabled   bool
	handler   func(*ExecutionContext, Event) error
}

// NewBasicEventHandler creates a new basic event handler
func NewBasicEventHandler(name, eventType string, priority int, handler func(*ExecutionContext, Event) error) *BasicEventHandler {
	return &BasicEventHandler{
		name:      name,
		eventType: eventType,
		priority:  priority,
		enabled:   true,
		handler:   handler,
	}
}

// GetEventType returns the type of events this handler processes
func (beh *BasicEventHandler) GetEventType() string {
	return beh.eventType
}

// Handle processes an event
func (beh *BasicEventHandler) Handle(ctx *ExecutionContext, event Event) error {
	if !beh.enabled {
		return nil
	}

	if beh.handler != nil {
		return beh.handler(ctx, event)
	}

	// Default handling - just log the event
	fmt.Printf("[%s] Event handler %s processed event %s\n",
		time.Now().Format("15:04:05"), beh.name, event.Type)

	return nil
}

// GetPriority returns the handler priority
func (beh *BasicEventHandler) GetPriority() int {
	return beh.priority
}

// SetEnabled enables or disables the handler
func (beh *BasicEventHandler) SetEnabled(enabled bool) {
	beh.enabled = enabled
}

// IsEnabled returns whether the handler is enabled
func (beh *BasicEventHandler) IsEnabled() bool {
	return beh.enabled
}

// DamageEventHandler handles damage events
type DamageEventHandler struct {
	*BasicEventHandler
	healthKey string
}

// NewDamageEventHandler creates a new damage event handler
func NewDamageEventHandler(name, healthKey string, priority int) *DamageEventHandler {
	handler := &DamageEventHandler{
		BasicEventHandler: &BasicEventHandler{
			name:      name,
			eventType: "damage",
			priority:  priority,
			enabled:   true,
		},
		healthKey: healthKey,
	}

	handler.handler = handler.handleDamage
	return handler
}

// handleDamage processes damage events
func (deh *DamageEventHandler) handleDamage(ctx *ExecutionContext, event Event) error {
	// Get damage amount from event
	damageData, exists := event.Data["amount"]
	if !exists {
		return fmt.Errorf("damage event missing amount")
	}

	damage, ok := damageData.(float64)
	if !ok {
		return fmt.Errorf("invalid damage amount type")
	}

	// Get current health
	currentHealth, exists := ctx.Blackboard.GetFloat(deh.healthKey)
	if !exists {
		currentHealth = 100.0 // Default health
	}

	// Apply damage
	newHealth := currentHealth - damage
	if newHealth < 0 {
		newHealth = 0
	}

	ctx.Blackboard.Set(deh.healthKey, newHealth)

	// Store damage event in memory
	if ctx.Agent.GetMemory() != nil {
		memEvent := MemoryEvent{
			Type:      "damage_received",
			Timestamp: event.Timestamp,
			Data: map[string]interface{}{
				"damage":     damage,
				"old_health": currentHealth,
				"new_health": newHealth,
				"source":     event.Source,
			},
			Importance: damage / 10.0, // Higher damage = more important
			Tags:       []string{"combat", "damage", "health"},
		}

		ctx.Agent.GetMemory().Remember(memEvent)
	}

	// Trigger low health event if necessary
	if newHealth <= 20.0 && currentHealth > 20.0 {
		lowHealthEvent := Event{
			ID:        fmt.Sprintf("low_health_%d", time.Now().UnixNano()),
			Type:      "low_health",
			Source:    ctx.Agent.GetID(),
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"health": newHealth,
			},
			Priority: 8,
		}

		ctx.Agent.HandleEvent(lowHealthEvent)
	}

	fmt.Printf("[%s] Agent %s took %f damage, health: %f\n",
		time.Now().Format("15:04:05"), ctx.Agent.GetID(), damage, newHealth)

	return nil
}

// HealEventHandler handles healing events
type HealEventHandler struct {
	*BasicEventHandler
	healthKey    string
	maxHealthKey string
}

// NewHealEventHandler creates a new heal event handler
func NewHealEventHandler(name, healthKey, maxHealthKey string, priority int) *HealEventHandler {
	handler := &HealEventHandler{
		BasicEventHandler: &BasicEventHandler{
			name:      name,
			eventType: "heal",
			priority:  priority,
			enabled:   true,
		},
		healthKey:    healthKey,
		maxHealthKey: maxHealthKey,
	}

	handler.handler = handler.handleHeal
	return handler
}

// handleHeal processes heal events
func (heh *HealEventHandler) handleHeal(ctx *ExecutionContext, event Event) error {
	// Get heal amount from event
	healData, exists := event.Data["amount"]
	if !exists {
		return fmt.Errorf("heal event missing amount")
	}

	healAmount, ok := healData.(float64)
	if !ok {
		return fmt.Errorf("invalid heal amount type")
	}

	// Get current health
	currentHealth, exists := ctx.Blackboard.GetFloat(heh.healthKey)
	if !exists {
		currentHealth = 50.0 // Default health
	}

	// Get max health
	maxHealth, exists := ctx.Blackboard.GetFloat(heh.maxHealthKey)
	if !exists {
		maxHealth = 100.0 // Default max health
	}

	// Apply healing
	newHealth := currentHealth + healAmount
	if newHealth > maxHealth {
		newHealth = maxHealth
	}

	ctx.Blackboard.Set(heh.healthKey, newHealth)

	// Store heal event in memory
	if ctx.Agent.GetMemory() != nil {
		memEvent := MemoryEvent{
			Type:      "healing_received",
			Timestamp: event.Timestamp,
			Data: map[string]interface{}{
				"heal_amount": healAmount,
				"old_health":  currentHealth,
				"new_health":  newHealth,
				"source":      event.Source,
			},
			Importance: healAmount / 20.0,
			Tags:       []string{"healing", "health"},
		}

		ctx.Agent.GetMemory().Remember(memEvent)
	}

	fmt.Printf("[%s] Agent %s healed for %f, health: %f\n",
		time.Now().Format("15:04:05"), ctx.Agent.GetID(), healAmount, newHealth)

	return nil
}

// ItemEventHandler handles item-related events
type ItemEventHandler struct {
	*BasicEventHandler
	inventoryKey string
}

// NewItemEventHandler creates a new item event handler
func NewItemEventHandler(name, inventoryKey string, priority int) *ItemEventHandler {
	handler := &ItemEventHandler{
		BasicEventHandler: &BasicEventHandler{
			name:      name,
			eventType: "item",
			priority:  priority,
			enabled:   true,
		},
		inventoryKey: inventoryKey,
	}

	handler.handler = handler.handleItem
	return handler
}

// handleItem processes item events
func (ieh *ItemEventHandler) handleItem(ctx *ExecutionContext, event Event) error {
	action, exists := event.Data["action"]
	if !exists {
		return fmt.Errorf("item event missing action")
	}

	actionStr, ok := action.(string)
	if !ok {
		return fmt.Errorf("invalid action type")
	}

	itemType, exists := event.Data["item_type"]
	if !exists {
		return fmt.Errorf("item event missing item_type")
	}

	itemTypeStr, ok := itemType.(string)
	if !ok {
		return fmt.Errorf("invalid item_type")
	}

	quantity := 1
	if quantityData, exists := event.Data["quantity"]; exists {
		if q, ok := quantityData.(int); ok {
			quantity = q
		}
	}

	// Get current inventory
	inventoryData, exists := ctx.Blackboard.Get(ieh.inventoryKey)
	var inventory map[string]int

	if exists {
		if inv, ok := inventoryData.(map[string]int); ok {
			inventory = inv
		} else {
			inventory = make(map[string]int)
		}
	} else {
		inventory = make(map[string]int)
	}

	// Process action
	switch actionStr {
	case "add", "pickup":
		inventory[itemTypeStr] += quantity
		fmt.Printf("[%s] Agent %s picked up %d %s\n",
			time.Now().Format("15:04:05"), ctx.Agent.GetID(), quantity, itemTypeStr)

	case "remove", "use", "drop":
		if inventory[itemTypeStr] >= quantity {
			inventory[itemTypeStr] -= quantity
			if inventory[itemTypeStr] <= 0 {
				delete(inventory, itemTypeStr)
			}
			fmt.Printf("[%s] Agent %s used/dropped %d %s\n",
				time.Now().Format("15:04:05"), ctx.Agent.GetID(), quantity, itemTypeStr)
		} else {
			return fmt.Errorf("insufficient %s in inventory", itemTypeStr)
		}

	default:
		return fmt.Errorf("unknown item action: %s", actionStr)
	}

	// Update inventory in blackboard
	ctx.Blackboard.Set(ieh.inventoryKey, inventory)

	// Store item event in memory
	if ctx.Agent.GetMemory() != nil {
		memEvent := MemoryEvent{
			Type:      "item_" + actionStr,
			Timestamp: event.Timestamp,
			Data: map[string]interface{}{
				"item_type": itemTypeStr,
				"quantity":  quantity,
				"action":    actionStr,
			},
			Importance: 0.3,
			Tags:       []string{"inventory", "item", actionStr},
		}

		ctx.Agent.GetMemory().Remember(memEvent)
	}

	return nil
}

// CombatEventHandler handles combat-related events
type CombatEventHandler struct {
	*BasicEventHandler
	combatStateKey string
}

// NewCombatEventHandler creates a new combat event handler
func NewCombatEventHandler(name, combatStateKey string, priority int) *CombatEventHandler {
	handler := &CombatEventHandler{
		BasicEventHandler: &BasicEventHandler{
			name:      name,
			eventType: "combat",
			priority:  priority,
			enabled:   true,
		},
		combatStateKey: combatStateKey,
	}

	handler.handler = handler.handleCombat
	return handler
}

// handleCombat processes combat events
func (ceh *CombatEventHandler) handleCombat(ctx *ExecutionContext, event Event) error {
	action, exists := event.Data["action"]
	if !exists {
		return fmt.Errorf("combat event missing action")
	}

	actionStr, ok := action.(string)
	if !ok {
		return fmt.Errorf("invalid action type")
	}

	switch actionStr {
	case "enter_combat":
		ctx.Blackboard.Set(ceh.combatStateKey, true)
		ctx.Blackboard.Set("combat_start_time", time.Now())

		if target, exists := event.Data["target"]; exists {
			ctx.Blackboard.Set("combat_target", target)
		}

		fmt.Printf("[%s] Agent %s entered combat\n",
			time.Now().Format("15:04:05"), ctx.Agent.GetID())

	case "exit_combat":
		ctx.Blackboard.Set(ceh.combatStateKey, false)
		ctx.Blackboard.Delete("combat_target")

		if startTime, exists := ctx.Blackboard.Get("combat_start_time"); exists {
			if start, ok := startTime.(time.Time); ok {
				duration := time.Since(start)
				ctx.Blackboard.Set("last_combat_duration", duration)
			}
		}

		fmt.Printf("[%s] Agent %s exited combat\n",
			time.Now().Format("15:04:05"), ctx.Agent.GetID())

	case "attack":
		if target, exists := event.Data["target"]; exists {
			ctx.Blackboard.Set("last_attack_target", target)
			ctx.Blackboard.Set("last_attack_time", time.Now())
		}

	default:
		return fmt.Errorf("unknown combat action: %s", actionStr)
	}

	// Store combat event in memory
	if ctx.Agent.GetMemory() != nil {
		memEvent := MemoryEvent{
			Type:       "combat_" + actionStr,
			Timestamp:  event.Timestamp,
			Data:       event.Data,
			Importance: 0.7, // Combat events are important
			Tags:       []string{"combat", actionStr},
		}

		ctx.Agent.GetMemory().Remember(memEvent)
	}

	return nil
}

// EnvironmentEventHandler handles environment change events
type EnvironmentEventHandler struct {
	*BasicEventHandler
	environmentKey string
}

// NewEnvironmentEventHandler creates a new environment event handler
func NewEnvironmentEventHandler(name, environmentKey string, priority int) *EnvironmentEventHandler {
	handler := &EnvironmentEventHandler{
		BasicEventHandler: &BasicEventHandler{
			name:      name,
			eventType: "environment",
			priority:  priority,
			enabled:   true,
		},
		environmentKey: environmentKey,
	}

	handler.handler = handler.handleEnvironment
	return handler
}

// handleEnvironment processes environment events
func (eeh *EnvironmentEventHandler) handleEnvironment(ctx *ExecutionContext, event Event) error {
	// Get current environment data
	envData, exists := ctx.Blackboard.Get(eeh.environmentKey)
	var environment map[string]interface{}

	if exists {
		if env, ok := envData.(map[string]interface{}); ok {
			environment = env
		} else {
			environment = make(map[string]interface{})
		}
	} else {
		environment = make(map[string]interface{})
	}

	// Update environment with event data
	for key, value := range event.Data {
		environment[key] = value
	}

	ctx.Blackboard.Set(eeh.environmentKey, environment)

	// Store environment change in memory
	if ctx.Agent.GetMemory() != nil {
		memEvent := MemoryEvent{
			Type:       "environment_change",
			Timestamp:  event.Timestamp,
			Data:       event.Data,
			Importance: 0.4,
			Tags:       []string{"environment", "change"},
		}

		ctx.Agent.GetMemory().Remember(memEvent)
	}

	fmt.Printf("[%s] Agent %s environment updated: %v\n",
		time.Now().Format("15:04:05"), ctx.Agent.GetID(), event.Data)

	return nil
}

// CreateEvent creates a new event with a unique ID
func CreateEvent(eventType, source, target string, data map[string]interface{}, priority int) Event {
	return Event{
		ID:        fmt.Sprintf("%s_%d", eventType, time.Now().UnixNano()),
		Type:      eventType,
		Source:    source,
		Target:    target,
		Timestamp: time.Now(),
		Data:      data,
		Priority:  priority,
	}
}

// CreateDamageEvent creates a damage event
func CreateDamageEvent(source, target string, damage float64) Event {
	return CreateEvent("damage", source, target, map[string]interface{}{
		"amount": damage,
	}, 7)
}

// CreateHealEvent creates a heal event
func CreateHealEvent(source, target string, healAmount float64) Event {
	return CreateEvent("heal", source, target, map[string]interface{}{
		"amount": healAmount,
	}, 5)
}

// CreateItemEvent creates an item event
func CreateItemEvent(source, target, action, itemType string, quantity int) Event {
	return CreateEvent("item", source, target, map[string]interface{}{
		"action":    action,
		"item_type": itemType,
		"quantity":  quantity,
	}, 3)
}

// CreateCombatEvent creates a combat event
func CreateCombatEvent(source, target, action string, data map[string]interface{}) Event {
	eventData := map[string]interface{}{
		"action": action,
	}

	// Merge additional data
	for key, value := range data {
		eventData[key] = value
	}

	return CreateEvent("combat", source, target, eventData, 6)
}

// CreateEnvironmentEvent creates an environment event
func CreateEnvironmentEvent(source string, changes map[string]interface{}) Event {
	return CreateEvent("environment", source, "*", changes, 4)
}
