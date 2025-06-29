package system

import (
	"context"
	"time"
)

// Manager orchestrates all systems in the game
// Handles execution order, dependencies, and lifecycle
type Manager interface {
	// System registration

	RegisterSystem(System) error
	UnregisterSystem(name string) error
	GetSystem(name string) (System, bool)
	ListSystems() []System
	HasSystem(name string) bool

	// System lifecycle

	InitializeAll(ctx context.Context, world World) error
	ShutdownAll(ctx context.Context) error
	EnableSystem(name string) error
	DisableSystem(name string) error
	RestartSystem(name string) error

	// Execution control

	Update(deltaTime float64, world World) error
	FixedUpdate(fixedDeltaTime float64, world World) error
	UpdatePhase(phase ExecutionPhase, deltaTime float64, world World) error

	// Configuration

	SetSystemConfig(name string, config Config) error
	GetSystemConfig(name string) (Config, bool)

	// Monitoring and debugging

	GetMetrics() ManagerMetrics
	GetSystemMetrics(name string) (Metrics, bool)
	GetExecutionOrder() []string
	ValidateDependencies() error

	// Hot reload support

	ReloadSystem(name string, newSystem System) error
	CanReload(name string) bool

	// Events

	OnSystemRegistered(func(System))
	OnSystemUnregistered(func(string))
	OnSystemError(func(string, error))
}

// ManagerMetrics provides system manager statistics
type ManagerMetrics struct {
	RegisteredSystems uint32
	EnabledSystems    uint32
	RunningSystem     uint32
	TotalUpdateTime   time.Duration
	AverageUpdateTime time.Duration
	SystemErrorCount  map[string]uint32
	LastUpdateTime    time.Time
	UpdatesPerSecond  uint32
}

// Scheduler handles complex system execution patterns
type Scheduler interface {
	// Scheduling

	ScheduleSystem(name string, schedule Schedule) error
	UnscheduleSystem(name string) error

	// Execution

	ShouldExecute(name string, currentTime time.Time) bool
	GetNextExecution(name string) time.Time

	// Monitoring

	GetScheduleStatus(name string) ScheduleStatus
}

// Schedule defines when and how often a system should run
type Schedule interface {
	Type() ScheduleType
	ShouldExecute(lastExecution time.Time, currentTime time.Time) bool
	NextExecution(currentTime time.Time) time.Time
}

// ScheduleType defines different scheduling patterns
type ScheduleType uint8

const (
	ScheduleEveryFrame ScheduleType = iota
	ScheduleFixedInterval
	ScheduleCron
	ScheduleConditional
	ScheduleOnDemand
)

// ScheduleStatus provides information about scheduled execution
type ScheduleStatus struct {
	IsScheduled      bool
	LastExecution    time.Time
	NextExecution    time.Time
	ExecutionCount   uint64
	MissedExecutions uint64
	AverageLatency   time.Duration
}
