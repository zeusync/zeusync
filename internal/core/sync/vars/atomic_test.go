package vars

import (
	"fmt"
	"math/rand/v2"
	"testing"
)

// Оптимизированные бенчмарки
func Benchmark_TestBaseSyncV2Variable(b *testing.B) {
	b.Run("Benchmark_BaseSyncV2Var: Type: int | Mode: Sync | History: None", func(b *testing.B) {
		v := NewSyncVarV2(100, 0)
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_ = v.Set(i) // Избегаем умножения
		}
	})

	b.Run("Benchmark_BaseSyncV2Var: Type: int | Mode: Async | History: None", func(b *testing.B) {
		v := NewSyncVarV2(0, 0)

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

	b.Run("Benchmark_BaseSyncV2Var: Type: string | Mode: Sync | History: None", func(b *testing.B) {
		v := NewSyncVarV2("", 0)

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

	b.Run("Benchmark_BaseSyncVarV2: Type: string | Mode: Async | History: None", func(b *testing.B) {
		v := NewSyncVarV2("", 0)

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
	b.Run("Benchmark_BaseSyncV2Var: Get operations", func(b *testing.B) {
		v := NewSyncVarV2(42, 0)
		b.ResetTimer()
		b.ReportAllocs()

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_, _ = v.Get()
			}
		})
	})

	b.Run("Benchmark_BaseSyncV2Var: Mixed Read/Write", func(b *testing.B) {
		v := NewSyncVarV2(0, 0)
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
func Benchmark_TypedSyncV2Var(b *testing.B) {
	b.Run("TypedSyncV2Var[int]: Sync", func(b *testing.B) {
		v := NewTypedSyncVarV2[int](0, 0)
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_ = v.Set(i)
		}
	})

	b.Run("TypedSyncV2Var[int]: Async", func(b *testing.B) {
		v := NewTypedSyncVarV2[int](0, 0)
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

	b.Run("TypedSyncVarV2[string]: Sync", func(b *testing.B) {
		v := NewTypedSyncVarV2[string]("", 0)

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
