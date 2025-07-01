package generic

import "sync"

type Pool[T any] struct {
	pool sync.Pool
}

func NewPool[T any](generate func() T) *Pool[T] {
	return &Pool[T]{
		pool: sync.Pool{
			New: func() any {
				return generate()
			},
		},
	}
}

func NewHotPool[T any](generate func() T, hotSize int) *Pool[T] {
	p := NewPool[T](generate)
	for i := 0; i < hotSize; i++ {
		p.pool.Put(generate())
	}
	return p
}

func (p *Pool[T]) Get() T {
	return p.pool.Get().(T)
}

func (p *Pool[T]) Put(value T) {
	p.pool.Put(value)
}
