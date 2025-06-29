package interfaces

import (
	"github.com/zeusync/zeusync/internal/core/models"
)

// EntityID represents a unique identifier for entities
// Can be used for both ECS entities and OOP objects
type EntityID uint64

// Entity represents a game object that can hold components
// This interface is flexible enough to support ECS, OOP, and Actor patterns
type Entity interface {
	// Basic identity

	ID() EntityID
	Name() string
	SetName(string)

	// Component management (ECS style)

	AddComponent(ComponentID, Component) error
	RemoveComponent(ComponentID) error
	GetComponent(ComponentID) (Component, bool)
	HasComponent(ComponentID) bool
	ListComponents() []ComponentID

	// State management

	IsActive() bool
	SetActive(bool)
	IsDestroyed() bool
	Destroy()

	// Hierarchy support (for OOP style)

	Parent() Entity
	SetParent(Entity) error
	Children() []Entity
	AddChild(Entity) error
	RemoveChild(EntityID) error

	// Serialization

	Marshal() ([]byte, error)
	Unmarshal([]byte) error
	Clone() Entity

	// Metadata

	Tags() []string
	AddTag(string)
	RemoveTag(string)
	HasTag(string) bool
	Metadata() map[string]interface{}
	SetMetadata(string, interface{})
	GetMetadata(string) (interface{}, bool)
}

// Factory creates entities of specific types
type Factory interface {
	Create() Entity
	CreateWithID(EntityID) Entity
	TypeName() string
	DefaultComponents() []ComponentID
}

// ComponentFactory creates components of specific types
type ComponentFactory interface {
	Create() Component
	TypeID() ComponentID
	TypeName() string
	Version() string
	Schema() ComponentSchema
}

// FieldDefinition describes a field in a component
type FieldDefinition struct {
	Name        string
	Type        FieldType
	Required    bool
	Default     interface{}
	Constraints []FieldConstraint
	Description string
	Metadata    map[string]interface{}
}

// FieldType represents the data type of field
type FieldType uint16

const (
	FieldTypeInt FieldType = iota
	FieldTypeUInt
	FieldTypeString
	FieldTypeFloat
	FieldTypeBool
	FieldTypeArray
	FieldTypeObject
	FieldTypeEntityID
	FieldTypeComponentID
	FieldTypeCustom = 0xFFFF - 1
)

type FieldConstraint interface {
	Validate(value any) error
	Type() string
	Parameters() map[string]interface{}
}

// Iterator provides iteration over query results
type Iterator interface {
	models.DualIterator[Entity, EntityID]
}
