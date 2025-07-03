package protocol

import (
	"net"
	"time"
)

// Connection represents a network connection
type Connection interface {
	// Identity

	ID() string
	RemoteAddr() net.Addr
	LocalAddr() net.Addr

	// Data transfer

	Send(data []byte) error
	Receive() ([]byte, error)
	SendMessage(message IMessage) error
	ReceiveMessage() (IMessage, error)

	// Connection state

	IsAlive() bool
	IsClosed() bool
	LastActivity() time.Time
	Close() error
	CloseWithReason(reason string) error

	// Configuration

	SetTimeout(timeout time.Duration) error
	SetKeepAlive(keepAlive bool) error
	SetBufferSize(size int) error

	// Metadata

	Metadata() map[string]interface{}
	SetMetadata(key string, value interface{})
	GetMetadata(key string) (interface{}, bool)

	// Events

	OnClose(callback func(string))
	OnError(callback func(error))
	OnMessage(callback func(IMessage))
}
