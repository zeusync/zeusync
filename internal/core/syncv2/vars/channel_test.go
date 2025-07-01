package vars

import (
	"sync"
	"testing"
	"time"
)

func TestBufferedChannelVar(t *testing.T) {
	// Test basic functionality
	ch := NewBufferedChannel[string](5, "initial")

	// Test Get
	if ch.Get() != "initial" {
		t.Errorf("Expected 'initial', got %v", ch.Get())
	}

	// Test Set
	ch.Set("updated")
	if ch.Get() != "updated" {
		t.Errorf("Expected 'updated', got %v", ch.Get())
	}

	// Test Send (non-blocking)
	if !ch.Send("sent1") {
		t.Error("Send should succeed on buffered channel")
	}

	// Test Receive (non-blocking)
	value, ok := ch.Receive()
	if !ok || value != "sent1" {
		t.Errorf("Expected 'sent1', got %v, ok=%v", value, ok)
	}

	// Test version tracking
	initialVersion := ch.Version()
	ch.Set("version_test")
	if ch.Version() <= initialVersion {
		t.Error("Version should increase after Set")
	}

	// Test dirty flag
	ch.MarkClean()
	if ch.IsDirty() {
		t.Error("Should not be dirty after MarkClean")
	}

	ch.Set("dirty_test")
	if !ch.IsDirty() {
		t.Error("Should be dirty after Set")
	}
}

func TestUnbufferedChannelVar(t *testing.T) {
	ch := NewUnbufferedChannel[int](42)

	// Test basic functionality
	if ch.Get() != 42 {
		t.Errorf("Expected 42, got %v", ch.Get())
	}

	// Test that Send fails on unbuffered channel without receiver
	if ch.Send(100) {
		t.Error("Send should fail on unbuffered channel without receiver")
	}

	// Test BufferSize
	if ch.BufferSize() != 0 {
		t.Errorf("Expected buffer size 0, got %d", ch.BufferSize())
	}

	// Test SetBufferSize should fail
	if err := ch.SetBufferSize(10); err == nil {
		t.Error("SetBufferSize should fail on unbuffered channel")
	}
}

func TestPriorityChannelVar(t *testing.T) {
	ch := NewPriorityChannel[string](3, "initial")

	// Test priority ordering
	ch.SendWithPriority("low", 1)
	ch.SendWithPriority("high", 10)
	ch.SendWithPriority("medium", 5)

	// Should receive in priority order: high, medium, low
	value1, ok1 := ch.Receive()
	if !ok1 || value1 != "high" {
		t.Errorf("Expected 'high', got %v, ok=%v", value1, ok1)
	}

	value2, ok2 := ch.Receive()
	if !ok2 || value2 != "medium" {
		t.Errorf("Expected 'medium', got %v, ok=%v", value2, ok2)
	}

	value3, ok3 := ch.Receive()
	if !ok3 || value3 != "low" {
		t.Errorf("Expected 'low', got %v, ok=%v", value3, ok3)
	}

	// Test buffer size
	if ch.BufferSize() != 3 {
		t.Errorf("Expected buffer size 3, got %d", ch.BufferSize())
	}
}

func TestBroadcastChannelVar(t *testing.T) {
	ch := NewBroadcastChannel[int](2, 0)

	// Subscribe multiple receivers
	sub1 := ch.Subscribe()
	sub2 := ch.Subscribe()
	sub3 := ch.Subscribe()

	// Send a value
	if !ch.Send(42) {
		t.Error("Send should succeed")
	}

	// All subscribers should receive the value
	select {
	case value := <-sub1:
		if value != 42 {
			t.Errorf("Sub1: expected 42, got %v", value)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Sub1: timeout waiting for value")
	}

	select {
	case value := <-sub2:
		if value != 42 {
			t.Errorf("Sub2: expected 42, got %v", value)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Sub2: timeout waiting for value")
	}

	select {
	case value := <-sub3:
		if value != 42 {
			t.Errorf("Sub3: expected 42, got %v", value)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Sub3: timeout waiting for value")
	}

	// Test unsubscribe
	ch.Unsubscribe(sub1)

	// Send another value
	ch.Send(100)

	// Only sub2 and sub3 should receive
	select {
	case <-sub1:
		t.Error("Sub1 should not receive after unsubscribe")
	case <-time.After(50 * time.Millisecond):
		// Expected - sub1 should not receive
	}
}

func TestChannelVarConcurrency(t *testing.T) {
	ch := NewBufferedChannel[int](100, 0)

	const numGoroutines = 10
	const numOperations = 100

	var wg sync.WaitGroup

	// Start writers
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				ch.Send(id*1000 + j)
			}
		}(i)
	}

	// Start readers
	received := make(map[int]bool)
	var mu sync.Mutex

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				if value, ok := ch.Receive(); ok {
					mu.Lock()
					received[value] = true
					mu.Unlock()
				}
			}
		}()
	}

	wg.Wait()

	// Check that we received some values (exact count may vary due to timing)
	mu.Lock()
	receivedCount := len(received)
	mu.Unlock()

	if receivedCount == 0 {
		t.Error("Should have received some values")
	}

	t.Logf("Received %d unique values out of %d sent", receivedCount, numGoroutines*numOperations)
}

func TestPriorityChannelVarBlocking(t *testing.T) {
	ch := NewPriorityChannel[string](2, "initial")

	// Test blocking send and receive
	var wg sync.WaitGroup

	// Start a receiver
	wg.Add(1)
	go func() {
		defer wg.Done()
		value := ch.ReceiveBlocking()
		if value != "test" {
			t.Errorf("Expected 'test', got %v", value)
		}
	}()

	// Give receiver time to start
	time.Sleep(10 * time.Millisecond)

	// Send value (should unblock receiver)
	ch.SendBlocking("test")

	wg.Wait()
}

func TestChannelVarClose(t *testing.T) {
	ch := NewBufferedChannel[string](5, "initial")

	// Test that channel works before close
	if !ch.Send("test") {
		t.Error("Send should work before close")
	}

	// Close the channel
	if err := ch.Close(); err != nil {
		t.Errorf("Close should not return error: %v", err)
	}

	// Test that Send fails after close
	if ch.Send("after_close") {
		t.Error("Send should fail after close")
	}
}
