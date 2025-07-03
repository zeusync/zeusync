package protocol

import (
	"net"
	"time"
)

// Transport provides low-level network transport
type Transport interface {
	// Connection handling

	Listen(address string) error
	Accept() (Connection, error)
	Dial(address string) (Connection, error)
	Close() error

	// Configuration

	SetKeepAlive(keepAlive bool) error
	SetTimeout(timeout time.Duration) error
	SetBufferSize(size int) error

	// Status

	LocalAddr() net.Addr
	IsListening() bool
}
