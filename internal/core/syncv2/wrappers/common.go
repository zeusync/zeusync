package wrappers

import "github.com/zeusync/zeusync/internal/core/syncv2"

// callbackResolver adapts a callback function to the ConflictResolver interface
type callbackResolver[T any] struct {
	callback func(local, remote T) T
}

func (r *callbackResolver[T]) Resolve(local, remote T, _ sync.ConflictMetadata) T {
	return r.callback(local, remote)
}

func (r *callbackResolver[T]) Priority() int {
	return 1 // Default priority
}
