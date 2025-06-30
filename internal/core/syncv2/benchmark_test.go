package sync_test

import (
	"testing"

	"github.com/zeusync/zeusync/internal/core/sync"
	"github.com/zeusync/zeusync/internal/core/sync/factory"
)

func BenchmarkAtomic(b *testing.B) {
	f := factory.NewVariableFactory()
	v, _ := f.CreateWithStrategy(sync.StrategyAtomic, 0)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = v.Set(1)
			_, _ = v.Get()
		}
	})
}

func BenchmarkMutex(b *testing.B) {
	f := factory.NewVariableFactory()
	v, _ := f.CreateWithStrategy(sync.StrategyMutex, 0)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = v.Set(1)
			_, _ = v.Get()
		}
	})
}
