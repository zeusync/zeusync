package fields

import (
	"errors"
	"github.com/stretchr/testify/require"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

// TestBatch tests batch operations
func TestBatch(t *testing.T) {
	fa := NewFA[int](0)
	batch := NewBatch(fa)

	// Add operations to batch
	batch.Set(10).Transaction(func(current int) int {
		return current * 2
	}).Set(30)

	// Execute batch
	batch.Execute()

	if got := fa.Get(); got != 30 {
		t.Errorf("Expected final value 30, got %d", got)
	}
}

// TestWaitGroup tests WaitGroup functionality
func TestWaitGroup(t *testing.T) {
	wg := NewWaitGroup()
	fa := NewFA[int](0)

	const numGoroutines = 10
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			fa.Transaction(func(current int) int {
				return current + 1
			})
		}()
	}

	// Test wait with timeout
	if !wg.WaitWithTimeout(time.Second) {
		t.Error("WaitWithTimeout should succeed")
	}

	if got := fa.Get(); got != numGoroutines {
		t.Errorf("Expected final value %d, got %d", numGoroutines, got)
	}
}

// TestAggregator tests aggregator functionality
func TestAggregator(t *testing.T) {
	target := NewFA[int](0)
	aggregator := NewAggregator(target, func(values []int) int {
		sum := 0
		for _, v := range values {
			sum += v
		}
		return sum
	})
	defer aggregator.Close()

	// Add sources
	source1 := NewFA[int](10)
	source2 := NewFA[int](20)
	source3 := NewFA[int](30)

	aggregator.AddSource(source1)
	aggregator.AddSource(source2)
	aggregator.AddSource(source3)

	// Give time for aggregation
	time.Sleep(10 * time.Millisecond)

	if got := target.Get(); got != 60 {
		t.Errorf("Expected aggregated value 60, got %d", got)
	}

	// Update one source
	source1.Set(15)
	time.Sleep(10 * time.Millisecond)

	if got := target.Get(); got != 65 {
		t.Errorf("Expected updated aggregated value 65, got %d", got)
	}
}

// TestDebouncer tests debouncer functionality
func TestDebouncer(t *testing.T) {
	source := NewFA[int](0)
	target := NewFA[int](0)
	debouncer := NewDebouncer(source, target, 50*time.Millisecond)
	defer debouncer.Close()

	// Rapid updates
	source.Set(1)
	source.Set(2)
	source.Set(3)

	// Should not update immediately
	if got := target.Get(); got != 0 {
		t.Errorf("Expected target to not update immediately, got %d", got)
	}

	// Wait for debounce
	time.Sleep(100 * time.Millisecond)

	if got := target.Get(); got != 3 {
		t.Errorf("Expected debounced value 3, got %d", got)
	}
}

// TestThrottler tests throttler functionality
func TestThrottler(t *testing.T) {
	source := NewFA[int](0)
	target := NewFA[int](0)
	throttler := NewThrottler(source, target, 50*time.Millisecond)
	defer throttler.Close()

	// First update should go through
	source.Set(1)
	time.Sleep(10 * time.Millisecond)

	if got := target.Get(); got != 1 {
		t.Errorf("Expected first update to go through, got %d", got)
	}

	// Rapid updates should be throttled
	source.Set(2)
	source.Set(3)
	time.Sleep(10 * time.Millisecond)

	if got := target.Get(); got != 1 {
		t.Errorf("Expected updates to be throttled, got %d", got)
	}

	// Wait for throttle period
	time.Sleep(60 * time.Millisecond)
	source.Set(4)
	time.Sleep(10 * time.Millisecond)

	if got := target.Get(); got != 4 {
		t.Errorf("Expected update after throttle period, got %d", got)
	}
}

// TestValidator tests validator functionality
func TestValidator(t *testing.T) {
	fa := NewFA[int](0)
	var errorCount int32

	validator := NewValidator(fa, func(value int) error {
		if value < 0 {
			return errors.New("negative value not allowed")
		}
		return nil
	}, func(err error) {
		atomic.AddInt32(&errorCount, 1)
	})
	defer validator.Close()

	// Valid value
	fa.Set(10)
	time.Sleep(10 * time.Millisecond)

	if count := atomic.LoadInt32(&errorCount); count != 0 {
		t.Errorf("Expected no errors for valid value, got %d", count)
	}

	// Invalid value
	fa.Set(-5)
	time.Sleep(10 * time.Millisecond)

	if count := atomic.LoadInt32(&errorCount); count != 1 {
		t.Errorf("Expected 1 error for invalid value, got %d", count)
	}
}

// TestTransformer tests transformer functionality
func TestTransformer(t *testing.T) {
	source := NewFA[int](10)
	target := NewFA[string]("")

	transformer := NewTransformer(source, target, func(value int) string {
		return strconv.Itoa(value * 2)
	})
	defer transformer.Close()

	// Check initial transformation
	time.Sleep(10 * time.Millisecond)
	if got := target.Get(); got != "20" {
		t.Errorf("Expected initial transformed value '20', got '%s'", got)
	}

	// Update source
	source.Set(15)
	time.Sleep(10 * time.Millisecond)

	if got := target.Get(); got != "30" {
		t.Errorf("Expected transformed value '30', got '%s'", got)
	}
}

// TestSynchronizer tests synchronizer functionality
func TestSynchronizer(t *testing.T) {
	fa1 := NewFA[int](0)
	fa2 := NewFA[int](0)
	fa3 := NewFA[int](0)

	synchronizer := NewSynchronizer(fa1, fa2, fa3)
	defer synchronizer.Close()

	// Update one accessor
	fa1.Set(42)
	time.Sleep(10 * time.Millisecond)

	// All should have the same value
	if got := fa2.Get(); got != 42 {
		t.Errorf("Expected synchronized value 42 in fa2, got %d", got)
	}

	if got := fa3.Get(); got != 42 {
		t.Errorf("Expected synchronized value 42 in fa3, got %d", got)
	}

	// Update another accessor
	fa2.Set(100)
	time.Sleep(10 * time.Millisecond)

	if got := fa1.Get(); got != 100 {
		t.Errorf("Expected synchronized value 100 in fa1, got %d", got)
	}

	if got := fa3.Get(); got != 100 {
		t.Errorf("Expected synchronized value 100 in fa3, got %d", got)
	}
}

// TestTimeoutWrapper tests timeout wrapper functionality
func TestTimeoutWrapper(t *testing.T) {
	fa := NewFA[int](42)
	wrapper := WithTimeout(fa, 100*time.Millisecond)

	// Test Get with timeout
	value, err := wrapper.Get()
	if err != nil {
		t.Errorf("Get should not timeout: %v", err)
	}
	if value != 42 {
		t.Errorf("Expected value 42, got %d", value)
	}

	// Test Set with timeout
	err = wrapper.Set(100)
	if err != nil {
		t.Errorf("Set should not timeout: %v", err)
	}
	if got := fa.Get(); got != 100 {
		t.Errorf("Expected value 100 after set, got %d", got)
	}

	// Test Transaction with timeout
	result, err := wrapper.Transaction(func(current int) int {
		return current * 2
	})
	if err != nil {
		t.Errorf("Transaction should not timeout: %v", err)
	}
	if result != 200 {
		t.Errorf("Expected transaction result 200, got %d", result)
	}
}

// TestTimeoutWrapperTimeout tests timeout behavior
func TestTimeoutWrapperTimeout(t *testing.T) {
	fa := NewFA[int](42)
	wrapper := WithTimeout(fa, time.Nanosecond) // Short timeout

	// Sleep to ensure timeout
	time.Sleep(5 * time.Millisecond)

	// Test Get timeout
	_, err := wrapper.Get()
	require.Error(t, err, "Get should timeout")

	// Test Set timeout
	err = wrapper.Set(100)
	require.Error(t, err, "Set should timeout")

	// Test Transaction timeout
	_, err = wrapper.Transaction(func(current int) int {
		time.Sleep(10 * time.Millisecond) // Simulate slow operation
		return current * 2
	})
	require.Error(t, err, "Transaction should timeout")
}

// TestUtilsConcurrency tests utilities under concurrent access
func TestUtilsConcurrency(t *testing.T) {
	source := NewFA[int64](0)
	target := NewFA[int64](0)

	// Create multiple utilities
	debouncer := NewDebouncer(source, target, 10*time.Millisecond)
	defer debouncer.Close()

	validator := NewValidator(source, func(value int64) error {
		if value < 0 {
			return errors.New("negative value")
		}
		return nil
	}, func(err error) {
		// Handle error
	})
	defer validator.Close()

	// Concurrent updates
	const numGoroutines = 10
	const numUpdates = 100

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < numUpdates; j++ {
				source.Set(int64(id*numUpdates + j))
				time.Sleep(time.Microsecond)
			}
		}(i)
	}

	// Wait for operations to complete
	time.Sleep(200 * time.Millisecond)

	// Verify no panics occurred (test passes if we reach here)
	t.Log("Concurrent utilities test completed successfully")
}
