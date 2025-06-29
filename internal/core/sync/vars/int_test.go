package vars

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/require"
)

func Benchmark_TestIntSyncVariable(b *testing.B) {
	b.Run("Benchmark_IntSyncVar: Type: int | Mode: Sync | History: None", func(b *testing.B) {
		v := NewInt(0, 0)

		intPool := newPool(func() int {
			return rand.IntN(math.MaxInt)
		})

		for i := 0; i < b.N; i++ {
			require.NoError(b, v.Set(intPool.Get()))
		}
	})

	b.Run("Benchmark_IntSyncVar: Type: int | Mode: Async | History: None", func(b *testing.B) {
		v := NewMutex(0, 0)

		intPool := newPool(func() int {
			return rand.IntN(math.MaxInt)
		})

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = v.Set(intPool.Get())
			}
		})
	})
}
