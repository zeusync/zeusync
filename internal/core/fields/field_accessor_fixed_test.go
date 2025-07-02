package fields

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestFA_SubscriptionCleanupFixed tests that unsubscribe properly cleans up
func TestFA_SubscriptionCleanupFixed(t *testing.T) {
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
	fa.subsMx.RLock()
	initialCount := len(fa.subscribers)
	fa.subsMx.RUnlock()

	if initialCount != 10 {
		t.Errorf("Expected 10 subscribers, got %d", initialCount)
	}

	// Unsubscribe all
	for _, unsub := range unsubscribers {
		unsub()
	}

	// Check that subscribers were removed
	fa.subsMx.RLock()
	finalCount := len(fa.subscribers)
	fa.subsMx.RUnlock()

	if finalCount != 0 {
		t.Errorf("Expected 0 subscribers after cleanup, got %d", finalCount)
	}
}

// TestFA_NoDeadlockInNotifications tests that notifications don't cause deadlocks
func TestFA_NoDeadlockInNotifications(t *testing.T) {
	fa := NewFA[int](0)

	// Add a subscriber that tries to modify the field accessor
	unsub := fa.Subscribe(func(newValue int) {
		if newValue < 5 {
			// This should not cause a deadlock
			fa.Set(newValue + 1)
		}
	})
	defer unsub()

	// This should complete without deadlock
	done := make(chan bool, 1)
	go func() {
		fa.Set(1)
		done <- true
	}()

	select {
	case <-done:
		// Success - no deadlock
	case <-time.After(5 * time.Second):
		t.Fatal("Deadlock detected - operation did not complete")
	}

	// Give time for all notifications to complete
	time.Sleep(100 * time.Millisecond)

	// Verify final value
	if got := fa.Get(); got < 5 {
		t.Logf("Final value: %d (notifications may still be processing)", got)
	}
}

// TestFA_ConcurrentSubscriptionManagement tests concurrent subscription add/remove
func TestFA_ConcurrentSubscriptionManagement(t *testing.T) {
	fa := NewFA[int](0)
	const numGoroutines = 50
	const numOperations = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 2) // For both subscription and value setting goroutines

	// Goroutines that add/remove subscriptions
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				unsub := fa.Subscribe(func(newValue int) {
					// Minimal work
					_ = newValue * 2
				})
				time.Sleep(time.Microsecond)
				unsub()
			}
		}()
	}

	// Goroutines that set values
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				fa.Set(id*numOperations + j)
				time.Sleep(time.Microsecond)
			}
		}(i)
	}

	// Wait for all operations to complete
	done := make(chan bool, 1)
	go func() {
		wg.Wait()
		done <- true
	}()

	select {
	case <-done:
		// Success - no deadlock
		t.Log("Concurrent subscription management completed successfully")
	case <-time.After(30 * time.Second):
		t.Fatal("Deadlock detected in concurrent subscription management")
	}
}

// TestFA_LockedOperationsWithNotifications tests that locked operations work with notifications
func TestFA_LockedOperationsWithNotifications(t *testing.T) {
	fa := NewFA[int](0)

	var notificationCount int32
	unsub := fa.Subscribe(func(newValue int) {
		atomic.AddInt32(&notificationCount, 1)
	})
	defer unsub()

	// Perform locked operations
	fa.Lock()
	fa.Set(10)
	fa.Set(20)
	fa.Set(30)
	fa.Unlock()

	// Give time for notifications
	time.Sleep(100 * time.Millisecond)

	// Should have received 3 notifications
	count := atomic.LoadInt32(&notificationCount)
	if count != 3 {
		t.Errorf("Expected 3 notifications, got %d", count)
	}

	if got := fa.Get(); got != 30 {
		t.Errorf("Expected final value 30, got %d", got)
	}
}

// TestFA_TransactionWithSubscribers tests transactions with active subscribers
func TestFA_TransactionWithSubscribers(t *testing.T) {
	fa := NewFA[int64](0)

	var lastNotifiedValue int64
	unsub := fa.Subscribe(func(newValue int64) {
		atomic.StoreInt64(&lastNotifiedValue, newValue)
	})
	defer unsub()

	// Perform concurrent transactions
	const numGoroutines = 10
	const numTransactions = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numTransactions; j++ {
				fa.Transaction(func(current int64) int64 {
					return current + 1
				})
			}
		}()
	}

	wg.Wait()

	expected := int64(numGoroutines * numTransactions)
	if got := fa.Get(); got != expected {
		t.Errorf("Expected final value %d, got %d", expected, got)
	}

	// Give time for final notification
	time.Sleep(50 * time.Millisecond)

	notified := atomic.LoadInt64(&lastNotifiedValue)
	if notified != expected {
		t.Errorf("Expected last notified value %d, got %d", expected, notified)
	}
}
