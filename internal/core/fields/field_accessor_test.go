package fields

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestFA_BasicOperations tests basic get/set operations
func TestFA_BasicOperations(t *testing.T) {
	fa := NewFA[int](42)

	// Test initial value
	if got := fa.Get(); got != 42 {
		t.Errorf("Expected initial value 42, got %d", got)
	}

	// Test set operation
	fa.Set(100)
	if got := fa.Get(); got != 100 {
		t.Errorf("Expected value 100 after set, got %d", got)
	}

	// Test version increment
	if version := fa.Version(); version != 1 {
		t.Errorf("Expected version 1 after one set operation, got %d", version)
	}
}

// TestFA_ZeroValue tests creation without initial value
func TestFA_ZeroValue(t *testing.T) {
	fa := NewFA[int]()

	if got := fa.Get(); got != 0 {
		t.Errorf("Expected zero value 0, got %d", got)
	}
}

// TestFA_AtomicOperations tests atomic operations
func TestFA_AtomicOperations(t *testing.T) {
	fa := NewFA[int](10)

	// Test CompareAndSwap
	if !fa.CompareAndSwap(10, 20) {
		t.Error("CompareAndSwap should succeed when values match")
	}

	if fa.CompareAndSwap(10, 30) {
		t.Error("CompareAndSwap should fail when values don't match")
	}

	// Test Swap
	old := fa.Swap(50)
	if old != 20 {
		t.Errorf("Swap should return old value 20, got %d", old)
	}

	if got := fa.Get(); got != 50 {
		t.Errorf("Expected value 50 after swap, got %d", got)
	}
}

// TestFA_Transaction tests transaction functionality
func TestFA_Transaction(t *testing.T) {
	fa := NewFA[int](10)

	result := fa.Transaction(func(current int) int {
		return current * 2
	})

	if result != 20 {
		t.Errorf("Expected transaction result 20, got %d", result)
	}

	if got := fa.Get(); got != 20 {
		t.Errorf("Expected value 20 after transaction, got %d", got)
	}
}

// TestFA_Versioning tests version control functionality
func TestFA_Versioning(t *testing.T) {
	fa := NewFA[int](10)
	initialVersion := fa.Version()

	fa.Set(20)
	if !fa.ChangedSince(initialVersion) {
		t.Error("ChangedSince should return true after value change")
	}

	currentVersion := fa.Version()
	if fa.ChangedSince(currentVersion) {
		t.Error("ChangedSince should return false for current version")
	}
}

// TestFA_Subscriptions tests subscription functionality
func TestFA_Subscriptions(t *testing.T) {
	fa := NewFA[int](10)

	var receivedValue int
	var callCount int32

	unsub := fa.Subscribe(func(newValue int) {
		receivedValue = newValue
		atomic.AddInt32(&callCount, 1)
	})
	defer unsub()

	fa.Set(20)

	// Give some time for async notification
	time.Sleep(10 * time.Millisecond)

	if receivedValue != 20 {
		t.Errorf("Expected received value 20, got %d", receivedValue)
	}

	if atomic.LoadInt32(&callCount) != 1 {
		t.Errorf("Expected callback to be called once, got %d times", callCount)
	}
}

// TestFA_ConditionalSubscriptions tests conditional subscription functionality
func TestFA_ConditionalSubscriptions(t *testing.T) {
	fa := NewFA[int](10)

	var receivedValue int
	var callCount int32

	// Only notify for values > 15
	unsub := fa.SubscribeIf(func(newValue int) {
		receivedValue = newValue
		atomic.AddInt32(&callCount, 1)
	}, func(value int) bool {
		return value > 15
	})
	defer unsub()

	fa.Set(12) // Should not trigger
	time.Sleep(10 * time.Millisecond)

	if atomic.LoadInt32(&callCount) != 0 {
		t.Error("Callback should not be called for value <= 15")
	}

	fa.Set(20) // Should trigger
	time.Sleep(10 * time.Millisecond)

	if receivedValue != 20 {
		t.Errorf("Expected received value 20, got %d", receivedValue)
	}

	if atomic.LoadInt32(&callCount) != 1 {
		t.Errorf("Expected callback to be called once, got %d times", callCount)
	}
}

// TestFA_ChannelSubscriptions tests channel-based subscriptions
func TestFA_ChannelSubscriptions(t *testing.T) {
	fa := NewFA[int](10)

	ch, unsub := fa.SubscribeCh()
	defer unsub()

	// Should receive initial value
	select {
	case value := <-ch:
		if value != 10 {
			t.Errorf("Expected initial value 10, got %d", value)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Should receive initial value")
	}

	fa.Set(20)

	// Should receive updated value
	select {
	case value := <-ch:
		if value != 20 {
			t.Errorf("Expected updated value 20, got %d", value)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Should receive updated value")
	}
}

// TestFA_History tests history functionality
func TestFA_History(t *testing.T) {
	fa := NewFA[int](10)

	fa.Set(20)
	fa.Set(30)
	fa.Set(40)

	history := fa.History(3)
	expected := []int{20, 30, 40}

	if len(history) != len(expected) {
		t.Errorf("Expected history length %d, got %d", len(expected), len(history))
	}

	for i, value := range expected {
		if i < len(history) && history[i] != value {
			t.Errorf("Expected history[%d] = %d, got %d", i, value, history[i])
		}
	}
}

// TestFA_Delta tests delta functionality
func TestFA_Delta(t *testing.T) {
	fa := NewFA[int](10)
	initialVersion := fa.Version()

	fa.Set(20)
	fa.Set(30)

	delta, err := fa.Delta(initialVersion)
	if err != nil {
		t.Errorf("Delta should not return error: %v", err)
	}

	expected := []int{20, 30}
	if len(delta) != len(expected) {
		t.Errorf("Expected delta length %d, got %d", len(expected), len(delta))
	}

	for i, value := range expected {
		if i < len(delta) && delta[i] != value {
			t.Errorf("Expected delta[%d] = %d, got %d", i, value, delta[i])
		}
	}
}

// TestFA_ContextOperations tests context-aware operations
func TestFA_ContextOperations(t *testing.T) {
	fa := NewFA[int](10)
	ctx := context.Background()

	// Test GetWithContext
	value, err := fa.GetWithContext(ctx)
	if err != nil {
		t.Errorf("GetWithContext should not return error: %v", err)
	}
	if value != 10 {
		t.Errorf("Expected value 10, got %d", value)
	}

	// Test SetWithContext
	err = fa.SetWithContext(ctx, 20)
	if err != nil {
		t.Errorf("SetWithContext should not return error: %v", err)
	}
	if got := fa.Get(); got != 20 {
		t.Errorf("Expected value 20 after SetWithContext, got %d", got)
	}

	// Test with cancelled context
	cancelledCtx, cancel := context.WithCancel(ctx)
	cancel()

	_, err = fa.GetWithContext(cancelledCtx)
	if err == nil {
		t.Error("GetWithContext should return error for cancelled context")
	}

	err = fa.SetWithContext(cancelledCtx, 30)
	if err == nil {
		t.Error("SetWithContext should return error for cancelled context")
	}
}

// TestFA_Serialization tests serialization functionality
func TestFA_Serialization(t *testing.T) {
	fa := NewFA[int](42)

	// Test serialization
	data, err := fa.Serialize()
	if err != nil {
		t.Errorf("Serialize should not return error: %v", err)
	}

	// Test deserialization
	newFA := NewFA[int]()
	err = newFA.Deserialize(data)
	if err != nil {
		t.Errorf("Deserialize should not return error: %v", err)
	}

	if got := newFA.Get(); got != 42 {
		t.Errorf("Expected deserialized value 42, got %d", got)
	}
}

// TestFA_Clone tests cloning functionality
func TestFA_Clone(t *testing.T) {
	fa := NewFA[int](42)
	fa.Set(100) // Change value and version

	clone := fa.Clone()

	// Clone should have same value
	if got := clone.Get(); got != 100 {
		t.Errorf("Expected cloned value 100, got %d", got)
	}

	// But should be independent
	fa.Set(200)
	if got := clone.Get(); got != 100 {
		t.Errorf("Clone should be independent, expected 100, got %d", got)
	}
}

// TestFA_Merge tests merge functionality
func TestFA_Merge(t *testing.T) {
	fa1 := NewFA[int](10)
	fa2 := NewFA[int](20)

	fa1.Merge(fa2, func(current, other int) int {
		return current + other
	})

	if got := fa1.Get(); got != 30 {
		t.Errorf("Expected merged value 30, got %d", got)
	}
}

// TestFA_Locking tests locking functionality
func TestFA_Locking(t *testing.T) {
	fa := NewFA[int](10)

	// Test basic locking
	fa.Lock()
	fa.Set(20)
	fa.Unlock()

	if got := fa.Get(); got != 20 {
		t.Errorf("Expected value 20 after locked operation, got %d", got)
	}

	// Test read locking
	fa.RLock()
	value := fa.Get()
	fa.RUnlock()

	if value != 20 {
		t.Errorf("Expected value 20 with read lock, got %d", value)
	}

	// Test TryLock
	if !fa.TryLock(100 * time.Millisecond) {
		t.Error("TryLock should succeed when lock is available")
	}
	fa.Unlock()
}

// TestFA_ConcurrentAccess tests concurrent access patterns
func TestFA_ConcurrentAccess(t *testing.T) {
	fa := NewFA[int64](0)
	const numGoroutines = 100
	const numOperations = 1000

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Concurrent increments
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				fa.Transaction(func(current int64) int64 {
					return current + 1
				})
			}
		}()
	}

	wg.Wait()

	expected := int64(numGoroutines * numOperations)
	if got := fa.Get(); got != expected {
		t.Errorf("Expected final value %d, got %d", expected, got)
	}
}

// TestFA_SubscriptionCleanup tests that unsubscribe properly cleans up
func TestFA_SubscriptionCleanup(t *testing.T) {
	fa := NewFA[int](10)

	// Add multiple subscriptions
	var unsubscribers []func()
	for i := 0; i < 10; i++ {
		unsub := fa.Subscribe(func(newValue int) {
			// Do nothing
		})
		unsubscribers = append(unsubscribers, unsub)
	}

	// Check that subscribers were added
	fa.mx.RLock()
	initialCount := len(fa.subscribers)
	fa.mx.RUnlock()

	if initialCount != 10 {
		t.Errorf("Expected 10 subscribers, got %d", initialCount)
	}

	// Unsubscribe all
	for _, unsub := range unsubscribers {
		unsub()
	}

	// Check that subscribers were removed
	fa.mx.RLock()
	finalCount := len(fa.subscribers)
	fa.mx.RUnlock()

	if finalCount != 0 {
		t.Errorf("Expected 0 subscribers after cleanup, got %d", finalCount)
	}
}
