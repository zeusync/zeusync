package vars

import (
	"github.com/stretchr/testify/require"
	"testing"
)

const defaultPoolSize = 1_000

func TestAtomic_Atomic(t *testing.T) {
	t.Run("Atomic", func(t *testing.T) {
		t.Run("Int64: Basic Operations", func(t *testing.T) {
			v := NewAtomicInt64(0)

			require.Equal(t, int64(0), v.Get())
			require.True(t, true, v.IsDirty())

			v.Set(10)
			require.Equal(t, int64(10), v.Get())
			require.True(t, true, v.IsDirty())
		})
	})
}

func BenchmarkAtomic(b *testing.B) {
	b.Run("Atomic[int64]", func(b *testing.B) {
		v := NewAtomicInt64(0)

		var values = make([]int64, defaultPoolSize)
		for i := 0; i < defaultPoolSize; i++ {
			values[i] = int64(i)
		}

		b.Run("OpSet", func(b *testing.B) {
			b.Run("Sync", func(b *testing.B) {
				b.ReportAllocs()
				b.ResetTimer()

				for i := 0; i < b.N; i++ {
					v.Set(values[i%defaultPoolSize])
				}
			})

			b.Run("Async", func(b *testing.B) {
				b.ReportAllocs()
				b.ResetTimer()

				b.RunParallel(func(pb *testing.PB) {
					i := 0
					for pb.Next() {
						v.Set(values[i%defaultPoolSize])
						i++
					}
				})
			})

		})

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

		b.Run("OpSwap", func(b *testing.B) {
			b.Run("Sync", func(b *testing.B) {
				b.ReportAllocs()
				b.ResetTimer()

				for i := 0; i < b.N; i++ {
					_ = v.Swap(values[i%defaultPoolSize])
				}
			})

			b.Run("Async", func(b *testing.B) {
				b.ReportAllocs()
				b.ResetTimer()

				b.RunParallel(func(pb *testing.PB) {
					i := 0
					for pb.Next() {
						_ = v.Swap(values[i%defaultPoolSize])
						i++
					}
				})
			})

		})
	})
}
