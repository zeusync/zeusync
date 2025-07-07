package protocol

import (
	"github.com/google/uuid"
	"testing"
)

func Benchmark_CustomUUI(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = generateUUID()
	}
}

func Benchmark_OptimazedCustomUUI(b *testing.B) {
	g := NewOptimizedUUIDGenerator(1_000)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = g.Generate()
	}
}

func Benchmark_StandardUUID(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = uuid.NewString()
	}
}
