package fields

import (
	"strconv"
	"testing"
	"time"
)

// BenchmarkBatch benchmarks batch operations
func BenchmarkBatch(b *testing.B) {
	fa := NewFA[int](0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		batch := NewBatch(fa)
		batch.Set(i).Transaction(func(current int) int {
			return current + 1
		})
		batch.Execute()
	}
}

// BenchmarkBatchParallel benchmarks concurrent batch operations
func BenchmarkBatchParallel(b *testing.B) {
	fa := NewFA[int](0)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			batch := NewBatch(fa)
			batch.Set(i).Transaction(func(current int) int {
				return current + 1
			})
			batch.Execute()
			i++
		}
	})
}

// BenchmarkAggregator benchmarks aggregator performance
func BenchmarkAggregator(b *testing.B) {
	target := NewFA[int](0)
	aggregator := NewAggregator(target, func(values []int) int {
		sum := 0
		for _, v := range values {
			sum += v
		}
		return sum
	})
	defer aggregator.Close()

	// Add sources
	const numSources = 10
	sources := make([]FieldAccessor[int], numSources)
	for i := 0; i < numSources; i++ {
		sources[i] = NewFA[int](i)
		aggregator.AddSource(sources[i])
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sourceIndex := i % numSources
		sources[sourceIndex].Set(i)
	}
}

// BenchmarkDebouncer benchmarks debouncer performance
func BenchmarkDebouncer(b *testing.B) {
	source := NewFA[int](0)
	target := NewFA[int](0)
	debouncer := NewDebouncer(source, target, time.Microsecond)
	defer debouncer.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		source.Set(i)
	}
}

// BenchmarkThrottler benchmarks throttler performance
func BenchmarkThrottler(b *testing.B) {
	source := NewFA[int](0)
	target := NewFA[int](0)
	throttler := NewThrottler(source, target, time.Microsecond)
	defer throttler.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		source.Set(i)
	}
}

// BenchmarkValidator benchmarks validator performance
func BenchmarkValidator(b *testing.B) {
	fa := NewFA[int](0)
	validator := NewValidator(fa, func(value int) error {
		if value < 0 {
			return nil // Simple validation
		}
		return nil
	}, func(err error) {
		// Handle error
	})
	defer validator.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fa.Set(i)
	}
}

// BenchmarkTransformer benchmarks transformer performance
func BenchmarkTransformer(b *testing.B) {
	source := NewFA[int](0)
	target := NewFA[string]("")
	transformer := NewTransformer(source, target, func(value int) string {
		return strconv.Itoa(value)
	})
	defer transformer.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		source.Set(i)
	}
}

// BenchmarkSynchronizer benchmarks synchronizer performance
func BenchmarkSynchronizer(b *testing.B) {
	const numAccessors = 5
	accessors := make([]FieldAccessor[int], numAccessors)
	for i := 0; i < numAccessors; i++ {
		accessors[i] = NewFA[int](0)
	}

	synchronizer := NewSynchronizer(accessors...)
	defer synchronizer.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		accessorIndex := i % numAccessors
		accessors[accessorIndex].Set(i)
	}
}

// BenchmarkTimeoutWrapper benchmarks timeout wrapper performance
func BenchmarkTimeoutWrapper(b *testing.B) {
	fa := NewFA[int](0)
	wrapper := WithTimeout(fa, time.Second)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = wrapper.Set(i)
	}
}

// BenchmarkTimeoutWrapperGet benchmarks timeout wrapper get performance
func BenchmarkTimeoutWrapperGet(b *testing.B) {
	fa := NewFA[int](42)
	wrapper := WithTimeout(fa, time.Second)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = wrapper.Get()
		}
	})
}

// BenchmarkTimeoutWrapperTransaction benchmarks timeout wrapper transaction performance
func BenchmarkTimeoutWrapperTransaction(b *testing.B) {
	fa := NewFA[int](0)
	wrapper := WithTimeout(fa, time.Second)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = wrapper.Transaction(func(current int) int {
			return current + 1
		})
	}
}

// BenchmarkMultipleUtilities benchmarks multiple utilities working together
func BenchmarkMultipleUtilities(b *testing.B) {
	source := NewFA[int](0)
	intermediate := NewFA[int](0)
	target := NewFA[string]("")

	// Chain utilities
	debouncer := NewDebouncer(source, intermediate, time.Microsecond)
	defer debouncer.Close()

	transformer := NewTransformer(intermediate, target, func(value int) string {
		return strconv.Itoa(value * 2)
	})
	defer transformer.Close()

	validator := NewValidator(source, func(value int) error {
		return nil // No validation for benchmark
	}, func(err error) {})
	defer validator.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		source.Set(i)
	}
}

// BenchmarkUtilityCreation benchmarks utility creation overhead
func BenchmarkUtilityCreation(b *testing.B) {
	b.Run("Debouncer", func(b *testing.B) {
		source := NewFA[int](0)
		target := NewFA[int](0)

		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			debouncer := NewDebouncer(source, target, time.Microsecond)
			debouncer.Close()
		}
	})

	b.Run("Throttler", func(b *testing.B) {
		source := NewFA[int](0)
		target := NewFA[int](0)

		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			throttler := NewThrottler(source, target, time.Microsecond)
			throttler.Close()
		}
	})

	b.Run("Transformer", func(b *testing.B) {
		source := NewFA[int](0)
		target := NewFA[string]("")

		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			transformer := NewTransformer(source, target, func(value int) string {
				return strconv.Itoa(value)
			})
			transformer.Close()
		}
	})

	b.Run("Validator", func(b *testing.B) {
		fa := NewFA[int](0)

		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			validator := NewValidator(fa, func(value int) error {
				return nil
			}, func(err error) {})
			validator.Close()
		}
	})
}

// BenchmarkUtilityMemoryUsage benchmarks memory usage of utilities
func BenchmarkUtilityMemoryUsage(b *testing.B) {
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		source := NewFA[int](i)
		target := NewFA[int](0)

		// Create and immediately close utilities to test allocation patterns
		debouncer := NewDebouncer(source, target, time.Microsecond)
		debouncer.Close()

		throttler := NewThrottler(source, target, time.Microsecond)
		throttler.Close()

		validator := NewValidator(source, func(value int) error {
			return nil
		}, func(err error) {})
		validator.Close()
	}
}

// BenchmarkComplexUtilityChain benchmarks a complex chain of utilities
func BenchmarkComplexUtilityChain(b *testing.B) {
	// Create a complex chain: source -> debouncer -> validator -> transformer -> aggregator -> target
	source := NewFA[int](0)
	debounced := NewFA[int](0)
	validated := NewFA[int](0)
	transformed := NewFA[string]("")
	target := NewFA[int](0)

	// Set up the chain
	debouncer := NewDebouncer(source, debounced, time.Nanosecond)
	defer debouncer.Close()

	validator := NewValidator(debounced, func(value int) error {
		return nil // No validation for benchmark
	}, func(err error) {})
	defer validator.Close()

	// Copy debounced to validated (validator doesn't modify values)
	copyTransformer := NewTransformer(debounced, validated, func(value int) int {
		return value
	})
	defer copyTransformer.Close()

	stringTransformer := NewTransformer(validated, transformed, func(value int) string {
		return strconv.Itoa(value)
	})
	defer stringTransformer.Close()

	// Create aggregator that sums multiple sources
	aggregator := NewAggregator(target, func(values []int) int {
		sum := 0
		for _, v := range values {
			sum += v
		}
		return sum
	})
	defer aggregator.Close()

	aggregator.AddSource(validated)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		source.Set(i)
	}
}
