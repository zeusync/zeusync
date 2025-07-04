package protocol

import (
	"testing"
)

func TestBinaryMessage_MarshalUnmarshal(t *testing.T) {
	// Создаем тестовое сообщение
	original := NewMessage("test_type", []byte("test payload")).(*BinaryMessage)
	original.SetHeader("key1", "value1")
	original.SetHeader("key2", "value2")

	// Сериализуем
	data, err := original.Marshal()
	if err != nil {
		t.Fatalf("Failed to marshal message: %v", err)
	}

	// Десериализуем
	restored := &BinaryMessage{}
	err = restored.Unmarshal(data)
	if err != nil {
		t.Fatalf("Failed to unmarshal message: %v", err)
	}

	// Проверяем поля
	if restored.Type() != original.Type() {
		t.Errorf("Type mismatch: got %s, want %s", restored.Type(), original.Type())
	}

	if string(restored.Payload()) != string(original.Payload()) {
		t.Errorf("Payload mismatch: got %s, want %s", string(restored.Payload()), string(original.Payload()))
	}

	if restored.GetHeader("key1") != original.GetHeader("key1") {
		t.Errorf("Header key1 mismatch: got %s, want %s", restored.GetHeader("key1"), original.GetHeader("key1"))
	}

	if restored.GetHeader("key2") != original.GetHeader("key2") {
		t.Errorf("Header key2 mismatch: got %s, want %s", restored.GetHeader("key2"), original.GetHeader("key2"))
	}
}

func TestBinaryMessage_EmptyPayload(t *testing.T) {
	// Тестируем сообщение с пустым payload
	original := NewMessage("empty_test", nil).(*BinaryMessage)

	data, err := original.Marshal()
	if err != nil {
		t.Fatalf("Failed to marshal empty message: %v", err)
	}

	restored := &BinaryMessage{}
	err = restored.Unmarshal(data)
	if err != nil {
		t.Fatalf("Failed to unmarshal empty message: %v", err)
	}

	if restored.Type() != original.Type() {
		t.Errorf("Type mismatch: got %s, want %s", restored.Type(), original.Type())
	}

	if restored.Payload() != nil {
		t.Errorf("Expected nil payload, got %v", restored.Payload())
	}
}

func TestBinaryMessage_LargePayload(t *testing.T) {
	// Тестируем большое сообщение
	largePayload := make([]byte, 1024*1024) // 1MB
	for i := range largePayload {
		largePayload[i] = byte(i % 256)
	}

	original := NewMessage("large_test", largePayload).(*BinaryMessage)

	data, err := original.Marshal()
	if err != nil {
		t.Fatalf("Failed to marshal large message: %v", err)
	}

	restored := &BinaryMessage{}
	err = restored.Unmarshal(data)
	if err != nil {
		t.Fatalf("Failed to unmarshal large message: %v", err)
	}

	restoredPayload := restored.Payload()
	if len(restoredPayload) != len(largePayload) {
		t.Errorf("Payload length mismatch: got %d, want %d", len(restoredPayload), len(largePayload))
	}

	// Проверяем несколько байтов
	for i := 0; i < 100; i++ {
		if restoredPayload[i] != largePayload[i] {
			t.Errorf("Payload byte %d mismatch: got %d, want %d", i, restoredPayload[i], largePayload[i])
		}
	}
}

func TestBinaryMessage_ManyHeaders(t *testing.T) {
	// Тестируем сообщение с множеством заголовков
	original := NewMessage("headers_test", []byte("test")).(*BinaryMessage)

	for i := 0; i < 100; i++ {
		key := "key" + string(rune('0'+i%10))
		value := "value" + string(rune('0'+i%10))
		original.SetHeader(key, value)
	}

	data, err := original.Marshal()
	if err != nil {
		t.Fatalf("Failed to marshal message with many headers: %v", err)
	}

	restored := &BinaryMessage{}
	err = restored.Unmarshal(data)
	if err != nil {
		t.Fatalf("Failed to unmarshal message with many headers: %v", err)
	}

	originalHeaders := original.Headers()
	restoredHeaders := restored.Headers()

	if len(restoredHeaders) != len(originalHeaders) {
		t.Errorf("Headers count mismatch: got %d, want %d", len(restoredHeaders), len(originalHeaders))
	}

	for key, value := range originalHeaders {
		if restoredHeaders[key] != value {
			t.Errorf("Header %s mismatch: got %s, want %s", key, restoredHeaders[key], value)
		}
	}
}

func TestBinaryMessage_Clone(t *testing.T) {
	original := NewMessage("clone_test", []byte("test payload")).(*BinaryMessage)
	original.SetHeader("test", "value")

	clone := original.Clone().(*BinaryMessage)

	// Проверяем, что клон идентичен оригиналу
	if clone.ID() != original.ID() {
		t.Errorf("ID mismatch: got %s, want %s", clone.ID(), original.ID())
	}

	if clone.Type() != original.Type() {
		t.Errorf("Type mismatch: got %s, want %s", clone.Type(), original.Type())
	}

	if string(clone.Payload()) != string(original.Payload()) {
		t.Errorf("Payload mismatch: got %s, want %s", string(clone.Payload()), string(original.Payload()))
	}

	if clone.GetHeader("test") != original.GetHeader("test") {
		t.Errorf("Header mismatch: got %s, want %s", clone.GetHeader("test"), original.GetHeader("test"))
	}

	// Прове��яем, что изменения в клоне не влияют на оригинал
	clone.SetHeader("test", "new_value")
	if original.GetHeader("test") == "new_value" {
		t.Error("Clone modification affected original")
	}
}

func TestBinaryMessage_Size(t *testing.T) {
	message := NewMessage("size_test", []byte("payload")).(*BinaryMessage)
	message.SetHeader("key", "value")

	size := message.Size()
	if size <= 0 {
		t.Errorf("Invalid size: %d", size)
	}

	// Размер должен быть больше длины payload
	if size <= len("payload") {
		t.Errorf("Size %d should be greater than payload length %d", size, len("payload"))
	}
}

func TestBinaryMessage_ThreadSafety(t *testing.T) {
	message := NewMessage("thread_test", []byte("payload")).(*BinaryMessage)

	// Запускаем несколько горутин для одновременного доступа
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- true }()

			// Читаем и пишем заголовки
			for j := 0; j < 100; j++ {
				key := "key" + string(rune('0'+id))
				value := "value" + string(rune('0'+j%10))
				message.SetHeader(key, value)
				_ = message.GetHeader(key)
				_ = message.Type()
				_ = message.Payload()
			}
		}(i)
	}

	// Ждем завершения всех горутин
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestMessagePool(t *testing.T) {
	pool := NewMessagePool()

	// Получаем сообщение из пула
	msg1 := pool.Get()
	if msg1 == nil {
		t.Fatal("Pool returned nil message")
	}

	// Проверяем, что сообщение очищено
	if msg1.Type() != "" {
		t.Errorf("Message type not reset: %s", msg1.Type())
	}

	if msg1.Payload() != nil {
		t.Errorf("Message payload not reset: %v", msg1.Payload())
	}

	// Используем сообщение
	msg1.msgType = "test"
	msg1.payload = []byte("test")
	msg1.SetHeader("key", "value")

	// Возвращаем в пул
	pool.Put(msg1)

	// Получаем снова
	msg2 := pool.Get()

	// Должно быть то же сообщение, но очищенное
	if msg2 != msg1 {
		t.Error("Pool should reuse messages")
	}

	if msg2.Type() != "" {
		t.Errorf("Reused message type not reset: %s", msg2.Type())
	}

	if msg2.GetHeader("key") != "" {
		t.Errorf("Reused message headers not reset: %s", msg2.GetHeader("key"))
	}
}

func TestGetPooledMessage(t *testing.T) {
	msg := GetPooledMessage("test_type", []byte("test_payload"))

	if msg.Type() != "test_type" {
		t.Errorf("Type mismatch: got %s, want test_type", msg.Type())
	}

	if string(msg.Payload()) != "test_payload" {
		t.Errorf("Payload mismatch: got %s, want test_payload", string(msg.Payload()))
	}

	if msg.ID() == "" {
		t.Error("Message ID should not be empty")
	}

	if msg.Timestamp().IsZero() {
		t.Error("Message timestamp should not be zero")
	}

	// Возвращаем в пул
	msg.Release()
}

// Бенчмарки

func BenchmarkBinaryMessage_Marshal(b *testing.B) {
	message := NewMessage("benchmark", []byte("benchmark payload")).(*BinaryMessage)
	message.SetHeader("key1", "value1")
	message.SetHeader("key2", "value2")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := message.Marshal()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBinaryMessage_Unmarshal(b *testing.B) {
	message := NewMessage("benchmark", []byte("benchmark payload")).(*BinaryMessage)
	message.SetHeader("key1", "value1")
	message.SetHeader("key2", "value2")

	data, err := message.Marshal()
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		msg := &BinaryMessage{}
		err := msg.Unmarshal(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBinaryMessage_Clone(b *testing.B) {
	message := NewMessage("benchmark", []byte("benchmark payload")).(*BinaryMessage)
	message.SetHeader("key1", "value1")
	message.SetHeader("key2", "value2")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = message.Clone()
	}
}

func BenchmarkMessagePool_GetPut(b *testing.B) {
	pool := NewMessagePool()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		msg := pool.Get()
		pool.Put(msg)
	}
}

func BenchmarkBinaryMessage_SetHeader(b *testing.B) {
	message := NewMessage("benchmark", []byte("payload")).(*BinaryMessage)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		message.SetHeader("key", "value")
	}
}

func BenchmarkBinaryMessage_GetHeader(b *testing.B) {
	message := NewMessage("benchmark", []byte("payload")).(*BinaryMessage)
	message.SetHeader("key", "value")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = message.GetHeader("key")
	}
}
