package server

import "errors"

// Server-specific errors
var (
	ErrServerClosed         = errors.New("server is closed")
	ErrServerNotRunning     = errors.New("server is not running")
	ErrServerAlreadyRunning = errors.New("server is already running")
	ErrMaxClientsReached    = errors.New("maximum clients reached")
	ErrClientNotFound       = errors.New("client not found")
	ErrGroupNotFound        = errors.New("group not found")
	ErrScopeNotFound        = errors.New("scope not found")
	ErrInvalidMessage       = errors.New("invalid message")
	ErrInvalidConfig        = errors.New("invalid server configuration")
	ErrListenerFailed       = errors.New("failed to create listener")
	ErrTransportFailed      = errors.New("transport operation failed")
)
