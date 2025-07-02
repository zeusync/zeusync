package vars

import (
	"math/rand"
	"sync"
	"testing"

	sync2 "github.com/zeusync/zeusync/internal/core/syncv2"
)

type channelOperationType uint8

const (
	OpSend channelOperationType = iota
	OpReceive
	OpClose
)

func Benchmark_BufferedChannel(b *testing.B) {
	runChannelBenchmarks(b,
		"BufferedChannel",
		NewBufferedChannel[int](1000, 0),
		func() int { return rand.Intn(1_000) },
		1_000,
	)
}

func Benchmark_UnbufferedChannel(b *testing.B) {
	runChannelBenchmarks[int](b,
		"UnbufferedChannel",
		NewUnbufferedChannel[int](0),
		func() int { return rand.Intn(1_000) },
		1_000,
	)
}

func Benchmark_PriorityChannel(b *testing.B) {
	runChannelBenchmarks[int](b,
		"PriorityChannel",
		NewPriorityChannel[int](1000, 0),
		func() int { return rand.Intn(1_000) },
		1_000,
	)
}

func Benchmark_CycleChannel(b *testing.B) {
	runChannelBenchmarks[int](b,
		"CycleChannel",
		NewCycleChannel[int](1000, 0),
		func() int { return rand.Intn(1_000) },
		1_000,
	)
}

// Comparison benchmarks
func BenchmarkChannel_Comparison(b *testing.B) {
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

	b.Run("CycleChannel", func(b *testing.B) {
		ch := NewCycleChannel[int](100, 0)
		benchmarkChannelOperations(b, ch)
	})
}

// Memory allocation benchmarks
func BenchmarkChannel_Memory(b *testing.B) {
	b.ResetTimer()
	b.Run("BufferedChannel_Alloc", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			ch := NewBufferedChannel[string](10, "test")
			_ = ch
		}
	})

	b.Run("UnbufferedChannel_Alloc", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			ch := NewUnbufferedChannel[string]("test")
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

	b.Run("CycleChannel_Alloc", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			ch := NewCycleChannel[string](10, "test")
			_ = ch
		}
	})
}

// Concurrent access benchmarks
func BenchmarkChannel_Concurrent(b *testing.B) {
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

	b.Run("BroadcastChannel_Concurrent", func(b *testing.B) {
		ch := NewBroadcastChannel[int](1000, 0)
		benchmarkConcurrentAccess(b, ch)
	})

	b.Run("CycleChannel_Concurrent", func(b *testing.B) {
		ch := NewCycleChannel[int](1000, 0)
		benchmarkConcurrentAccess(b, ch)
	})
}

// Throughput benchmarks
func BenchmarkChannel_Throughput(b *testing.B) {
	b.Run("BufferedChannel_Throughput", func(b *testing.B) {
		ch := NewBufferedChannel[int](10000, 0)
		benchmarkThroughput(b, ch)
	})

	b.Run("UnbufferedChannel_Throughput", func(b *testing.B) {
		ch := NewUnbufferedChannel[int](0)
		benchmarkThroughput(b, ch)
	})

	b.Run("PriorityChannel_Throughput", func(b *testing.B) {
		ch := NewPriorityChannel[int](10000, 0)
		benchmarkThroughput(b, ch)
	})

	b.Run("BroadcastChannel_Throughput", func(b *testing.B) {
		ch := NewBroadcastChannel[int](10000, 0)
		benchmarkThroughput(b, ch)
	})

	b.Run("CycleChannel_Throughput", func(b *testing.B) {
		ch := NewCycleChannel[int](10000, 0)
		benchmarkThroughput(b, ch)
	})
}

func benchmarkChannelOperations(b *testing.B, ch interface {
	Get() int
	Set(int)
	Version() uint64
	IsDirty() bool
	MarkClean()
},
) {
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

func benchmarkConcurrentAccess(b *testing.B, ch interface {
	Get() int
	Set(int)
	Send(int) bool
	Receive() (int, bool)
},
) {
	const numGoroutines = 10

	b.ReportAllocs()
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

func benchmarkThroughput(b *testing.B, ch interface {
	Send(int) bool
	Receive() (int, bool)
},
) {
	b.ReportAllocs()
	// Produce
	go func() {
		for i := 0; ; i++ {
			ch.Send(i)
		}
	}()

	b.ResetTimer()

	// Consume
	for i := 0; i < b.N; i++ {
		ch.Receive()
	}
}

func runChannelBenchmarks[T any](
	b *testing.B,
	name string,
	root sync2.ChannelRoot[T],
	valueGenerator func() T,
	poolSize int,
) {
	values := generateValues(valueGenerator, poolSize)

	operations := []struct {
		name string
		op   channelOperationType
	}{
		{"Send", OpSend},
		{"Receive", OpReceive},
		{"Close", OpClose},
	}

	modes := []string{Sync, Async}

	for _, operation := range operations {
		for _, mode := range modes {
			benchName := name + " | " + operation.name + " | " + mode
			b.Run(benchName, func(b *testing.B) {
				runChannelBenchmark(b, root, values, operation.op, mode)
			})
		}
	}
}

func runChannelBenchmark[T any](
	b *testing.B,
	root sync2.ChannelRoot[T],
	values []T,
	operation channelOperationType,
	mode string,
) {
	b.ReportAllocs()
	b.ResetTimer()

	valuesLen := len(values)

	if mode == Sync {
		switch operation {
		case OpSend:
			for i := 0; i < b.N; i++ {
				_ = root.Send(values[i%valuesLen])
			}
		case OpReceive:
			for i := 0; i < b.N; i++ {
				_, _ = root.Receive()
			}
		case OpClose:
			for i := 0; i < b.N; i++ {
				_ = root.Close()
			}
		}
	} else {
		switch operation {
		case OpSend:
			b.RunParallel(func(pb *testing.PB) {
				i := 0
				for pb.Next() {
					_ = root.Send(values[i%valuesLen])
					i++
				}
			})
		case OpReceive:
			b.RunParallel(func(pb *testing.PB) {
				i := 0
				for pb.Next() {
					_, _ = root.Receive()
					i++
				}
			})
		case OpClose:
			b.RunParallel(func(pb *testing.PB) {
				i := 0
				for pb.Next() {
					_ = root.Close()
					i++
				}
			})
		}
	}
}
