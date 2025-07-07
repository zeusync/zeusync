package protocol

import (
	"errors"
	"time"
)

// Core protocol errors
var (
	// Connection errors

	ErrConnectionClosed      = errors.New("connection is closed")
	ErrConnectionTimeout     = errors.New("connection timeout")
	ErrConnectionRefused     = errors.New("connection refused")
	ErrConnectionLost        = errors.New("connection lost")
	ErrInvalidConnection     = errors.New("invalid connection")
	ErrMaxConnectionsReached = errors.New("maximum connections reached")

	// Client errors

	ErrClientNotFound     = errors.New("client not found")
	ErrClientInactive     = errors.New("client is inactive")
	ErrClientDisconnected = errors.New("client is disconnected")
	ErrMaxClientsExceeded = errors.New("maximum clients exceeded")
	ErrInvalidClientID    = errors.New("invalid client ID")

	// Message errors

	ErrMessageTooLarge       = errors.New("message too large")
	ErrMessageExpired        = errors.New("message expired")
	ErrInvalidMessage        = errors.New("invalid message")
	ErrMessageQueueFull      = errors.New("message queue is full")
	ErrSerializationFailed   = errors.New("message serialization failed")
	ErrDeserializationFailed = errors.New("message deserialization failed")

	// Stream errors

	ErrStreamClosed      = errors.New("stream is closed")
	ErrStreamNotFound    = errors.New("stream not found")
	ErrMaxStreamsReached = errors.New("maximum streams reached")
	ErrStreamTimeout     = errors.New("stream timeout")
	ErrInvalidStreamID   = errors.New("invalid stream ID")
	ErrStreamReset       = errors.New("stream was reset")

	// Group errors

	ErrGroupNotFound      = errors.New("group not found")
	ErrGroupClosed        = errors.New("group is closed")
	ErrMaxGroupsExceeded  = errors.New("maximum groups exceeded")
	ErrGroupAlreadyExists = errors.New("group already exists")
	ErrInvalidGroupID     = errors.New("invalid group ID")

	// Scope errors

	ErrScopeNotFound       = errors.New("scope not found")
	ErrScopeClosed         = errors.New("scope is closed")
	ErrMaxScopesExceeded   = errors.New("maximum scopes exceeded")
	ErrScopeAlreadyExists  = errors.New("scope already exists")
	ErrInvalidScopeID      = errors.New("invalid scope ID")
	ErrMaxDataSizeExceeded = errors.New("maximum data size exceeded")
	ErrMaxKeysExceeded     = errors.New("maximum keys exceeded")
	ErrDataNotFound        = errors.New("data not found")

	// Transport errors

	ErrTransportNotSupported = errors.New("transport not supported")
	ErrTransportClosed       = errors.New("transport is closed")
	ErrTransportFailed       = errors.New("transport failed")
	ErrInvalidAddress        = errors.New("invalid address")
	ErrBindFailed            = errors.New("bind failed")
	ErrListenFailed          = errors.New("listen failed")
	ErrDialFailed            = errors.New("dial failed")

	// Registry errors

	ErrRegistryClosed    = errors.New("registry is closed")
	ErrAlreadyRegistered = errors.New("already registered")
	ErrNotRegistered     = errors.New("not registered")

	// Configuration errors

	ErrInvalidConfig        = errors.New("invalid configuration")
	ErrConfigurationMissing = errors.New("configuration missing")
	ErrUnsupportedFeature   = errors.New("unsupported feature")

	// Security errors

	ErrAuthenticationFailed = errors.New("authentication failed")
	ErrAuthorizationFailed  = errors.New("authorization failed")
	ErrCertificateInvalid   = errors.New("certificate invalid")
	ErrEncryptionFailed     = errors.New("encryption failed")
	ErrDecryptionFailed     = errors.New("decryption failed")

	// Protocol errors

	ErrProtocolViolation = errors.New("protocol violation")
	ErrInvalidFrame      = errors.New("invalid frame")
	ErrFrameTooLarge     = errors.New("frame too large")
	ErrInvalidHeader     = errors.New("invalid header")
	ErrChecksumMismatch  = errors.New("checksum mismatch")

	// Resource errors

	ErrResourceExhausted   = errors.New("resource exhausted")
	ErrMemoryLimitExceeded = errors.New("memory limit exceeded")
	ErrCPULimitExceeded    = errors.New("CPU limit exceeded")
	ErrDiskSpaceExhausted  = errors.New("disk space exhausted")

	// Concurrency errors

	ErrDeadlock      = errors.New("deadlock detected")
	ErrRaceCondition = errors.New("race condition detected")
	ErrLockTimeout   = errors.New("lock timeout")

	// Generic errors

	ErrNotImplemented        = errors.New("not implemented")
	ErrOperationNotSupported = errors.New("operation not supported")
	ErrInternalError         = errors.New("internal error")
	ErrUnknownError          = errors.New("unknown error")
)

// ErrorCode represents a numeric error code for efficient error handling
type ErrorCode int

const (
	// Success

	ErrorCodeSuccess ErrorCode = 0

	// Connection error codes (1000-1999)

	ErrorCodeConnectionClosed      ErrorCode = 1001
	ErrorCodeConnectionTimeout     ErrorCode = 1002
	ErrorCodeConnectionRefused     ErrorCode = 1003
	ErrorCodeConnectionLost        ErrorCode = 1004
	ErrorCodeInvalidConnection     ErrorCode = 1005
	ErrorCodeMaxConnectionsReached ErrorCode = 1006
	ErrorCodeProtocolViolation     ErrorCode = 1007
	ErrorCodeAuthenticationFailed  ErrorCode = 1008

	// Client error codes (2000-2999)

	ErrorCodeClientNotFound      ErrorCode = 2001
	ErrorCodeClientInactive      ErrorCode = 2002
	ErrorCodeClientDisconnected  ErrorCode = 2003
	ErrorCodeMaxClientsExceeded  ErrorCode = 2004
	ErrorCodeInvalidClientID     ErrorCode = 2005
	ErrorCodeAuthorizationFailed ErrorCode = 2006

	// Message error codes (3000-3999)

	ErrorCodeMessageTooLarge       ErrorCode = 3001
	ErrorCodeMessageExpired        ErrorCode = 3002
	ErrorCodeInvalidMessage        ErrorCode = 3003
	ErrorCodeMessageQueueFull      ErrorCode = 3004
	ErrorCodeSerializationFailed   ErrorCode = 3005
	ErrorCodeDeserializationFailed ErrorCode = 3006

	// Stream error codes (4000-4999)

	ErrorCodeStreamClosed      ErrorCode = 4001
	ErrorCodeStreamNotFound    ErrorCode = 4002
	ErrorCodeMaxStreamsReached ErrorCode = 4003
	ErrorCodeStreamTimeout     ErrorCode = 4004
	ErrorCodeInvalidStreamID   ErrorCode = 4005
	ErrorCodeStreamReset       ErrorCode = 4006
	ErrorCodeResourceExhausted ErrorCode = 4007

	// Group error codes (5000-5999)

	ErrorCodeGroupNotFound      ErrorCode = 5001
	ErrorCodeGroupClosed        ErrorCode = 5002
	ErrorCodeMaxGroupsExceeded  ErrorCode = 5003
	ErrorCodeGroupAlreadyExists ErrorCode = 5004
	ErrorCodeInvalidGroupID     ErrorCode = 5005

	// Scope error codes (6000-6999)

	ErrorCodeScopeNotFound       ErrorCode = 6001
	ErrorCodeScopeClosed         ErrorCode = 6002
	ErrorCodeMaxScopesExceeded   ErrorCode = 6003
	ErrorCodeScopeAlreadyExists  ErrorCode = 6004
	ErrorCodeInvalidScopeID      ErrorCode = 6005
	ErrorCodeMaxDataSizeExceeded ErrorCode = 6006
	ErrorCodeMaxKeysExceeded     ErrorCode = 6007
	ErrorCodeDataNotFound        ErrorCode = 6008

	// Transport error codes (7000-7999)

	ErrorCodeTransportNotSupported ErrorCode = 7001
	ErrorCodeTransportClosed       ErrorCode = 7002
	ErrorCodeTransportFailed       ErrorCode = 7003
	ErrorCodeInvalidAddress        ErrorCode = 7004
	ErrorCodeBindFailed            ErrorCode = 7005
	ErrorCodeListenFailed          ErrorCode = 7006
	ErrorCodeDialFailed            ErrorCode = 7007

	// Generic error codes (9000-9999)

	ErrorCodeNotImplemented        ErrorCode = 9001
	ErrorCodeOperationNotSupported ErrorCode = 9002
	ErrorCodeInternalError         ErrorCode = 9003
	ErrorCodeUnknownError          ErrorCode = 9999
)

// Error represents a protocol-specific error with additional context
type Error struct {
	Code      ErrorCode
	Message   string
	Cause     error
	Context   map[string]interface{}
	Timestamp int64
}

// Error implements the error interface
func (e *Error) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

// Unwrap returns the underlying error
func (e *Error) Unwrap() error {
	return e.Cause
}

// NewProtocolError creates a new protocol error
func NewProtocolError(code ErrorCode, message string, cause error) *Error {
	return &Error{
		Code:      code,
		Message:   message,
		Cause:     cause,
		Context:   make(map[string]interface{}),
		Timestamp: time.Now().Unix(),
	}
}

// WithContext adds context to the error
func (e *Error) WithContext(key string, value interface{}) *Error {
	e.Context[key] = value
	return e
}

// IsTemporary checks if the error is temporary and the operation can be retried
func (e *Error) IsTemporary() bool {
	switch e.Code {
	case ErrorCodeConnectionTimeout,
		ErrorCodeConnectionLost,
		ErrorCodeMessageQueueFull,
		ErrorCodeStreamTimeout,
		ErrorCodeResourceExhausted:
		return true
	default:
		return false
	}
}

// IsRetryable checks if the operation should be retried
func (e *Error) IsRetryable() bool {
	return e.IsTemporary()
}

// IsFatal checks if the error is fatal and the connection should be closed
func (e *Error) IsFatal() bool {
	switch e.Code {
	case ErrorCodeConnectionClosed,
		ErrorCodeConnectionRefused,
		ErrorCodeProtocolViolation,
		ErrorCodeAuthenticationFailed,
		ErrorCodeAuthorizationFailed:
		return true
	default:
		return false
	}
}

// Error mapping from standard errors to error codes
var errorCodeMap = map[error]ErrorCode{
	ErrConnectionClosed:      ErrorCodeConnectionClosed,
	ErrConnectionTimeout:     ErrorCodeConnectionTimeout,
	ErrConnectionRefused:     ErrorCodeConnectionRefused,
	ErrConnectionLost:        ErrorCodeConnectionLost,
	ErrInvalidConnection:     ErrorCodeInvalidConnection,
	ErrMaxConnectionsReached: ErrorCodeMaxConnectionsReached,

	ErrClientNotFound:     ErrorCodeClientNotFound,
	ErrClientInactive:     ErrorCodeClientInactive,
	ErrClientDisconnected: ErrorCodeClientDisconnected,
	ErrMaxClientsExceeded: ErrorCodeMaxClientsExceeded,
	ErrInvalidClientID:    ErrorCodeInvalidClientID,

	ErrMessageTooLarge:       ErrorCodeMessageTooLarge,
	ErrMessageExpired:        ErrorCodeMessageExpired,
	ErrInvalidMessage:        ErrorCodeInvalidMessage,
	ErrMessageQueueFull:      ErrorCodeMessageQueueFull,
	ErrSerializationFailed:   ErrorCodeSerializationFailed,
	ErrDeserializationFailed: ErrorCodeDeserializationFailed,

	ErrStreamClosed:      ErrorCodeStreamClosed,
	ErrStreamNotFound:    ErrorCodeStreamNotFound,
	ErrMaxStreamsReached: ErrorCodeMaxStreamsReached,
	ErrStreamTimeout:     ErrorCodeStreamTimeout,
	ErrInvalidStreamID:   ErrorCodeInvalidStreamID,
	ErrStreamReset:       ErrorCodeStreamReset,

	ErrGroupNotFound:      ErrorCodeGroupNotFound,
	ErrGroupClosed:        ErrorCodeGroupClosed,
	ErrMaxGroupsExceeded:  ErrorCodeMaxGroupsExceeded,
	ErrGroupAlreadyExists: ErrorCodeGroupAlreadyExists,
	ErrInvalidGroupID:     ErrorCodeInvalidGroupID,

	ErrScopeNotFound:       ErrorCodeScopeNotFound,
	ErrScopeClosed:         ErrorCodeScopeClosed,
	ErrMaxScopesExceeded:   ErrorCodeMaxScopesExceeded,
	ErrScopeAlreadyExists:  ErrorCodeScopeAlreadyExists,
	ErrInvalidScopeID:      ErrorCodeInvalidScopeID,
	ErrMaxDataSizeExceeded: ErrorCodeMaxDataSizeExceeded,
	ErrMaxKeysExceeded:     ErrorCodeMaxKeysExceeded,
	ErrDataNotFound:        ErrorCodeDataNotFound,

	ErrTransportNotSupported: ErrorCodeTransportNotSupported,
	ErrTransportClosed:       ErrorCodeTransportClosed,
	ErrTransportFailed:       ErrorCodeTransportFailed,
	ErrInvalidAddress:        ErrorCodeInvalidAddress,
	ErrBindFailed:            ErrorCodeBindFailed,
	ErrListenFailed:          ErrorCodeListenFailed,
	ErrDialFailed:            ErrorCodeDialFailed,

	ErrNotImplemented:        ErrorCodeNotImplemented,
	ErrOperationNotSupported: ErrorCodeOperationNotSupported,
	ErrInternalError:         ErrorCodeInternalError,
	ErrUnknownError:          ErrorCodeUnknownError,
}

// GetErrorCode returns the error code for a given error
func GetErrorCode(err error) ErrorCode {
	if code, exists := errorCodeMap[err]; exists {
		return code
	}

	// Check if it's already a ProtocolError
	var protocolErr *Error
	if errors.As(err, &protocolErr) {
		return protocolErr.Code
	}

	return ErrorCodeUnknownError
}

// WrapError wraps a standard error into a ProtocolError
func WrapError(err error, message string) *Error {
	code := GetErrorCode(err)
	return NewProtocolError(code, message, err)
}
