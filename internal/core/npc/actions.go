package npc

import (
	"fmt"
	"time"
)

// WaitActionNode represents an action that waits for a specified duration
type WaitActionNode struct {
	*BaseNode
	duration  time.Duration
	startTime time.Time
	isWaiting bool
}

// NewWaitActionNode creates a new wait action node
func NewWaitActionNode(name string, duration time.Duration) *WaitActionNode {
	return &WaitActionNode{
		BaseNode: NewBaseNode(name, "WaitAction"),
		duration: duration,
	}
}

// Execute runs the wait action
func (wan *WaitActionNode) Execute(ctx *ExecutionContext) NodeStatus {
	if !wan.isWaiting {
		// Start waiting
		wan.startTime = time.Now()
		wan.isWaiting = true
		wan.SetStatus(StatusRunning)
		return StatusRunning
	}

	// Check if wait time has elapsed
	if time.Since(wan.startTime) >= wan.duration {
		wan.isWaiting = false
		wan.SetStatus(StatusSuccess)
		return StatusSuccess
	}

	wan.SetStatus(StatusRunning)
	return StatusRunning
}

// Reset resets the wait action
func (wan *WaitActionNode) Reset() {
	wan.BaseNode.Reset()
	wan.isWaiting = false
	wan.startTime = time.Time{}
}

// CanExecute checks if the action can be executed
func (wan *WaitActionNode) CanExecute(ctx *ExecutionContext) bool {
	return true
}

// Clone creates a copy of the wait action node
func (wan *WaitActionNode) Clone() Node {
	return NewWaitActionNode(wan.name, wan.duration)
}

// LogActionNode represents an action that logs a message
type LogActionNode struct {
	*BaseNode
	message string
}

// NewLogActionNode creates a new log action node
func NewLogActionNode(name, message string) *LogActionNode {
	return &LogActionNode{
		BaseNode: NewBaseNode(name, "LogAction"),
		message:  message,
	}
}

// Execute runs the log action
func (lan *LogActionNode) Execute(ctx *ExecutionContext) NodeStatus {
	// In a real implementation, this would use a proper logging system
	fmt.Printf("[%s] %s: %s\n", time.Now().Format("15:04:05"), lan.name, lan.message)

	// Store log message in blackboard for debugging
	ctx.Blackboard.Set("last_log_message", lan.message)
	ctx.Blackboard.Set("last_log_time", time.Now())

	lan.SetStatus(StatusSuccess)
	return StatusSuccess
}

// CanExecute checks if the action can be executed
func (lan *LogActionNode) CanExecute(ctx *ExecutionContext) bool {
	return true
}

// Clone creates a copy of the log action node
func (lan *LogActionNode) Clone() Node {
	return NewLogActionNode(lan.name, lan.message)
}

// SetBlackboardActionNode sets a value in the blackboard
type SetBlackboardActionNode struct {
	*BaseNode
	key   string
	value interface{}
}

// NewSetBlackboardActionNode creates a new set blackboard action node
func NewSetBlackboardActionNode(name, key string, value interface{}) *SetBlackboardActionNode {
	return &SetBlackboardActionNode{
		BaseNode: NewBaseNode(name, "SetBlackboardAction"),
		key:      key,
		value:    value,
	}
}

// Execute runs the set blackboard action
func (sban *SetBlackboardActionNode) Execute(ctx *ExecutionContext) NodeStatus {
	ctx.Blackboard.Set(sban.key, sban.value)
	sban.SetStatus(StatusSuccess)
	return StatusSuccess
}

// CanExecute checks if the action can be executed
func (sban *SetBlackboardActionNode) CanExecute(ctx *ExecutionContext) bool {
	return true
}

// Clone creates a copy of the set blackboard action node
func (sban *SetBlackboardActionNode) Clone() Node {
	return NewSetBlackboardActionNode(sban.name, sban.key, sban.value)
}

// MoveToActionNode represents a movement action
type MoveToActionNode struct {
	*BaseNode
	targetKey    string
	positionKey  string
	speedKey     string
	thresholdKey string
	isMoving     bool
}

// NewMoveToActionNode creates a new move to action node
func NewMoveToActionNode(name, targetKey, positionKey, speedKey, thresholdKey string) *MoveToActionNode {
	return &MoveToActionNode{
		BaseNode:     NewBaseNode(name, "MoveToAction"),
		targetKey:    targetKey,
		positionKey:  positionKey,
		speedKey:     speedKey,
		thresholdKey: thresholdKey,
	}
}

// Execute runs the move to action
func (mtan *MoveToActionNode) Execute(ctx *ExecutionContext) NodeStatus {
	// Get current position
	currentPosData, exists := ctx.Blackboard.Get(mtan.positionKey)
	if !exists {
		mtan.SetStatus(StatusFailure)
		return StatusFailure
	}

	currentPos, ok := currentPosData.(Position)
	if !ok {
		mtan.SetStatus(StatusFailure)
		return StatusFailure
	}

	// Get target position
	targetPosData, exists := ctx.Blackboard.Get(mtan.targetKey)
	if !exists {
		mtan.SetStatus(StatusFailure)
		return StatusFailure
	}

	targetPos, ok := targetPosData.(Position)
	if !ok {
		mtan.SetStatus(StatusFailure)
		return StatusFailure
	}

	// Get movement speed
	speed, exists := ctx.Blackboard.GetFloat(mtan.speedKey)
	if !exists {
		speed = 1.0 // Default speed
	}

	// Get distance threshold
	threshold, exists := ctx.Blackboard.GetFloat(mtan.thresholdKey)
	if !exists {
		threshold = 0.1 // Default threshold
	}

	// Calculate distance to target
	distance := calculateDistance(currentPos, targetPos)

	// Check if we've reached the target
	if distance <= threshold {
		mtan.isMoving = false
		mtan.SetStatus(StatusSuccess)
		return StatusSuccess
	}

	// Calculate movement direction
	deltaTime := ctx.DeltaTime.Seconds()
	moveDistance := speed * deltaTime

	if moveDistance >= distance {
		// We can reach the target in this frame
		ctx.Blackboard.Set(mtan.positionKey, targetPos)
		mtan.isMoving = false
		mtan.SetStatus(StatusSuccess)
		return StatusSuccess
	}

	// Move towards target
	ratio := moveDistance / distance
	newPos := Position{
		X: currentPos.X + (targetPos.X-currentPos.X)*ratio,
		Y: currentPos.Y + (targetPos.Y-currentPos.Y)*ratio,
		Z: currentPos.Z + (targetPos.Z-currentPos.Z)*ratio,
	}

	ctx.Blackboard.Set(mtan.positionKey, newPos)
	mtan.isMoving = true
	mtan.SetStatus(StatusRunning)
	return StatusRunning
}

// Reset resets the move to action
func (mtan *MoveToActionNode) Reset() {
	mtan.BaseNode.Reset()
	mtan.isMoving = false
}

// CanExecute checks if the action can be executed
func (mtan *MoveToActionNode) CanExecute(ctx *ExecutionContext) bool {
	return ctx.Blackboard.Has(mtan.positionKey) && ctx.Blackboard.Has(mtan.targetKey)
}

// Clone creates a copy of the move to action node
func (mtan *MoveToActionNode) Clone() Node {
	return NewMoveToActionNode(mtan.name, mtan.targetKey, mtan.positionKey, mtan.speedKey, mtan.thresholdKey)
}

// AttackActionNode represents an attack action
type AttackActionNode struct {
	*BaseNode
	targetKey     string
	damageKey     string
	rangeKey      string
	cooldownKey   string
	lastAttackKey string
}

// NewAttackActionNode creates a new attack action node
func NewAttackActionNode(name, targetKey, damageKey, rangeKey, cooldownKey, lastAttackKey string) *AttackActionNode {
	return &AttackActionNode{
		BaseNode:      NewBaseNode(name, "AttackAction"),
		targetKey:     targetKey,
		damageKey:     damageKey,
		rangeKey:      rangeKey,
		cooldownKey:   cooldownKey,
		lastAttackKey: lastAttackKey,
	}
}

// Execute runs the attack action
func (aan *AttackActionNode) Execute(ctx *ExecutionContext) NodeStatus {
	// Check cooldown
	if lastAttackTime, exists := ctx.Blackboard.Get(aan.lastAttackKey); exists {
		if lastTime, ok := lastAttackTime.(time.Time); ok {
			cooldown, exists := ctx.Blackboard.GetFloat(aan.cooldownKey)
			if !exists {
				cooldown = 1.0 // Default 1 second cooldown
			}

			if time.Since(lastTime).Seconds() < cooldown {
				aan.SetStatus(StatusFailure)
				return StatusFailure
			}
		}
	}

	// Get target
	target, exists := ctx.Blackboard.Get(aan.targetKey)
	if !exists {
		aan.SetStatus(StatusFailure)
		return StatusFailure
	}

	// Get damage
	damage, exists := ctx.Blackboard.GetFloat(aan.damageKey)
	if !exists {
		damage = 10.0 // Default damage
	}

	// Perform attack (simplified)
	ctx.Blackboard.Set("attack_target", target)
	ctx.Blackboard.Set("attack_damage", damage)
	ctx.Blackboard.Set(aan.lastAttackKey, time.Now())

	// Log attack
	fmt.Printf("[%s] %s: Attacking target with %f damage\n",
		time.Now().Format("15:04:05"), aan.name, damage)

	aan.SetStatus(StatusSuccess)
	return StatusSuccess
}

// CanExecute checks if the action can be executed
func (aan *AttackActionNode) CanExecute(ctx *ExecutionContext) bool {
	return ctx.Blackboard.Has(aan.targetKey)
}

// Clone creates a copy of the attack action node
func (aan *AttackActionNode) Clone() Node {
	return NewAttackActionNode(aan.name, aan.targetKey, aan.damageKey, aan.rangeKey, aan.cooldownKey, aan.lastAttackKey)
}

// PatrolActionNode represents a patrol action
type PatrolActionNode struct {
	*BaseNode
	waypointsKey    string
	positionKey     string
	speedKey        string
	currentWaypoint int
	isPatrolling    bool
}

// NewPatrolActionNode creates a new patrol action node
func NewPatrolActionNode(name, waypointsKey, positionKey, speedKey string) *PatrolActionNode {
	return &PatrolActionNode{
		BaseNode:        NewBaseNode(name, "PatrolAction"),
		waypointsKey:    waypointsKey,
		positionKey:     positionKey,
		speedKey:        speedKey,
		currentWaypoint: 0,
	}
}

// Execute runs the patrol action
func (pan *PatrolActionNode) Execute(ctx *ExecutionContext) NodeStatus {
	// Get waypoints
	waypointsData, exists := ctx.Blackboard.Get(pan.waypointsKey)
	if !exists {
		pan.SetStatus(StatusFailure)
		return StatusFailure
	}

	waypoints, ok := waypointsData.([]Position)
	if !ok {
		pan.SetStatus(StatusFailure)
		return StatusFailure
	}

	if len(waypoints) == 0 {
		pan.SetStatus(StatusFailure)
		return StatusFailure
	}

	// Get current position
	currentPosData, exists := ctx.Blackboard.Get(pan.positionKey)
	if !exists {
		pan.SetStatus(StatusFailure)
		return StatusFailure
	}

	currentPos, ok := currentPosData.(Position)
	if !ok {
		pan.SetStatus(StatusFailure)
		return StatusFailure
	}

	// Get target waypoint
	if pan.currentWaypoint >= len(waypoints) {
		pan.currentWaypoint = 0
	}

	targetPos := waypoints[pan.currentWaypoint]

	// Check if we've reached the current waypoint
	distance := calculateDistance(currentPos, targetPos)
	if distance <= 0.1 { // Threshold
		// Move to next waypoint
		pan.currentWaypoint = (pan.currentWaypoint + 1) % len(waypoints)
		pan.SetStatus(StatusRunning)
		return StatusRunning
	}

	// Move towards current waypoint
	speed, exists := ctx.Blackboard.GetFloat(pan.speedKey)
	if !exists {
		speed = 1.0
	}

	deltaTime := ctx.DeltaTime.Seconds()
	moveDistance := speed * deltaTime

	if moveDistance >= distance {
		// Reach waypoint this frame
		ctx.Blackboard.Set(pan.positionKey, targetPos)
	} else {
		// Move towards waypoint
		ratio := moveDistance / distance
		newPos := Position{
			X: currentPos.X + (targetPos.X-currentPos.X)*ratio,
			Y: currentPos.Y + (targetPos.Y-currentPos.Y)*ratio,
			Z: currentPos.Z + (targetPos.Z-currentPos.Z)*ratio,
		}
		ctx.Blackboard.Set(pan.positionKey, newPos)
	}

	pan.isPatrolling = true
	pan.SetStatus(StatusRunning)
	return StatusRunning
}

// Reset resets the patrol action
func (pan *PatrolActionNode) Reset() {
	pan.BaseNode.Reset()
	pan.isPatrolling = false
	// Don't reset currentWaypoint to maintain patrol state
}

// CanExecute checks if the action can be executed
func (pan *PatrolActionNode) CanExecute(ctx *ExecutionContext) bool {
	return ctx.Blackboard.Has(pan.waypointsKey) && ctx.Blackboard.Has(pan.positionKey)
}

// Clone creates a copy of the patrol action node
func (pan *PatrolActionNode) Clone() Node {
	return NewPatrolActionNode(pan.name, pan.waypointsKey, pan.positionKey, pan.speedKey)
}
