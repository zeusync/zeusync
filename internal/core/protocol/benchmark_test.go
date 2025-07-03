package protocol

import (
	"testing"
)

// Benchmark original JSON-based implementation
func BenchmarkOriginalMessage_Marshal(b *testing.B) {
	payload := map[string]interface{}{
		"text":   "Hello, World!",
		"number": 42,
		"array":  []int{1, 2, 3, 4, 5},
		"nested": map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
		},
	}
	message := NewMessage("benchmark", payload)
	message.SetHeader("source", "client1")
	message.SetHeader("target", "client2")

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := message.Marshal()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkOriginalMessage_Unmarshal(b *testing.B) {
	payload := map[string]interface{}{
		"text":   "Hello, World!",
		"number": 42,
		"array":  []int{1, 2, 3, 4, 5},
		"nested": map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
		},
	}
	message := NewMessage("benchmark", payload)
	message.SetHeader("source", "client1")
	message.SetHeader("target", "client2")

	data, err := message.Marshal()
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		newMessage := NewMessage("", nil)
		err := newMessage.Unmarshal(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkOriginalMessage_Clone(b *testing.B) {
	payload := map[string]interface{}{
		"text":   "Hello, World!",
		"number": 42,
		"array":  []int{1, 2, 3, 4, 5},
		"nested": map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
		},
	}
	message := NewMessage("benchmark", payload)
	message.SetHeader("source", "client1")
	message.SetHeader("target", "client2")

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = message.Clone()
	}
}

// Benchmark optimized binary implementation
func BenchmarkOptimizedMessage_Marshal(b *testing.B) {
	payload := map[string]interface{}{
		"text":   "Hello, World!",
		"number": 42,
		"array":  []int{1, 2, 3, 4, 5},
		"nested": map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
		},
	}
	message := NewOptimizedMessage("benchmark", payload)
	message.SetHeader("source", "client1")
	message.SetHeader("target", "client2")

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := message.Marshal()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkOptimizedMessage_Unmarshal(b *testing.B) {
	payload := map[string]interface{}{
		"text":   "Hello, World!",
		"number": 42,
		"array":  []int{1, 2, 3, 4, 5},
		"nested": map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
		},
	}
	message := NewOptimizedMessage("benchmark", payload)
	message.SetHeader("source", "client1")
	message.SetHeader("target", "client2")

	data, err := message.Marshal()
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		newMessage := &OptimizedMessage{}
		err := newMessage.Unmarshal(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkOptimizedMessage_Clone(b *testing.B) {
	payload := map[string]interface{}{
		"text":   "Hello, World!",
		"number": 42,
		"array":  []int{1, 2, 3, 4, 5},
		"nested": map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
		},
	}
	message := NewOptimizedMessage("benchmark", payload)
	message.SetHeader("source", "client1")
	message.SetHeader("target", "client2")

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = message.Clone()
	}
}

// Benchmark small messages
func BenchmarkSmallMessage_Original_Marshal(b *testing.B) {
	message := NewMessage("ping", "pong")

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := message.Marshal()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSmallMessage_Optimized_Marshal(b *testing.B) {
	message := NewOptimizedMessage("ping", "pong")

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := message.Marshal()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSmallMessage_Original_Unmarshal(b *testing.B) {
	message := NewMessage("ping", "pong")
	data, err := message.Marshal()
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		newMessage := NewMessage("", nil)
		err := newMessage.Unmarshal(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSmallMessage_Optimized_Unmarshal(b *testing.B) {
	message := NewOptimizedMessage("ping", "pong")
	data, err := message.Marshal()
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		newMessage := &OptimizedMessage{}
		err := newMessage.Unmarshal(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark large messages
func BenchmarkLargeMessage_Original_Marshal(b *testing.B) {
	// Create a large payload
	largeArray := make([]int, 1000)
	for i := range largeArray {
		largeArray[i] = i
	}

	payload := map[string]interface{}{
		"data": largeArray,
		"metadata": map[string]interface{}{
			"version": "1.0",
			"created": "2023-01-01",
		},
	}

	message := NewMessage("large", payload)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := message.Marshal()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLargeMessage_Optimized_Marshal(b *testing.B) {
	// Create a large payload
	largeArray := make([]int, 1000)
	for i := range largeArray {
		largeArray[i] = i
	}

	payload := map[string]interface{}{
		"data": largeArray,
		"metadata": map[string]interface{}{
			"version": "1.0",
			"created": "2023-01-01",
		},
	}

	message := NewOptimizedMessage("large", payload)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := message.Marshal()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Memory allocation benchmarks
func BenchmarkMessage_CreateAndMarshal_Original(b *testing.B) {
	payload := map[string]interface{}{
		"text": "Hello, World!",
		"num":  42,
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		message := NewMessage("test", payload)
		message.SetHeader("source", "client1")
		_, err := message.Marshal()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMessage_CreateAndMarshal_Optimized(b *testing.B) {
	payload := map[string]interface{}{
		"text": "Hello, World!",
		"num":  42,
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		message := NewOptimizedMessage("test", payload)
		message.SetHeader("source", "client1")
		_, err := message.Marshal()
		if err != nil {
			b.Fatal(err)
		}
	}
}
