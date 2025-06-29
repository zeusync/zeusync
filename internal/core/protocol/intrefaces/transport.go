package intrefaces

import (
	"io"
	"net"
)

type Transport interface {
	Listen(address net.Addr) error
	Accept() (Connection, error)
	Dial(address net.Addr) (Connection, error)
	io.Closer
}

type Connection interface {
	ID() string
	RemoteAddr() string
	Send([]byte) error
	Receive() ([]byte, error)
	io.Closer
	IsAlive() bool
}
