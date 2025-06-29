package interfaces

import (
	"github.com/zeusync/zeusync/internal/core/models"
)

// ComponentID represents a unique identifier for component types
type ComponentID uint32

// Component represents a data container or behavior
// Can be used for pure data (ECS) or methods (OOP)
type Component interface {
	// Type information

	TypeID() ComponentID
	TypeName() string

	// Lifecycle

	OnAttach(Entity) error
	OnDetach(Entity) error
	OnUpdate(deltaTime float64) error

	// Validation and integrity

	Validate() error
	IsValid() bool

	// Serialization

	Marshal() ([]byte, error)
	Unmarshal([]byte) error
	Clone() Component

	// Dependencies

	Dependencies() []ComponentID
	Conflicts() []ComponentID

	// Metadata

	Version() string
	Metadata() map[string]interface{}
}

// ComponentSchema defines the structure of a component
type ComponentSchema interface {
	Fields() []FieldDefinition
	Validate(Component) error
	Default() Component
	Migrate(from, to string, data []byte) ([]byte, error)
}

// ComponentIterator provides iteration over components
type ComponentIterator interface {
	models.DualIterator[Component, ComponentID]
}
