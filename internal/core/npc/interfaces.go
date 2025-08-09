package npc

import (
	"context"
	"time"
)

// NodeStatus represents the execution status of a behavior tree node
type NodeStatus int

const (
	StatusSuccess NodeStatus = iota
	StatusFailure
	StatusRunning
	StatusInvalid
)

func (s NodeStatus) String() string {
	switch s {
	case StatusSuccess:
		return "Success"
	case StatusFailure:
		return "Failure"
	case StatusRunning:
		return "Running"
	default:
		return "Invalid"
	}
}

// ExecutionContext contains the context for node execution
type ExecutionContext struct {
	Context    context.Context
	Blackboard *Blackboard
	Agent      Agent
	DeltaTime  time.Duration
}

// Node represents a single node in the behavior tree
type Node interface {
	// Execute runs the node logic and returns the status
	Execute(ctx *ExecutionContext) NodeStatus

	// GetType returns the type of the node
	GetType() string

	// GetName returns the name/identifier of the node
	GetName() string

	// Reset resets the node to its initial state
	Reset()

	// Clone creates a copy of the node
	Clone() Node
}

// CompositeNode represents a node that can have children
type CompositeNode interface {
	Node

	// AddChild adds a child node
	AddChild(child Node)

	// GetChildren returns all child nodes
	GetChildren() []Node

	// RemoveChild removes a child node
	RemoveChild(child Node) bool
}

// DecoratorNode represents a node that modifies the behavior of a single child
type DecoratorNode interface {
	Node

	// SetChild sets the child node
	SetChild(child Node)

	// GetChild returns the child node
	GetChild() Node
}

// ActionNode represents a leaf node that performs an action
type ActionNode interface {
	Node

	// CanExecute checks if the action can be executed
	CanExecute(ctx *ExecutionContext) bool
}

// ConditionNode represents a leaf node that checks a condition
type ConditionNode interface {
	Node

	// Evaluate evaluates the condition
	Evaluate(ctx *ExecutionContext) bool
}

// Sensor represents a component that gathers information from the environment
type Sensor interface {
	// GetName returns the sensor name
	GetName() string

	// Update updates the sensor data and writes to blackboard
	Update(ctx *ExecutionContext) error

	// IsEnabled returns whether the sensor is active
	IsEnabled() bool

	// SetEnabled enables or disables the sensor
	SetEnabled(enabled bool)

	// GetUpdateInterval returns how often the sensor should update
	GetUpdateInterval() time.Duration
}

// Memory represents the agent's memory system
type Memory interface {
	// Store saves data to memory
	Store(key string, value interface{}) error

	// Retrieve gets data from memory
	Retrieve(key string) (interface{}, bool)

	// Remember adds an experience/event to memory
	Remember(event MemoryEvent) error

	// Recall retrieves experiences based on criteria
	Recall(criteria MemoryCriteria) ([]MemoryEvent, error)

	// Forget removes data from memory
	Forget(key string) error

	// Clear clears all memory
	Clear() error

	// Save exports memory to persistent storage
	Save() ([]byte, error)

	// Load imports memory from persistent storage
	Load(data []byte) error
}

// MemoryEvent represents a single memory event
type MemoryEvent struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	Timestamp  time.Time              `json:"timestamp"`
	Data       map[string]interface{} `json:"data"`
	Importance float64                `json:"importance"`
	Tags       []string               `json:"tags"`
}

// MemoryCriteria defines criteria for memory recall
type MemoryCriteria struct {
	Type       string                 `json:"type,omitempty"`
	Tags       []string               `json:"tags,omitempty"`
	TimeRange  *TimeRange             `json:"time_range,omitempty"`
	Importance *float64               `json:"min_importance,omitempty"`
	Limit      int                    `json:"limit,omitempty"`
	Data       map[string]interface{} `json:"data,omitempty"`
}

// TimeRange represents a time range for memory queries
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// EventHandler handles events in the agent system
type EventHandler interface {
	// GetEventType returns the type of events this handler processes
	GetEventType() string

	// Handle processes an event
	Handle(ctx *ExecutionContext, event Event) error

	// GetPriority returns the handler priority (higher = more important)
	GetPriority() int
}

// Event represents an event in the system
type Event struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Source    string                 `json:"source"`
	Target    string                 `json:"target,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
	Priority  int                    `json:"priority"`
}

// Agent represents an AI agent
type Agent interface {
	// GetID returns the agent's unique identifier
	GetID() string

	// GetName returns the agent's name
	GetName() string

	// GetBlackboard returns the agent's blackboard
	GetBlackboard() *Blackboard

	// GetMemory returns the agent's memory system
	GetMemory() Memory

	// Update updates the agent (called each frame/tick)
	Update(ctx context.Context, deltaTime time.Duration) error

	// HandleEvent processes an event
	HandleEvent(event Event) error

	// AddSensor adds a sensor to the agent
	AddSensor(sensor Sensor)

	// RemoveSensor removes a sensor from the agent
	RemoveSensor(sensorName string) bool

	// GetSensors returns all sensors
	GetSensors() []Sensor

	// SetBehaviorTree sets the agent's behavior tree
	SetBehaviorTree(root Node)

	// GetBehaviorTree returns the agent's behavior tree
	GetBehaviorTree() Node

	// Save exports the agent's state
	Save() ([]byte, error)

	// Load imports the agent's state
	Load(data []byte) error

	// Reset resets the agent to initial state
	Reset()

	// IsActive returns whether the agent is active
	IsActive() bool

	// SetActive sets the agent's active state
	SetActive(active bool)
}

// BehaviorTreeBuilder helps build behavior trees from configuration
type BehaviorTreeBuilder interface {
	// BuildFromConfig creates a behavior tree from configuration
	BuildFromConfig(config *BehaviorTreeConfig) (Node, error)

	// RegisterNodeType registers a custom node type
	RegisterNodeType(nodeType string, factory NodeFactory)

	// GetRegisteredTypes returns all registered node types
	GetRegisteredTypes() []string
}

// NodeFactory creates nodes of a specific type
type NodeFactory interface {
	// CreateNode creates a new node instance
	CreateNode(config *NodeConfig) (Node, error)

	// GetNodeType returns the type of nodes this factory creates
	GetNodeType() string
}

// AgentManager manages multiple agents
type AgentManager interface {
	// CreateAgent creates a new agent
	CreateAgent(config *AgentConfig) (Agent, error)

	// GetAgent retrieves an agent by ID
	GetAgent(id string) (Agent, bool)

	// RemoveAgent removes an agent
	RemoveAgent(id string) bool

	// GetAllAgents returns all agents
	GetAllAgents() []Agent

	// Update updates all agents
	Update(ctx context.Context, deltaTime time.Duration) error

	// BroadcastEvent sends an event to all agents
	BroadcastEvent(event Event) error

	// SendEvent sends an event to a specific agent
	SendEvent(agentID string, event Event) error

	// SaveAll saves all agents' states
	SaveAll() (map[string][]byte, error)

	// LoadAll loads all agents' states
	LoadAll(data map[string][]byte) error
}
