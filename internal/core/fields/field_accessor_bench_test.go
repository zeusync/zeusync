package fields

import (
	"sync"
	"testing"
)

// BenchmarkFA_Get benchmarks the Get operation
func BenchmarkFA_Get(b *testing.B) {
	fa := NewFA[int](42)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = fa.Get()
		}
	})
}

// BenchmarkFA_Set benchmarks the Set operation
func BenchmarkFA_Set(b *testing.B) {
	fa := NewFA[int](0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fa.Set(i)
	}
}

// BenchmarkFA_SetParallel benchmarks concurrent Set operations
func BenchmarkFA_SetParallel(b *testing.B) {
	fa := NewFA[int](0)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			fa.Set(i)
			i++
		}
	})
}

// BenchmarkFA_CompareAndSwap benchmarks CompareAndSwap operation
func BenchmarkFA_CompareAndSwap(b *testing.B) {
	fa := NewFA[int](0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		current := fa.Get()
		fa.CompareAndSwap(current, i)
	}
}

// BenchmarkFA_CompareAndSwapParallel benchmarks concurrent CompareAndSwap operations
func BenchmarkFA_CompareAndSwapParallel(b *testing.B) {
	fa := NewFA[int](0)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			current := fa.Get()
			fa.CompareAndSwap(current, i)
			i++
		}
	})
}

// BenchmarkFA_Transaction benchmarks Transaction operation
func BenchmarkFA_Transaction(b *testing.B) {
	fa := NewFA[int](0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fa.Transaction(func(current int) int {
			return current + 1
		})
	}
}

// BenchmarkFA_TransactionParallel benchmarks concurrent Transaction operations
func BenchmarkFA_TransactionParallel(b *testing.B) {
	fa := NewFA[int](0)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			fa.Transaction(func(current int) int {
				return current + 1
			})
		}
	})
}

// BenchmarkFA_Subscribe benchmarks subscription creation
func BenchmarkFA_Subscribe(b *testing.B) {
	fa := NewFA[int](0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		unsub := fa.Subscribe(func(newValue int) {
			// Do nothing
		})
		unsub()
	}
}

// BenchmarkFA_SubscribeParallel benchmarks concurrent subscription creation
func BenchmarkFA_SubscribeParallel(b *testing.B) {
	fa := NewFA[int](0)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			unsub := fa.Subscribe(func(newValue int) {
				// Do nothing
			})
			unsub()
		}
	})
}

// BenchmarkFA_NotifySubscribers benchmarks notification performance
func BenchmarkFA_NotifySubscribers(b *testing.B) {
	fa := NewFA[int](0)

	// Add multiple subscribers
	const numSubscribers = 100
	var unsubscribers []func()
	for i := 0; i < numSubscribers; i++ {
		unsub := fa.Subscribe(func(newValue int) {
			// Minimal work to simulate real subscriber
			_ = newValue * 2
		})
		unsubscribers = append(unsubscribers, unsub)
	}
	defer func() {
		for _, unsub := range unsubscribers {
			unsub()
		}
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fa.Set(i)
	}
}

// BenchmarkFA_NotifySubscribersParallel benchmarks concurrent notifications
func BenchmarkFA_NotifySubscribersParallel(b *testing.B) {
	fa := NewFA[int](0)

	// Add multiple subscribers
	const numSubscribers = 100
	var unsubscribers []func()
	for i := 0; i < numSubscribers; i++ {
		unsub := fa.Subscribe(func(newValue int) {
			// Minimal work to simulate real subscriber
			_ = newValue * 2
		})
		unsubscribers = append(unsubscribers, unsub)
	}
	defer func() {
		for _, unsub := range unsubscribers {
			unsub()
		}
	}()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			fa.Set(i)
			i++
		}
	})
}

// BenchmarkFA_History benchmarks History operation
func BenchmarkFA_History(b *testing.B) {
	fa := NewFA[int](0)

	// Populate history
	for i := 0; i < 100; i++ {
		fa.Set(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = fa.History(10)
	}
}

// BenchmarkFA_HistoryParallel benchmarks concurrent History operations
func BenchmarkFA_HistoryParallel(b *testing.B) {
	fa := NewFA[int](0)

	// Populate history
	for i := 0; i < 100; i++ {
		fa.Set(i)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = fa.History(10)
		}
	})
}

// BenchmarkFA_Version benchmarks Version operation
func BenchmarkFA_Version(b *testing.B) {
	fa := NewFA[int](0)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = fa.Version()
		}
	})
}

// BenchmarkFA_ChangedSince benchmarks ChangedSince operation
func BenchmarkFA_ChangedSince(b *testing.B) {
	fa := NewFA[int](0)
	version := fa.Version()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = fa.ChangedSince(version)
		}
	})
}

// BenchmarkFA_Serialize benchmarks serialization
func BenchmarkFA_Serialize(b *testing.B) {
	fa := NewFA[int](42)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = fa.Serialize()
	}
}

// BenchmarkFA_Deserialize benchmarks deserialization
func BenchmarkFA_Deserialize(b *testing.B) {
	fa := NewFA[int](42)
	data, _ := fa.Serialize()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		newFA := NewFA[int]()
		_ = newFA.Deserialize(data)
	}
}

// BenchmarkFA_Clone benchmarks cloning operation
func BenchmarkFA_Clone(b *testing.B) {
	fa := NewFA[int](42)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = fa.Clone()
	}
}

// BenchmarkFA_MixedOperations benchmarks a realistic mixed workload
func BenchmarkFA_MixedOperations(b *testing.B) {
	fa := NewFA[int](0)

	// Add some subscribers
	const numSubscribers = 10
	var unsubscribers []func()
	for i := 0; i < numSubscribers; i++ {
		unsub := fa.Subscribe(func(newValue int) {
			_ = newValue * 2
		})
		unsubscribers = append(unsubscribers, unsub)
	}
	defer func() {
		for _, unsub := range unsubscribers {
			unsub()
		}
	}()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			switch i % 4 {
			case 0:
				fa.Set(i)
			case 1:
				_ = fa.Get()
			case 2:
				fa.Transaction(func(current int) int {
					return current + 1
				})
			case 3:
				_ = fa.Version()
			}
			i++
		}
	})
}

// BenchmarkFA_HighContentionScenario benchmarks high contention scenario
func BenchmarkFA_HighContentionScenario(b *testing.B) {
	fa := NewFA[int64](0)
	const numGoroutines = 100

	b.ResetTimer()

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	start := make(chan struct{})

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			<-start

			for j := 0; j < b.N/numGoroutines; j++ {
				fa.Transaction(func(current int64) int64 {
					return current + 1
				})
			}
		}()
	}

	close(start)
	wg.Wait()
}

// BenchmarkFA_MemoryUsage benchmarks memory usage patterns
func BenchmarkFA_MemoryUsage(b *testing.B) {
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		fa := NewFA[int](i)
		fa.Set(i * 2)
		_ = fa.Get()

		// Add and remove subscriber to test allocation patterns
		unsub := fa.Subscribe(func(newValue int) {})
		unsub()
	}
}
