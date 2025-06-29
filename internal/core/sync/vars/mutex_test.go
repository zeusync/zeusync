package vars

import (
	"fmt"
	"math/rand/v2"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

type pool[T any] struct {
	sync.Pool
}

func newPool[T any](factory func() T) *pool[T] {
	return &pool[T]{
		Pool: sync.Pool{
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

func Test_TestBaseSyncVariable(t *testing.T) {
	t.Run("BaseSyncVar: Type: int | Mode: Sync | History: None", func(t *testing.T) {
		v := NewMutex(100, 0)

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

	t.Run("BaseSyncVar: Type: int | Mode: Async | History: None", func(t *testing.T) {
		v := NewMutex(0, 0)

		var (
			val  any
			last any
			err  error
		)

		wg := sync.WaitGroup{}

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
func Benchmark_TestBaseSyncVariable(b *testing.B) {
	b.Run("Benchmark_BaseSyncVar: Type: int | Mode: Sync | History: None", func(b *testing.B) {
		v := NewMutex(100, 0)
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_ = v.Set(i) // Избегаем умножения
		}
	})

	b.Run("Benchmark_BaseSyncVar: Type: int | Mode: Async | History: None", func(b *testing.B) {
		v := NewMutex(0, 0)

		// Предварительно сгенерированные значения для избежания rand в горячем пути
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

	b.Run("Benchmark_BaseSyncVar: Type: string | Mode: Sync | History: None", func(b *testing.B) {
		v := NewMutex("", 0)

		// Предварительно сгенерированные строки
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

	b.Run("Benchmark_BaseSyncVar: Type: string | Mode: Async | History: None", func(b *testing.B) {
		v := NewMutex("", 0)

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

	// Дополнительные бенчмарки для проверки разных сценариев
	b.Run("Benchmark_BaseSyncVar: Get operations", func(b *testing.B) {
		v := NewMutex(42, 0)
		b.ResetTimer()
		b.ReportAllocs()

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_, _ = v.Get()
			}
		})
	})

	b.Run("Benchmark_BaseSyncVar: Mixed Read/Write", func(b *testing.B) {
		v := NewMutex(0, 0)
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

// Специализированные бенчмарки для MutexTyped
func Benchmark_TypedSyncVar(b *testing.B) {
	b.Run("MutexTyped[int]: Sync", func(b *testing.B) {
		v := NewMutexTyped[int](0, 0)
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_ = v.Set(i)
		}
	})

	b.Run("MutexTyped[int]: Async", func(b *testing.B) {
		v := NewMutexTyped[int](0, 0)
		b.ResetTimer()
		b.ReportAllocs()

		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				_ = v.Set(i)
				i++
			}
		})
	})

	b.Run("MutexTyped[string]: Sync", func(b *testing.B) {
		v := NewMutexTyped[string]("", 0)

		const poolSize = 100
		strings := make([]string, poolSize)
		for i := range strings {
			strings[i] = fmt.Sprintf("str_%d", i)
		}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_ = v.Set(strings[i%poolSize])
		}
	})
}

// Микро-бенчмарки для отдельных операций
func Benchmark_Operations(b *testing.B) {
	b.Run("fastEqual vs reflect.DeepEqual - int", func(b *testing.B) {
		a, b1 := 42, 42
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_ = fastEqual(a, b1)
		}
	})

	b.Run("reflect.DeepEqual - int", func(b *testing.B) {
		a, b1 := 42, 42
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_ = reflect.DeepEqual(a, b1)
		}
	})

	b.Run("atomic.Value vs mutex", func(b *testing.B) {
		var av atomic.Value
		av.Store(42)

		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = av.Load()
			}
		})
	})
}
