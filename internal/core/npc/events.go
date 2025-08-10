package npc

import (
	"github.com/zeusync/zeusync/internal/core/events/bus"
)

// NewEventBus creates a new shared event bus instance based on the core bus implementation.
func NewEventBus() bus.EventBus {
	return bus.New()
}
