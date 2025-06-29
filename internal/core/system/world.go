package system

import (
	"github.com/zeusync/zeusync/internal/core/models/interfaces"
	"time"
)

// World provides access to the game world state
// Acts as a facade for different architectural patterns
type World interface {
	// Entity management

	EntityRegistry() interfaces.EntityRegistry

	// Time management

	DeltaTime() float64
	FixedDeltaTime() float64
	TotalTime() time.Duration
	FrameCount() int64

	// Resource management

	GetResource(name string) (any, bool)
	SetResource(name string, value any)
	RemoveResource(name string)

	// Event system access

	PublishEvent(event any) error
	SubscribeToEvent(eventType string, handler func(any)) error

	// World state

	IsPaused() bool
	SetPaused(bool)
	IsShuttingDown() bool

	// Queries and searches

	FindEntity(name string) (interfaces.Entity, bool)
	FindEntitiesByTag(tag string) []interfaces.Entity
	FindEntitiesInRadius(x, y, z, radius float64) []interfaces.Entity

	// Spatial partitioning access

	GetSpatialPartition() SpatialPartition

	// Configuration

	GetConfig(key string) (any, bool)
	SetConfig(key string, value any)
}

// SpatialPartition provides spatial queries for performance
type SpatialPartition interface {
	Insert(entityId interfaces.EntityID, x, y, z float64) error
	Remove(entityId interfaces.EntityID) error
	Update(entityId interfaces.EntityID, x, y, z float64) error

	QueryRadius(x, y, z, radius float64) []interfaces.EntityID
	QueryBounds(minX, minY, minZ, maxX, maxY, maxZ float64) []interfaces.EntityID
	QueryRay(startX, startY, startZ, dirX, dirY, dirZ, maxDistance float64) []interfaces.EntityID

	Clear() error
	GetStatistics() SpatialStatistics
}

// SpatialStatistics provides information about spatial partitioning performance
type SpatialStatistics struct {
	NodeCount        uint16
	EntityCount      uint64
	MaxDepth         uint64
	AverageDepth     float64
	QueriesPerSecond float64
	UpdatesPerSecond float64
}
