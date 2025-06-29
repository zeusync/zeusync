package models

type EntityID uint64
type ComponentID uint32

type Entity interface {
	ID() EntityID
	AddComponent(ComponentID, Component) error
	RemoveComponent(ComponentID) error
	GetComponent(ComponentID) (Component, bool)
	HasComponent(ComponentID) bool
	ListComponents() []ComponentID
	Clone() Entity
}

type Component interface {
	TypeID() ComponentID
	Validate() error
	Clone() Component
}

type SerializableComponent interface {
	Component
	Serialize() ([]byte, error)
	Deserialize([]byte) error
}

type EntityRegistry interface {
	CreateEntity() Entity
	DestroyEntity(EntityID) error
	GetEntity(EntityID) (Entity, error)
	QueryEntities(Query) (EntityIterator, error)
	RegisterEntityType(string, EntityFactory) error
}

type Query interface{}

type EntityIterator interface{}

type EntityFactory interface{}
