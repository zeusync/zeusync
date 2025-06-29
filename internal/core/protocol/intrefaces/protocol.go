package intrefaces

import (
	"net"
	"time"
)

type Protocol interface {
	Name() string
	Start(addr net.Addr) error
	Stop() error
	Send(clientID string, message Message) error
	Broadcast(message Message) error
	RegisterHandler(messageType string, handler MessageHandler) error
	GetClient() []ClientInfo
}

type MessageHandler func(clientID string, message Message) error

type Message interface {
	Type() string
	Data() []byte
	Metadata() map[string]any
	Timestamp() time.Time
	Size() uint64
}

type ClientInfo struct {
	ID   string
	Addr net.Addr
}
