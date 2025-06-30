package resolver

import "github.com/zeusync/zeusync/internal/core/sync"

var _ sync.ConflictResolver = (*Resolver)(nil)

type Resolver struct {
	priority int
}

func (r *Resolver) Resolve(local, remote any, metadata sync.ConflictMetadata) any {
	return local
}

func (r *Resolver) CanResolve(local, remote any) bool {
	return true
}

func (r *Resolver) Priority() int {
	return r.priority
}

type TypedResolver[T any] struct {
	resolver sync.ConflictResolver
}

func NewTypedResolver[T any](resolver sync.ConflictResolver) *TypedResolver[T] {
	return &TypedResolver[T]{
		resolver: resolver,
	}
}

func (r *TypedResolver[T]) Resolve(local, remote T, metadata sync.ConflictMetadata) T {
	resolver := r.resolver.Resolve(any(local), any(remote), metadata)
	return resolver.(T)
}

func (r *TypedResolver[T]) CanResolve(local, remote T) bool {
	return r.resolver.CanResolve(local, remote)
}

func (r *TypedResolver[T]) Priority() int {
	return r.resolver.Priority()
}
