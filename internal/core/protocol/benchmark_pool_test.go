package protocol

import (
	"testing"
)

// Benchmark pooled messages
func BenchmarkMessage_CreateAndMarshal_Pooled(b *testing.B) {
	payload := map[string]interface{}{
		"text": "Hello, World!",
		"num":  42,
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		message := NewPooledMessage("test", payload)
		message.SetHeader("source", "client1")
		_, err := message.Marshal()
		if err != nil {
			b.Fatal(err)
		}
		message.Release()
	}
}

func BenchmarkMessage_CreateAndMarshal_PooledOptimized(b *testing.B) {
	payload := map[string]interface{}{
		"text": "Hello, World!",
		"num":  42,
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		message := NewPooledOptimizedMessage("test", payload)
		message.SetHeader("source", "client1")
		_, err := message.Marshal()
		if err != nil {
			b.Fatal(err)
		}
		message.Release()
	}
}

// Benchmark concurrent access
func BenchmarkMessage_Concurrent_Original(b *testing.B) {
	payload := map[string]interface{}{
		"text": "Hello, World!",
		"num":  42,
	}

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			message := NewMessage("test", payload)
			message.SetHeader("source", "client1")
			_, err := message.Marshal()
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkMessage_Concurrent_Pooled(b *testing.B) {
	payload := map[string]interface{}{
		"text": "Hello, World!",
		"num":  42,
	}

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			message := NewPooledMessage("test", payload)
			message.SetHeader("source", "client1")
			_, err := message.Marshal()
			if err != nil {
				b.Fatal(err)
			}
			message.Release()
		}
	})
}

func BenchmarkMessage_Concurrent_PooledOptimized(b *testing.B) {
	payload := map[string]interface{}{
		"text": "Hello, World!",
		"num":  42,
	}

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			message := NewPooledOptimizedMessage("test", payload)
			message.SetHeader("source", "client1")
			_, err := message.Marshal()
			if err != nil {
				b.Fatal(err)
			}
			message.Release()
		}
	})
}
