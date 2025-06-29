package interfaces

// EntityRegistry manages entity lifecycle and queries
// Supports different storage backends and query patterns
type EntityRegistry interface {
	// Entity lifecycle

	CreateEntity() Entity
	CreateEntityWithID(EntityID) Entity
	CreateEntityFromTemplate(templateName string) (Entity, error)
	DestroyEntity(EntityID) error
	GetEntity(EntityID) (Entity, bool)
	ExistsEntity(EntityID) bool

	// Bulk operations

	CreateEntities(count uint64) []Entity
	DestroyEntities(...EntityID) error
	GetEntities(...EntityID) map[EntityID]Entity

	// Query operations (supports ECS-style queries)

	QueryEntities(Query) Iterator
	QueryComponents(ComponentID) ComponentIterator
	QueryWithFilter(EntityFilter) Iterator

	// Type management

	RegisterEntityType(string, Factory) error
	UnregisterEntityType(string) error
	GetEntityType(string) (Factory, bool)
	ListEntityTypes() []string

	// Component type management

	RegisterComponentType(ComponentFactory) error
	UnregisterComponentType(ComponentID) error
	GetComponentType(ComponentID) (ComponentFactory, bool)
	GetComponentTypeByName(string) (ComponentFactory, bool)
	ListComponentTypes() []ComponentFactory

	// Statistics and monitoring

	EntityCount() uint64
	ComponentCount(ComponentID) uint64
	GetStatistics() RegistryStatistics

	// Serialization and persistence

	SaveSnapshot() ([]byte, error)
	LoadSnapshot([]byte) error
	SaveEntity(EntityID) ([]byte, error)
	LoadEntity([]byte) (Entity, error)

	// Event notifications

	OnEntityCreated(func(Entity))
	OnEntityDestroyed(func(EntityID))
	OnComponentAdded(func(EntityID, ComponentID))
	OnComponentRemoved(func(EntityID, ComponentID))
}

// Query represents a query for entities/components
// Supports complex filtering similar to database queries
type Query interface {
	// Component filters (ECS style)

	WithComponent(ComponentID) Query
	WithoutComponent(ComponentID) Query
	WithAnyComponent(...ComponentID) Query
	WithAllComponents(...ComponentID) Query

	// Property filters

	Where(field string, operator ComparisonOperator, value any) Query
	WhereTag(tag string) Query
	WhereActive(bool) Query
	WhereName(pattern string) Query

	// Spatial queries (for position-based components)

	WithinRadius(x, y, z, radius float64) Query
	WithinBounds(minX, minY, minZ, maxX, maxY, maxZ float64) Query

	// Sorting and limiting

	OrderBy(field string, ascending bool) Query
	Limit(count uint64) Query
	Offset(count uint64) Query

	// Execution

	Execute() Iterator
	Count() int
	First() (Entity, bool)
	Exists() bool
}

// EntityFilter is a function-based filter for entities
type EntityFilter func(Entity) bool

// ComparisonOperator defines comparison operations for queries
type ComparisonOperator int

const (
	OpEqual ComparisonOperator = iota
	OpNotEqual
	OpGreaterThan
	OpGreaterThanOrEqual
	OpLessThan
	OpLessThanOrEqual
	OpIn
	OpNotIn
	OpLike
	OpRegex
)

// RegistryStatistics provides runtime statistics
type RegistryStatistics struct {
	TotalEntities     uint64
	ActiveEntities    uint64
	TotalComponents   map[ComponentID]uint64
	MemoryUsage       uint64
	QueriesPerSecond  float64
	CreationRate      float64
	DestructionRate   float64
	AverageComponents float64
}
