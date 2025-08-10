package systems

import (
	"context"
	"time"

	"github.com/zeusync/zeusync/internal/core/models/interfaces"
)

// System represents a game logic processor
// Can be used for ECS systems, OOP behaviors, or Actor message processing
type System interface {
	// Identity

	Name() string
	Version() string
	Description() string

	// Lifecycle

	Initialize(ctx context.Context, world World) error
	Shutdown(ctx context.Context) error
	Reset() error

	// Execution

	Update(deltaTime float64, world World) error
	FixedUpdate(fixedDeltaTime float64, world World) error
	LateUpdate(deltaTime float64, world World) error

	// Configuration

	Priority() Priority
	ExecutionPhase() ExecutionPhase
	Dependencies() []string
	Conflicts() []string

	// State management

	IsEnabled() bool
	SetEnabled(bool) error
	IsInitialized() bool
	GetState() StateIdentity

	// Performance monitoring

	GetMetrics() Metrics
	GetLastExecutionTime() time.Duration
	GetAverageExecutionTime() time.Duration

	// Configuration

	Configure(config Config) error
	GetConfig() Config

	// Events

	OnEntityAdded(interfaces.Entity) error
	OnEntityRemoved(interfaces.EntityID) error
	OnComponentAdded(interfaces.EntityID, interfaces.ComponentID) error
	OnComponentRemoved(interfaces.EntityID, interfaces.ComponentID) error
}

// Priority defines execution order priority
type Priority uint16

// System priorities
const (
	PriorityLowestLowest  Priority = 100
	PriorityLowest        Priority = 200
	PriorityLowestNormal  Priority = 200
	PriorityLowestHighest Priority = 300

	PriorityLowestLow  Priority = 400
	PriorityLow        Priority = 500
	PriorityLowNormal  Priority = 500
	PriorityLowHighest Priority = 600

	PriorityNormalLowest  Priority = 700
	PriorityNormal        Priority = 600
	PriorityNormalNormal  Priority = 600
	PriorityNormalHighest Priority = 800

	PriorityHighLowest  Priority = 900
	PriorityHigh        Priority = 1000
	PriorityHighNormal  Priority = 1000
	PriorityHighHighest Priority = 1100

	PriorityHighestLow     Priority = 1200
	PriorityHighest        Priority = 1300
	PriorityHighestNormal  Priority = 1300
	PriorityHighestHighest Priority = 1400
)

// ExecutionPhase defines when a system runs
type ExecutionPhase uint8

const (
	PhasePreStart ExecutionPhase = iota
	PhaseStart
	PhasePostStart
	PhasePreUpdate
	PhaseUpdate
	PhasePostUpdate
	PhaseFixedUpdate
	PhaseLateUpdate
	PhasePreRender
	PhaseRender
	PhasePostRender
)

// StateIdentity represents the current state of a system
type StateIdentity uint8

const (
	StateUninitialized StateIdentity = iota
	StateInitializing
	StateRunning
	StatePaused
	StateShuttingDown
	StateShutdown
	StateError
	StateEnabled
	StateDisabled
	StateFailed
)

// Config holds system configuration
type Config map[string]any

// Metrics provides runtime metrics for a system
type Metrics struct {
	ExecutionCount       uint64
	TotalExecutionTime   time.Duration
	AverageExecutionTime time.Duration
	MaxExecutionTime     time.Duration
	MinExecutionTime     time.Duration
	ErrorCount           uint64
	LastError            error
	LastExecutionTime    time.Time
	EntitiesProcessed    uint64
}
