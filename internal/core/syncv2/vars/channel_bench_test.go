package vars

import (
	"sync"
	"testing"
)

func BenchmarkBufferedChannelVar_Send(b *testing.B) {
	ch := NewBufferedChannel[int](1000, 0)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			ch.Send(i)
			i++
		}
	})
}

func BenchmarkBufferedChannelVar_Receive(b *testing.B) {
	ch := NewBufferedChannel[int](1000, 0)

	// Pre-fill the channel
	for i := 0; i < 1000; i++ {
		ch.Send(i)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, ok := ch.Receive(); !ok {
				// Refill if empty
				for i := 0; i < 100; i++ {
					ch.Send(i)
				}
			}
		}
	})
}

func BenchmarkBufferedChannelVar_GetSet(b *testing.B) {
	ch := NewBufferedChannel[int](10, 0)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			ch.Set(i)
			_ = ch.Get()
			i++
		}
	})
}

func BenchmarkPriorityChannelVar_Send(b *testing.B) {
	ch := NewPriorityChannel[int](1000, 0)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			ch.SendWithPriority(i, i%10)
			i++
		}
	})
}

func BenchmarkPriorityChannelVar_Receive(b *testing.B) {
	ch := NewPriorityChannel[int](1000, 0)

	// Pre-fill the channel
	for i := 0; i < 1000; i++ {
		ch.SendWithPriority(i, i%10)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, ok := ch.Receive(); !ok {
				// Refill if empty
				for i := 0; i < 100; i++ {
					ch.SendWithPriority(i, i%10)
				}
			}
		}
	})
}

func BenchmarkBroadcastChannelVar_Send(b *testing.B) {
	ch := NewBroadcastChannel[int](10, 0)

	// Add some subscribers
	for i := 0; i < 5; i++ {
		sub := ch.Subscribe()
		go func() {
			for range sub {
				// Consume messages
			}
		}()
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			ch.Send(i)
			i++
		}
	})
}

func BenchmarkUnbufferedChannelVar_GetSet(b *testing.B) {
	ch := NewUnbufferedChannel[int](0)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			ch.Set(i)
			_ = ch.Get()
			i++
		}
	})
}

// Comparison benchmarks
func BenchmarkChannelVar_Comparison(b *testing.B) {
	b.Run("BufferedChannel", func(b *testing.B) {
		ch := NewBufferedChannel[int](100, 0)
		benchmarkChannelOperations(b, ch)
	})

	b.Run("UnbufferedChannel", func(b *testing.B) {
		ch := NewUnbufferedChannel[int](0)
		benchmarkChannelOperations(b, ch)
	})

	b.Run("PriorityChannel", func(b *testing.B) {
		ch := NewPriorityChannel[int](100, 0)
		benchmarkChannelOperations(b, ch)
	})

	b.Run("BroadcastChannel", func(b *testing.B) {
		ch := NewBroadcastChannel[int](100, 0)
		benchmarkChannelOperations(b, ch)
	})
}

func benchmarkChannelOperations(b *testing.B, ch interface {
	Get() int
	Set(int)
	Version() uint64
	IsDirty() bool
	MarkClean()
}) {
	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			ch.Set(i)
			_ = ch.Get()
			_ = ch.Version()
			if ch.IsDirty() {
				ch.MarkClean()
			}
			i++
		}
	})
}

// Memory allocation benchmarks
func BenchmarkChannelVar_Memory(b *testing.B) {
	b.ResetTimer()
	b.Run("BufferedChannel_Alloc", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			ch := NewBufferedChannel[string](10, "test")
			_ = ch
		}
	})

	b.Run("PriorityChannel_Alloc", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			ch := NewPriorityChannel[string](10, "test")
			_ = ch
		}
	})

	b.Run("BroadcastChannel_Alloc", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			ch := NewBroadcastChannel[string](10, "test")
			_ = ch
		}
	})
}

// Concurrent access benchmarks
func BenchmarkChannelVar_Concurrent(b *testing.B) {
	b.Run("BufferedChannel_Concurrent", func(b *testing.B) {
		ch := NewBufferedChannel[int](1000, 0)
		benchmarkConcurrentAccess(b, ch)
	})

	b.Run("UnbufferedChannel_Concurrent", func(b *testing.B) {
		ch := NewUnbufferedChannel[int](1000)
		benchmarkConcurrentAccess(b, ch)
	})

	b.Run("PriorityChannel_Concurrent", func(b *testing.B) {
		ch := NewPriorityChannel[int](1000, 0)
		benchmarkConcurrentAccess(b, ch)
	})
}

func benchmarkConcurrentAccess(b *testing.B, ch interface {
	Get() int
	Set(int)
	Send(int) bool
	Receive() (int, bool)
}) {
	const numGoroutines = 10

	b.ResetTimer()

	var wg sync.WaitGroup

	// Writers
	for i := 0; i < numGoroutines/2; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < b.N/numGoroutines; j++ {
				ch.Set(id*1000 + j)
				ch.Send(id*1000 + j)
			}
		}(i)
	}

	// Readers
	for i := 0; i < numGoroutines/2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < b.N/numGoroutines; j++ {
				_ = ch.Get()
				ch.Receive()
			}
		}()
	}

	wg.Wait()
}

// Throughput benchmarks
func BenchmarkChannelVar_Throughput(b *testing.B) {
	b.Run("BufferedChannel_Throughput", func(b *testing.B) {
		ch := NewBufferedChannel[int](10000, 0)
		benchmarkThroughput(b, ch)
	})

	b.Run("PriorityChannel_Throughput", func(b *testing.B) {
		ch := NewPriorityChannel[int](10000, 0)
		benchmarkThroughput(b, ch)
	})
}

func benchmarkThroughput(b *testing.B, ch interface {
	Send(int) bool
	Receive() (int, bool)
}) {
	// Producer
	go func() {
		for i := 0; ; i++ {
			ch.Send(i)
		}
	}()

	b.ResetTimer()

	// Consumer
	for i := 0; i < b.N; i++ {
		ch.Receive()
	}
}
