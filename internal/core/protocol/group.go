package protocol

import (
	"fmt"
)

// --- Group Implementation ---

// DefaultGroup is the default implementation of the Group interface.
type DefaultGroup struct {
	id          string
	subscribers *SyncMap[string, Subscriber]
}

// NewDefaultGroup creates a new DefaultGroup.
func NewDefaultGroup(id string) *DefaultGroup {
	return &DefaultGroup{
		id:          id,
		subscribers: NewSyncMap[string, Subscriber](),
	}
}

// ID returns the unique identifier for the group.
func (g *DefaultGroup) ID() string {
	return g.id
}

// Add adds a subscriber to the group.
func (g *DefaultGroup) Add(sub Subscriber) error {
	var subID string
	switch s := sub.(type) {
	case Client:
		subID = s.ID()
	case Scope:
		subID = fmt.Sprintf("%s/%s", s.Client().ID(), s.ID())
	default:
		return fmt.Errorf("unsupported subscriber type")
	}

	g.subscribers.Set(subID, sub)
	return nil
}

// Remove removes a subscriber from the group.
func (g *DefaultGroup) Remove(sub Subscriber) error {
	var subID string
	switch s := sub.(type) {
	case Client:
		subID = s.ID()
	case Scope:
		subID = fmt.Sprintf("%s/%s", s.Client().ID(), s.ID())
	default:
		return fmt.Errorf("unsupported subscriber type")
	}

	g.subscribers.Delete(subID)
	return nil
}

// Broadcast sends a message to all subscribers in the group.
func (g *DefaultGroup) Broadcast(msg Message) {
	g.subscribers.Range(func(_ string, sub Subscriber) bool {
		// Errors can be handled here, e.g., by logging or removing the subscriber.
		_ = sub.Send(msg)
		return true
	})
}

// Len returns the number of subscribers in the group.
func (g *DefaultGroup) Len() int {
	return g.subscribers.Len()
}
