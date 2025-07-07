package client

import "errors"

// Client-specific errors
var (
	ErrClientClosed      = errors.New("client is closed")
	ErrNotConnected      = errors.New("client is not connected")
	ErrAlreadyConnected  = errors.New("client is already connected")
	ErrConnectionTimeout = errors.New("connection timeout")
	ErrReconnectFailed   = errors.New("reconnection failed")
	ErrInvalidConfig     = errors.New("invalid client configuration")
	ErrHandlerNotFound   = errors.New("handler not found")
	ErrMessageTimeout    = errors.New("message timeout")
	ErrInvalidMessage    = errors.New("invalid message")
	ErrGroupNotFound     = errors.New("group not found")
	ErrScopeNotFound     = errors.New("scope not found")
)
