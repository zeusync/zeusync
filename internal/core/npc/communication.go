package npc

import "context"

// Communications primitives (minimal stubs)

// Message represents a simple in-process message between agents.
type Message struct {
	Type string
	Data any
}

// MessageQueue is an in-memory queue per agent.
type MessageQueue interface {
	Send(msg Message)
	TryReceive() (Message, bool)
}

type messageQueue struct{ ch chan Message }

func NewMessageQueue(buffer int) MessageQueue { return &messageQueue{ch: make(chan Message, buffer)} }
func (q *messageQueue) Send(msg Message)      { q.ch <- msg }
func (q *messageQueue) TryReceive() (Message, bool) {
	select {
	case m := <-q.ch:
		return m, true
	default:
		return Message{}, false
	}
}

// NetworkInterface is a very small abstraction for external comms.
type NetworkInterface interface {
	IsConnected(ctx context.Context) bool
	Send(ctx context.Context, payload any) error
}

type noopNetwork struct{}

func NewNoopNetwork() NetworkInterface                          { return noopNetwork{} }
func (noopNetwork) IsConnected(ctx context.Context) bool        { return false }
func (noopNetwork) Send(ctx context.Context, payload any) error { return nil }
