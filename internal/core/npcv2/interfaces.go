package npcv2

import (
	"context"
	"encoding/json"
	"time"

	bus "github.com/zeusync/zeusync/internal/core/events/bus"
)

// Status represents the execution result of a behavior node tick.
// It allows the tree to control flow and asynchronous-like waiting.
type Status int

const (
	StatusSuccess Status = iota
	StatusFailure
	StatusRunning
)

// Blackboard is a centralized, thread-safe storage for agent state and shared data.
// It supports namespaced keys, typed access, and persistence.
type Blackboard interface {
	// Get retrieves a value by key. Returns (nil, false) if absent.
	Get(key string) (any, bool)
	// Set assigns a value by key.
	Set(key string, value any)
	// Delete removes a value by key.
	Delete(key string)
	// Namespace returns a namespaced view of the blackboard using prefix"ns:" semantics.
	Namespace(ns string) Blackboard
	// Keys returns a snapshot of existing keys.
	Keys() []string
	// MarshalBinary persists the current state into compact binary bytes (default codec: gob).
	MarshalBinary() ([]byte, error)
	// UnmarshalBinary restores the state from compact binary bytes.
	UnmarshalBinary(b []byte) error
}

// Memory stores decisions history and provides persistence for agent memory between runs.
type Memory interface {
	// AppendDecision appends a decision record.
	AppendDecision(rec DecisionRecord)
	// History returns a copy snapshot of the decision history.
	History() []DecisionRecord
	// Reset clears all history.
	Reset()
	// Save serializes memory to a compact binary representation (default codec: gob).
	Save() ([]byte, error)
	// Load de-serializes memory from a compact binary representation.
	Load(b []byte) error
}

// TickContext is a context passed into nodes during Tick.
// It includes standard context plus shortcuts to blackboard and time.
type TickContext struct {
	Ctx    context.Context
	BB     Blackboard
	Memory Memory
	Clock  func() time.Time
}

// BehaviorNode is the fundamental interface for behavior tree nodes.
// Implementations must be stateless w.r.t. shared instances or keep per-agent state in Blackboard/Memory.
type BehaviorNode interface {
	// Tick executes one step of the node and returns a Status.
	// Implementations should avoid panics and return errors via blackboard or wrapping actions if needed.
	Tick(t TickContext) (Status, error)
	// Name returns a human-readable name for debugging and history.
	Name() string
}

// Action performs side effects and returns status based on Blackboard state.
type Action interface {
	BehaviorNode
}

// Condition evaluates to success/failure based on Blackboard state.
type Condition interface {
	BehaviorNode
}

// Decorator wraps a single child node and changes behavior (repeaters, timers, probability, etc.).
type Decorator interface {
	BehaviorNode
	SetChild(child BehaviorNode)
}

// Composite node manages multiple children (sequence, selector, parallel).
type Composite interface {
	BehaviorNode
	SetChildren(children ...BehaviorNode)
}

// Sensor pulls data from the external world and writes it to Blackboard.
type Sensor interface {
	// Name is used for registry and debugging.
	Name() string
	// Update is called each tick cycle to refresh Blackboard state.
	Update(ctx context.Context, bb Blackboard) error
}

// DecisionTree is a wrapper that holds a root node and exposes Tick.
type DecisionTree interface {
	Root() BehaviorNode
	Tick(t TickContext) (Status, error)
}

// Registry allows plug-and-play modules to be registered by name.
// It decouples configuration from concrete implementations.
type Registry interface {
	RegisterAction(name string, factory func(params map[string]any) (Action, error))
	RegisterCondition(name string, factory func(params map[string]any) (Condition, error))
	RegisterDecorator(name string, factory func(params map[string]any) (Decorator, error))
	RegisterComposite(name string, factory func(params map[string]any) (Composite, error))
	RegisterSensor(name string, factory func(params map[string]any) (Sensor, error))

	NewAction(name string, params map[string]any) (Action, error)
	NewCondition(name string, params map[string]any) (Condition, error)
	NewDecorator(name string, params map[string]any) (Decorator, error)
	NewComposite(name string, params map[string]any) (Composite, error)
	NewSensor(name string, params map[string]any) (Sensor, error)
}

// Agent coordinates sensors, decision tree, memory, blackboard, and events.
type Agent interface {
	// Step performs one cycle: sensors -> tree -> history.
	Step(ctx context.Context) (Status, error)
	// Blackboard returns the agent blackboard.
	Blackboard() Blackboard
	// Memory returns the agent decision memory.
	Memory() Memory
	// Events returns the shared event bus.
	Events() bus.EventBus
	// SaveState returns a binary snapshot of agent state (blackboard + memory).
	SaveState() ([]byte, error)
	// LoadState restores agent state from a binary snapshot.
	LoadState(b []byte) error
}

// DecisionRecord is used by Memory to keep audit logs of decisions and outcomes.
type DecisionRecord struct {
	Node      string          `json:"node"`
	Status    Status          `json:"status"`
	Duration  time.Duration   `json:"duration"`
	Timestamp time.Time       `json:"ts"`
	Metadata  json.RawMessage `json:"meta,omitempty"`
}
