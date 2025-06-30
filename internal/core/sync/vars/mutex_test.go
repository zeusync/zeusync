package vars

import (
	"fmt"
	"github.com/zeusync/zeusync/internal/core/sync"
	"math/rand/v2"
	sc "sync"
	"testing"

	"github.com/stretchr/testify/require"
)

type pool[T any] struct {
	sc.Pool
}

func newPool[T any](factory func() T) *pool[T] {
	return &pool[T]{
		Pool: sc.Pool{
			New: func() any {
				return factory()
			},
		},
	}
}

func (p *pool[T]) Get() T {
	return p.Pool.Get().(T)
}

func (p *pool[T]) Put(value T) {
	p.Pool.Put(value)
}

func Test_MutexVariable(t *testing.T) {
	t.Run("Mutex[int]: Mode: Sync | History: None", func(t *testing.T) {
		cfg := sync.VariableConfig{
			MaxHistory:    0,
			EnableHistory: false,
		}

		v := NewMutexVariable(cfg)

		var (
			val any
			err error
		)

		val, err = v.Get()
		require.NoError(t, err)
		require.Equal(t, 100, val)

		err = v.Set(120)
		require.NoError(t, err)

		require.True(t, v.IsDirty())
	})

	t.Run("Mutex[int]: Mode: Async | History: None", func(t *testing.T) {
		cfg := sync.VariableConfig{
			MaxHistory:    0,
			EnableHistory: false,
		}

		v := NewMutexVariable(cfg)

		var (
			val  any
			last any
			err  error
		)

		wg := sc.WaitGroup{}

		times := 5

		for i := 1; i < times+1; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				aErr := v.Set(i * 100)
				require.NoError(t, aErr)

				last, aErr = v.Get()
			}(i)
		}

		wg.Wait()

		val, err = v.Get()
		require.NoError(t, err)
		require.Equal(t, last, val)
	})
}

// Оптимизированные бенчмарки
func Benchmark_MutexVariable(b *testing.B) {
	b.Run("Mutex[int]: Mode: Sync | History: None", func(b *testing.B) {
		cfg := sync.VariableConfig{
			MaxHistory:    0,
			EnableHistory: false,
		}

		v := NewMutexVariable(cfg)
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_ = v.Set(i)
		}
	})

	b.Run("Mutex[int]: Mode: Async | History: None", func(b *testing.B) {
		cfg := sync.VariableConfig{
			MaxHistory:    0,
			EnableHistory: false,
		}

		v := NewMutexVariable(cfg)

		const poolSize = 1000
		values := make([]int, poolSize)
		for i := range values {
			values[i] = rand.IntN(1000000)
		}

		b.ResetTimer()
		b.ReportAllocs()

		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				_ = v.Set(values[i%poolSize])
				i++
			}
		})
	})

	b.Run("Mutex[string]: Mode: Sync | History: None", func(b *testing.B) {
		cfg := sync.VariableConfig{
			MaxHistory:    0,
			EnableHistory: false,
		}

		v := NewMutexVariable(cfg)

		const poolSize = 100
		strings := make([]string, poolSize)
		for i := range strings {
			strings[i] = fmt.Sprintf("test_string_%d", i)
		}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_ = v.Set(strings[i%poolSize])
		}
	})

	b.Run("Mutex[string]: Mode: Async | History: None", func(b *testing.B) {
		cfg := sync.VariableConfig{
			MaxHistory:    0,
			EnableHistory: false,
		}

		v := NewMutexVariable(cfg)

		// Предварительно сгенерированные строки
		const poolSize = 100
		strings := make([]string, poolSize)
		for i := range strings {
			strings[i] = fmt.Sprintf("test_string_%d", i)
		}

		b.ResetTimer()
		b.ReportAllocs()

		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				_ = v.Set(strings[i%poolSize])
				i++
			}
		})
	})

	b.Run("Mutex[struct]: Mode: Sync | History: None", func(b *testing.B) {
		cfg := sync.VariableConfig{
			MaxHistory:    0,
			EnableHistory: false,
		}

		v := NewMutexVariable(cfg)

		type obj struct {
			ID   int
			Name string
			Tags []string
			Age  uint8
		}

		const poolSize = 100
		structs := make([]obj, poolSize)
		for i := range structs {
			structs[i] = obj{
				ID:   i,
				Name: fmt.Sprintf("name_%d", i),
				Tags: []string{fmt.Sprintf("tag_%d", i), fmt.Sprintf("tag_%d", i+1), fmt.Sprintf("tag_%d", i+2)},
				Age:  uint8(i),
			}
		}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_ = v.Set(structs[i%poolSize])
		}
	})

	b.Run("Mutex[struct]: Mode: Async | History: None", func(b *testing.B) {
		cfg := sync.VariableConfig{
			MaxHistory:    0,
			EnableHistory: false,
		}

		type obj struct {
			ID   int
			Name string
			Tags []string
			Age  uint8
		}

		const poolSize = 100
		structs := make([]obj, poolSize)
		for i := range structs {
			structs[i] = obj{
				ID:   i,
				Name: fmt.Sprintf("name_%d", i),
				Tags: []string{fmt.Sprintf("tag_%d", i), fmt.Sprintf("tag_%d", i+1), fmt.Sprintf("tag_%d", i+2)},
				Age:  uint8(i),
			}
		}

		v := NewMutexVariable(cfg)

		b.ResetTimer()
		b.ReportAllocs()

		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				_ = v.Set(structs[i%poolSize])
				i++
			}
		})
	})

	b.Run("MutexTyped[int]: Mode: Sync | History: None", func(b *testing.B) {
		cfg := sync.VariableConfig{
			MaxHistory:    0,
			EnableHistory: false,
		}

		v := NewMutexTypedVariable[int](cfg)
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_ = v.Set(i)
		}
	})

	b.Run("MutexTyped[int]: Mode: Async | History: None", func(b *testing.B) {
		cfg := sync.VariableConfig{
			MaxHistory:    0,
			EnableHistory: false,
		}

		v := NewMutexTypedVariable[int](cfg)

		const poolSize = 1000
		values := make([]int, poolSize)
		for i := range values {
			values[i] = rand.IntN(1000000)
		}

		b.ResetTimer()
		b.ReportAllocs()

		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				_ = v.Set(values[i%poolSize])
				i++
			}
		})
	})

	b.Run("MutexTyped[string]: Mode: Sync | History: None", func(b *testing.B) {
		cfg := sync.VariableConfig{
			MaxHistory:    0,
			EnableHistory: false,
		}

		v := NewMutexTypedVariable[string](cfg)

		const poolSize = 100
		strings := make([]string, poolSize)
		for i := range strings {
			strings[i] = fmt.Sprintf("test_string_%d", i)
		}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_ = v.Set(strings[i%poolSize])
		}
	})

	b.Run("MutexTyped[string]: Mode: Async | History: None", func(b *testing.B) {
		cfg := sync.VariableConfig{
			MaxHistory:    0,
			EnableHistory: false,
		}

		v := NewMutexTypedVariable[string](cfg)

		// Предварительно сгенерированные строки
		const poolSize = 100
		strings := make([]string, poolSize)
		for i := range strings {
			strings[i] = fmt.Sprintf("test_string_%d", i)
		}

		b.ResetTimer()
		b.ReportAllocs()

		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				_ = v.Set(strings[i%poolSize])
				i++
			}
		})
	})

	b.Run("MutexTyped[struct]: Mode: Sync | History: None", func(b *testing.B) {
		cfg := sync.VariableConfig{
			MaxHistory:    0,
			EnableHistory: false,
		}

		type obj struct {
			ID   int
			Name string
			Tags []string
			Age  uint8
		}

		v := NewMutexTypedVariable[obj](cfg)

		const poolSize = 100
		structs := make([]obj, poolSize)
		for i := range structs {
			structs[i] = obj{
				ID:   i,
				Name: fmt.Sprintf("name_%d", i),
				Tags: []string{fmt.Sprintf("tag_%d", i), fmt.Sprintf("tag_%d", i+1), fmt.Sprintf("tag_%d", i+2)},
				Age:  uint8(i),
			}
		}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_ = v.Set(structs[i%poolSize])
		}
	})

	b.Run("MutexTyped[struct]: Mode: Async | History: None", func(b *testing.B) {
		cfg := sync.VariableConfig{
			MaxHistory:    0,
			EnableHistory: false,
		}

		type obj struct {
			ID   int
			Name string
			Tags []string
			Age  uint8
		}

		v := NewMutexTypedVariable[obj](cfg)

		const poolSize = 100
		structs := make([]obj, poolSize)
		for i := range structs {
			structs[i] = obj{
				ID:   i,
				Name: fmt.Sprintf("name_%d", i),
				Tags: []string{fmt.Sprintf("tag_%d", i), fmt.Sprintf("tag_%d", i+1), fmt.Sprintf("tag_%d", i+2)},
				Age:  uint8(i),
			}
		}

		b.ResetTimer()
		b.ReportAllocs()

		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				_ = v.Set(structs[i%poolSize])
				i++
			}
		})
	})

	b.Run("Mutex: Get operations", func(b *testing.B) {
		cfg := sync.VariableConfig{
			MaxHistory:    0,
			EnableHistory: false,
		}

		v := NewMutexVariable(cfg)
		b.ResetTimer()
		b.ReportAllocs()

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_, _ = v.Get()
			}
		})
	})

	b.Run("Mutex: Mixed Read/Write", func(b *testing.B) {
		cfg := sync.VariableConfig{
			MaxHistory:    0,
			EnableHistory: false,
		}

		v := NewMutexVariable(cfg)
		b.ResetTimer()
		b.ReportAllocs()

		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				if i%10 == 0 { // 10% записей, 90% чтений
					_ = v.Set(i)
				} else {
					_, _ = v.Get()
				}
				i++
			}
		})
	})
}
