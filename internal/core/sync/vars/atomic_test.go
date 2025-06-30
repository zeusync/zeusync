package vars

import (
	"fmt"
	"github.com/zeusync/zeusync/internal/core/sync"
	"math/rand/v2"
	"testing"
)

// Оптимизированные бенчмарки
func Benchmark_AtomicVariable(b *testing.B) {
	b.Run("Atomic[int]: Mode: Sync | History: None", func(b *testing.B) {
		cfg := sync.VariableConfig{
			MaxHistory:    0,
			EnableHistory: false,
		}

		v := NewAtomicVariable(cfg)
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_ = v.Set(i) // Избегаем умножения
		}
	})

	b.Run("Atomic[int]: Mode: Async | History: None", func(b *testing.B) {
		cfg := sync.VariableConfig{
			MaxHistory:    0,
			EnableHistory: false,
		}

		v := NewAtomicVariable(cfg)

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

	b.Run("Atomic[string]: Mode: Sync | History: None", func(b *testing.B) {
		cfg := sync.VariableConfig{
			MaxHistory:    0,
			EnableHistory: false,
		}

		v := NewAtomicVariable(cfg)

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

	b.Run("Atomic[string]: Mode: Async | History: None", func(b *testing.B) {
		cfg := sync.VariableConfig{
			MaxHistory:    0,
			EnableHistory: false,
		}

		v := NewAtomicVariable(cfg)

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

	b.Run("Atomic[struct]: Mode: Sync | History: None", func(b *testing.B) {
		cfg := sync.VariableConfig{
			MaxHistory:    0,
			EnableHistory: false,
		}

		v := NewAtomicVariable(cfg)

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

	b.Run("Atomic[struct]: Mode: Async | History: None", func(b *testing.B) {
		cfg := sync.VariableConfig{
			MaxHistory:    0,
			EnableHistory: false,
		}

		v := NewAtomicVariable(cfg)

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

		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				_ = v.Set(structs[i%poolSize])
				i++
			}
		})
	})

	b.Run("Atomic: Get operations", func(b *testing.B) {
		cfg := sync.VariableConfig{
			MaxHistory:    0,
			EnableHistory: false,
		}

		v := NewAtomicVariable(cfg)
		b.ResetTimer()
		b.ReportAllocs()

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_, _ = v.Get()
			}
		})
	})

	b.Run("Atomic: Mixed Read/Write", func(b *testing.B) {
		cfg := sync.VariableConfig{
			MaxHistory:    0,
			EnableHistory: false,
		}

		v := NewAtomicVariable(cfg)
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

	//  ======= TYPED ======= //

	b.Run("AtomicTyped[int]: Mode: Sync | History: None", func(b *testing.B) {
		cfg := sync.VariableConfig{
			MaxHistory:    0,
			EnableHistory: false,
		}

		v := NewAtomicTypedVariable[int](cfg)
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_ = v.Set(i)
		}
	})

	b.Run("AtomicTyped[int]: Mode: Async | History: None", func(b *testing.B) {
		cfg := sync.VariableConfig{
			MaxHistory:    0,
			EnableHistory: false,
		}

		v := NewAtomicTypedVariable[int](cfg)

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

	b.Run("AtomicTyped[string]: Mode: Sync | History: None", func(b *testing.B) {
		cfg := sync.VariableConfig{
			MaxHistory:    0,
			EnableHistory: false,
		}

		v := NewAtomicTypedVariable[string](cfg)

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

	b.Run("AtomicTyped[string]: Mode: Async | History: None", func(b *testing.B) {
		cfg := sync.VariableConfig{
			MaxHistory:    0,
			EnableHistory: false,
		}

		v := NewAtomicTypedVariable[string](cfg)

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

	b.Run("AtomicTyped[struct]: Mode: Sync | History: None", func(b *testing.B) {
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

		v := NewAtomicTypedVariable[obj](cfg)

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

	b.Run("AtomicTyped[struct]: Mode: Async | History: None", func(b *testing.B) {
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

		v := NewAtomicTypedVariable[obj](cfg)

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

	b.Run("Atomic: Get operations", func(b *testing.B) {
		cfg := sync.VariableConfig{
			MaxHistory:    0,
			EnableHistory: false,
		}

		v := NewAtomicVariable(cfg)
		b.ResetTimer()
		b.ReportAllocs()

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_, _ = v.Get()
			}
		})
	})

	b.Run("Atomic: Mixed Read/Write", func(b *testing.B) {
		cfg := sync.VariableConfig{
			MaxHistory:    0,
			EnableHistory: false,
		}

		v := NewAtomicVariable(cfg)
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
