package vars

import (
	"github.com/zeusync/zeusync/internal/core/syncv2"
	"testing"
)

type atomicTest[T any] struct {
	name       string
	createFn   func(initialVal T) sync.AtomicRoot[T]
	initialVal T
	setValue   T
	swapValue  T
}

func TestAtomic_AtomicInt64(t *testing.T) {
	tests := []atomicTest[int64]{
		{
			name: "Basic Operations: Set and Get",
			createFn: func(initialVal int64) sync.AtomicRoot[int64] {
				return NewAtomicInt64(initialVal)
			},
		},
		{
			name: "Basic Operations: Set and Get With Initial Value",
			createFn: func(initialVal int64) sync.AtomicRoot[int64] {
				return NewAtomicInt64(initialVal)
			},
			initialVal: 64,
			setValue:   100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := tt.createFn(tt.initialVal)

			t.Run("Get", func(t *testing.T) {
				a.Get()
			})

			t.Run("Set", func(t *testing.T) {
				a.Set(tt.setValue)
			})
		})
	}
}

func BenchmarkAtomic(b *testing.B) {
}
