package vars

import (
	"context"
	"fmt"
	"github.com/zeusync/zeusync/internal/core/syncv2/types"
	"strconv"
	"testing"
	"time"
)

func BenchmarkSharded(b *testing.B) {
	var keys = make([]string, types.defaultPoolSize)
	var values = make([]int, types.defaultPoolSize)
	for i := 0; i < types.defaultPoolSize; i++ {
		keys[i] = fmt.Sprintf("key-%d", i)
		values[i] = i
	}

	b.Run("HashSharded", func(b *testing.B) {
		benchmarkShardedImpl(b, NewHashSharded(16, 0), keys, values)
	})

	b.Run("ConsistentHashSharded", func(b *testing.B) {
		benchmarkShardedImpl(b, NewConsistentHashSharded(16, 0), keys, values)
	})

	b.Run("RangeSharded", func(b *testing.B) {
		benchmarkShardedImpl(b, NewRangeSharded([]string{"2", "4", "6", "8"}, 0), keys, values)
	})

	b.Run("AdaptiveSharded", func(b *testing.B) {
		v := NewAdaptiveSharded(16, 0)
		v.SetResizeThreshold(10_000) // High threshold to avoid resize during benchmark
		benchmarkShardedImpl(b, v, keys, values)
	})
}

func benchmarkShardedImpl(b *testing.B, v interface {
	Get() int
	Set(int)
	GetShard(string) int
	SetShard(string, int)
	GetShardCount() int
	ForEachShard(func(int, int) bool)
	MergeShard(func([]int) int) int
}, keys []string, values []int) {

	b.Run("OpGet", func(b *testing.B) {
		b.Run("Sync", func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_ = v.Get()
			}
		})

		b.Run("Async", func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					_ = v.Get()
				}
			})
		})
	})

	b.Run("OpSet", func(b *testing.B) {
		b.Run("Sync", func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				v.Set(values[i%types.defaultPoolSize])
			}
		})

		b.Run("Async", func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			b.RunParallel(func(pb *testing.PB) {
				i := 0
				for pb.Next() {
					v.Set(values[i%types.defaultPoolSize])
					i++
				}
			})
		})
	})

	b.Run("OpGetShard", func(b *testing.B) {
		b.Run("Sync", func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_ = v.GetShard(keys[i%types.defaultPoolSize])
			}
		})

		b.Run("Async", func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			b.RunParallel(func(pb *testing.PB) {
				i := 0
				for pb.Next() {
					_ = v.GetShard(keys[i%types.defaultPoolSize])
					i++
				}
			})
		})
	})

	b.Run("OpSetShard", func(b *testing.B) {
		b.Run("Sync", func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				v.SetShard(keys[i%types.defaultPoolSize], values[i%types.defaultPoolSize])
			}
		})

		b.Run("Async", func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			b.RunParallel(func(pb *testing.PB) {
				i := 0
				for pb.Next() {
					v.SetShard(keys[i%types.defaultPoolSize], values[i%types.defaultPoolSize])
					i++
				}
			})
		})
	})

	b.Run("OpForEachShard", func(b *testing.B) {
		b.Run("Sync", func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				v.ForEachShard(func(shardIndex int, value int) bool {
					return true // Continue iteration
				})
			}
		})

		b.Run("Async", func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					v.ForEachShard(func(shardIndex int, value int) bool {
						return true // Continue iteration
					})
				}
			})
		})
	})

	b.Run("OpMergeShard", func(b *testing.B) {
		mergeFunc := func(values []int) int {
			sum := 0
			for _, val := range values {
				sum += val
			}
			return sum
		}

		b.Run("Sync", func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_ = v.MergeShard(mergeFunc)
			}
		})

		b.Run("Async", func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					_ = v.MergeShard(mergeFunc)
				}
			})
		})
	})
}

func BenchmarkSharded_ShardDistribution(b *testing.B) {
	shardCounts := []int{4, 8, 16, 32, 64}

	for _, shardCount := range shardCounts {
		b.Run(fmt.Sprintf("HashSharded-%d-shards", shardCount), func(b *testing.B) {
			v := NewHashSharded(shardCount, 0)
			benchmarkShardDistribution(b, v, shardCount)
		})

		b.Run(fmt.Sprintf("ConsistentHashSharded-%d-shards", shardCount), func(b *testing.B) {
			v := NewConsistentHashSharded(shardCount, 0)
			benchmarkShardDistribution(b, v, shardCount)
		})
	}
}

func benchmarkShardDistribution(b *testing.B, v interface {
	GetShard(string) int
	SetShard(string, int)
}, _ int) {
	keys := make([]string, b.N)
	for i := 0; i < b.N; i++ {
		keys[i] = strconv.Itoa(i)
	}

	runCtx, cancel := context.WithTimeout(b.Context(), time.Second*15)
	defer cancel()

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			select {
			case <-runCtx.Done():
				b.SkipNow()
				return
			}

			key := keys[i%len(keys)]
			v.SetShard(key, i)
			_ = v.GetShard(key)
			i++
		}
	})
}

func BenchmarkSharded_ContentionLevels(b *testing.B) {
	contentionLevels := []int{1, 2, 4, 8, 16, 32}

	for _, level := range contentionLevels {
		b.Run(fmt.Sprintf("HashSharded-contention-%d", level), func(b *testing.B) {
			v := NewHashSharded(16, 0)
			benchmarkContention(b, v, level)
		})

		b.Run(fmt.Sprintf("AdaptiveSharded-contention-%d", level), func(b *testing.B) {
			v := NewAdaptiveSharded(16, 0)
			v.SetResizeThreshold(100000) // High threshold
			benchmarkContention(b, v, level)
		})
	}
}

func benchmarkContention(b *testing.B, v interface {
	GetShard(string) int
	SetShard(string, int)
}, contentionLevel int) {
	// Create limited set of keys to increase contention
	keys := make([]string, contentionLevel)
	for i := 0; i < contentionLevel; i++ {
		keys[i] = fmt.Sprintf("contended-key-%d", i)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := keys[i%len(keys)]
			v.SetShard(key, i)
			_ = v.GetShard(key)
			i++
		}
	})
}

func BenchmarkSharded_MergeOperations(b *testing.B) {
	mergeFunctions := map[string]func([]int) int{
		"Sum": func(values []int) int {
			sum := 0
			for _, val := range values {
				sum += val
			}
			return sum
		},
		"Max": func(values []int) int {
			if len(values) == 0 {
				return 0
			}
			m := values[0]
			for _, val := range values {
				if val > m {
					m = val
				}
			}
			return m
		},
		"Count": func(values []int) int {
			return len(values)
		},
		"Average": func(values []int) int {
			if len(values) == 0 {
				return 0
			}
			sum := 0
			for _, val := range values {
				sum += val
			}
			return sum / len(values)
		},
	}

	implementations := map[string]func() interface {
		MergeShard(func([]int) int) int
		SetShard(string, int)
	}{
		"HashSharded": func() interface {
			MergeShard(func([]int) int) int
			SetShard(string, int)
		} {
			v := NewHashSharded(16, 0)
			// Pre-populate with some data
			for i := 0; i < 100; i++ {
				v.SetShard(fmt.Sprintf("key-%d", i), i)
			}
			return v
		},
		"ConsistentHashSharded": func() interface {
			MergeShard(func([]int) int) int
			SetShard(string, int)
		} {
			v := NewConsistentHashSharded(16, 0)
			// Pre-populate with some data
			for i := 0; i < 100; i++ {
				v.SetShard(fmt.Sprintf("key-%d", i), i)
			}
			return v
		},
		"RangeSharded": func() interface {
			MergeShard(func([]int) int) int
			SetShard(string, int)
		} {
			v := NewRangeSharded([]string{"2", "4", "6", "8"}, 0)
			// Pre-populate with some data
			for i := 0; i < 100; i++ {
				v.SetShard(fmt.Sprintf("key-%d", i), i)
			}
			return v
		},
		"AdaptiveSharded": func() interface {
			MergeShard(func([]int) int) int
			SetShard(string, int)
		} {
			v := NewAdaptiveSharded(16, 0)
			v.SetResizeThreshold(100000) // High threshold
			// Pre-populate with some data
			for i := 0; i < 100; i++ {
				v.SetShard(fmt.Sprintf("key-%d", i), i)
			}
			return v
		},
	}

	for implName, implFactory := range implementations {
		for mergeName, mergeFunc := range mergeFunctions {
			b.Run(fmt.Sprintf("%s-%s", implName, mergeName), func(b *testing.B) {
				v := implFactory()

				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					_ = v.MergeShard(mergeFunc)
				}
			})
		}
	}
}

func BenchmarkSharded_AdaptiveResize(b *testing.B) {
	b.Run("AdaptiveSharded-Resize", func(b *testing.B) {
		v := NewAdaptiveSharded(2, 0)
		v.SetResizeThreshold(100) // Low threshold to trigger resize

		keys := make([]string, 1000)
		for i := 0; i < 1000; i++ {
			keys[i] = fmt.Sprintf("key-%d", i%10) // Limited key set to trigger resize
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := keys[i%len(keys)]
			v.SetShard(key, i)
			_ = v.GetShard(key)
		}
	})
}
