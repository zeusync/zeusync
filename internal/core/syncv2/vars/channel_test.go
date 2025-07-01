package vars

import (
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
	"time"
)

func TestBufferedChannel(t *testing.T) {
	t.Run("BufferedChannel: Basic functionality", func(t *testing.T) {
		// Test basic functionality
		ch := NewBufferedChannel[string](5, "initial")

		require.Equal(t, "initial", ch.Get())

		ch.Set("updated")
		require.Equal(t, "updated", ch.Get())

		require.True(t, ch.Send("sent1"))

		// Test Receive (non-blocking) first queue value
		value, ok := ch.Receive()
		require.True(t, ok)
		require.Equal(t, "updated", value)

		// Test Receive  (non-blocking) second queue
		value, ok = ch.Receive()
		require.True(t, ok)
		require.Equal(t, "sent1", value)

		// Test version tracking
		initialVersion := ch.Version()
		ch.Set("version_test")
		require.Less(t, initialVersion, ch.Version())

		require.True(t, ch.IsDirty())
		ch.MarkClean()
		require.False(t, ch.IsDirty())

		ch.Set("dirty_test")
		require.True(t, ch.IsDirty())
	})
}

func TestUnbufferedChannel(t *testing.T) {
	t.Run("UnbufferedChannel: Basic functionality", func(t *testing.T) {
		// Test basic functionality
		ch := NewUnbufferedChannel[string]("initial")

		require.Equal(t, "initial", ch.Get())

		ch.Set("updated")
		require.Equal(t, "updated", ch.Get())

		go func() {
			require.True(t, ch.Send("sent1"))
		}()

		value := ch.ReceiveBlocking()
		require.Equal(t, "sent1", value)

		// Test version tracking
		initialVersion := ch.Version()
		ch.Set("version_test")
		require.Less(t, initialVersion, ch.Version())

		require.True(t, ch.IsDirty())
		ch.MarkClean()
		require.False(t, ch.IsDirty())

		ch.Set("dirty_test")
		require.True(t, ch.IsDirty())

		// Test BufferSize
		require.Equal(t, 0, ch.BufferSize())
		// Test SetBufferSize should fail
		require.Error(t, ch.SetBufferSize(10))
	})
}

func TestPriorityChannel(t *testing.T) {
	t.Run("PriorityChannel: Basic functionality", func(t *testing.T) {
		const maxSize = 3
		ch := NewPriorityChannel[string](maxSize, "initial")

		values := []struct {
			value    string
			priority int
		}{
			{value: "low", priority: 10},
			{value: "high", priority: 100},
			{value: "medium", priority: 50},
		}

		// Test priority ordering
		for _, v := range values {
			ch.SendBlockingWithPriority(v.value, v.priority)
		}

		// Should receive in priority order: high, medium, low
		value, ok := ch.Receive()
		require.True(t, ok)
		require.Equal(t, "high", value)

		value, ok = ch.Receive()
		require.True(t, ok)
		require.Equal(t, "medium", value)

		value, ok = ch.Receive()
		require.True(t, ok)
		require.Equal(t, "low", value)

		value, ok = ch.Receive()
		require.False(t, ok)
		require.Equal(t, "", value)

		// Test buffer size
		require.Equal(t, maxSize, ch.BufferSize())
	})
}

func TestBroadcastChannel(t *testing.T) {
	t.Run("BroadcastChannel: Basic functionality", func(t *testing.T) {
		ch := NewBroadcastChannel[int](3, 0)

		// Subscribe multiple receivers
		sub1 := ch.Subscribe()
		sub2 := ch.Subscribe()
		sub3 := ch.Subscribe()

		subs := []*Subscription[int]{sub1, sub3, sub2}

		require.True(t, ch.Send(42))

		// All subscribers should receive the value
		wg := sync.WaitGroup{}
		for _, sub := range subs {
			wg.Add(1)
			go func(sub <-chan int) {
				defer wg.Done()
				select {
				case value := <-sub:
					require.Equal(t, 42, value)
				case <-time.After(100 * time.Millisecond):
					t.Error("Timeout waiting for value")
				}
			}(sub.Channel())
		}
		wg.Wait()

		// Test unsubscribe
		ch.Unsubscribe(sub1)

		// Send another value
		require.True(t, ch.Send(100))

		// Only sub2 and sub3 should receive
		select {
		case value, ok := <-sub1.Channel():
			if ok {
				t.Errorf("Sub1 should not receive after unsubscribe, received: %d", value)
			}
		case <-time.After(50 * time.Millisecond):
			// Expected - sub1 should not receive
		}
	})
}

func TestChannelConcurrency(t *testing.T) {
	t.Run("BufferedChanel: Concurrent send and receive ", func(t *testing.T) {
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

		require.Greater(t, receivedCount, 0)
		t.Logf("Received %d unique values out of %d sent", receivedCount, numGoroutines*numOperations)
	})
}

func TestPriorityChannelBlocking(t *testing.T) {
	t.Run("PriorityChannel: Blocking send and receive", func(t *testing.T) {
		ch := NewPriorityChannel[string](2, "initial")

		// Test blocking send and receive
		var wg sync.WaitGroup

		// Start a receiver
		wg.Add(1)
		go func() {
			defer wg.Done()
			value := ch.ReceiveBlocking()
			require.Equal(t, "test", value)
		}()

		// Give receiver time to start
		time.Sleep(10 * time.Millisecond)

		// Send value (should unblock receiver)
		ch.SendBlocking("test")

		wg.Wait()
	})
}

func TestChannelClose(t *testing.T) {
	t.Run("BufferedChannel: Close", func(t *testing.T) {
		ch := NewBufferedChannel[string](5, "initial")

		// Test that ch works before close
		require.True(t, ch.Send("test"))

		// Close the ch
		require.NoError(t, ch.Close(), "Close should not fail")

		// Test that Send fails after close
		require.False(t, ch.Send("after_close"))
	})
}
