package vars

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestChannelVars(t *testing.T) {
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

	t.Run("CycleChannel: Basic functionality", func(t *testing.T) {
		const valuesAmount = 10
		ch := NewCycleChannel[int](valuesAmount, 0)

		for i := 0; i < valuesAmount; i++ {
			require.True(t, ch.Send(i))
		}

		for i := 1; i <= valuesAmount; i++ {
			value, ok := ch.Receive()
			require.True(t, ok)
			require.Less(t, value, i)
		}
	})
}

func TestChannelConcurrency(t *testing.T) {
	const (
		numSenders      = 10
		numReceivers    = 100
		valuesPerSender = 10
	)

	t.Run("BufferedChanel: Concurrent send and receive ", func(t *testing.T) {
		ch := NewBufferedChannel[int](numSenders, 0)

		var wg sync.WaitGroup

		// Start senders
		for i := 0; i < numSenders; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < numReceivers; j++ {
					ch.Send(id*1000 + j)
				}
			}(i)
		}

		// Start readers
		received := make(map[int]bool)
		var mu sync.Mutex

		for i := 0; i < numSenders; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < numReceivers; j++ {
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
		t.Logf("Received %d unique values out of %d sent", receivedCount, numSenders*numReceivers)
	})

	t.Run("UnbufferedChannel: Concurrent send and receive", func(t *testing.T) {
		ch := NewUnbufferedChannel[int](0)

		var sWg, rWg sync.WaitGroup
		received := make(map[int]bool)
		var mu sync.Mutex

		// Start receivers first
		for i := 0; i < numReceivers; i++ {
			rWg.Add(1)
			go func() {
				defer rWg.Done()
				for j := 0; j < numSenders*valuesPerSender/numReceivers; j++ {
					value := ch.ReceiveBlocking()
					mu.Lock()
					received[value] = true
					mu.Unlock()
				}
			}()
		}

		// Then start senders
		for i := 0; i < numSenders; i++ {
			sWg.Add(1)
			go func(id int) {
				defer sWg.Done()
				for j := 0; j < valuesPerSender; j++ {
					ch.Send(id*1000 + j)
				}
			}(i)
		}

		sWg.Wait()
		rWg.Wait()

		mu.Lock()
		receivedCount := len(received)
		mu.Unlock()

		require.Greater(t, receivedCount, 0, "No values were received")
		t.Logf("Received %d unique values out of %d sent", receivedCount, numSenders*valuesPerSender)
	})

	t.Run("PriorityChannel: Concurrent send and receive", func(t *testing.T) {
		ch := NewPriorityChannel[int](numSenders*valuesPerSender, 0)

		var sWg, rWg sync.WaitGroup
		received := make(map[int]bool)
		var mu sync.Mutex

		// Start receivers first
		for i := 0; i < numReceivers; i++ {
			rWg.Add(1)
			go func() {
				defer rWg.Done()
				for j := 0; j < numSenders*valuesPerSender/numReceivers; j++ {
					value := ch.ReceiveBlocking()
					mu.Lock()
					received[value] = true
					mu.Unlock()
				}
			}()
		}

		// Then start senders
		for i := 0; i < numSenders; i++ {
			sWg.Add(1)
			go func(id int) {
				defer sWg.Done()
				for j := 0; j < valuesPerSender; j++ {
					v := id*1000 + j
					ch.SendWithPriority(v, v)
				}
			}(i)
		}

		sWg.Wait()
		rWg.Wait()

		mu.Lock()
		receivedCount := len(received)
		mu.Unlock()

		require.Greater(t, receivedCount, 0, "No values were received")
		t.Logf("Received %d unique values out of %d sent", receivedCount, numSenders*valuesPerSender)
	})

	t.Run("CycleChannel: Concurrent send and receive", func(t *testing.T) {
		ch := NewCycleChannel[int](numSenders, 5)

		var sWg, rWg sync.WaitGroup
		received := make(map[int]bool)
		var mu sync.Mutex
		stop := make(chan struct{})

		for i := 0; i < numReceivers; i++ {
			rWg.Add(1)
			go func() {
				defer rWg.Done()
				for {
					select {
					case <-stop:
						return
					default:
						if value, ok := ch.Receive(); ok {
							mu.Lock()
							received[value] = true
							mu.Unlock()
						}
						time.Sleep(5 * time.Millisecond)
					}
				}
			}()
		}

		for i := 0; i < numSenders; i++ {
			sWg.Add(1)
			go func(id int) {
				defer sWg.Done()
				for j := 0; j < valuesPerSender; j++ {
					ch.Send(id*1000 + j)
				}
			}(i)
		}

		sWg.Wait()
		time.Sleep(100 * time.Millisecond)
		close(stop)
		rWg.Wait()

		mu.Lock()
		receivedCount := len(received)
		mu.Unlock()

		require.Greater(t, receivedCount, 0, "No values were received")

		t.Logf("Received %d unique values out of %d sent", receivedCount, numSenders*valuesPerSender)
	})
}

func TestChannelBlocking(t *testing.T) {
	t.Run("BufferedChannel: Blocking send and receive", func(t *testing.T) {
		ch := NewBufferedChannel[string](2, "initial")

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

	t.Run("UnbufferedChannel: Blocking send and receive", func(t *testing.T) {
		ch := NewUnbufferedChannel[string]("initial")

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

	t.Run("CycleChannel: Blocking send and receive", func(t *testing.T) {
		ch := NewCycleChannel[string](5, "initial")

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

	t.Run("UnbufferedChannel: Close", func(t *testing.T) {
		ch := NewUnbufferedChannel[string]("initial")

		// Close the ch
		require.NoError(t, ch.Close(), "Close should not fail")

		// Test that Send fails after close
		require.False(t, ch.Send("after_close"))
	})

	t.Run("PriorityChannel: Close", func(t *testing.T) {
		ch := NewPriorityChannel[string](5, "initial")

		// Test that ch works before close
		require.True(t, ch.Send("test"))

		// Close the ch
		require.NoError(t, ch.Close(), "Close should not fail")

		// Test that Send fails after close
		require.False(t, ch.Send("after_close"))
	})

	t.Run("BroadcastChannel: Close", func(t *testing.T) {
		ch := NewBroadcastChannel[string](5, "initial")

		// Test that ch works before close
		require.True(t, ch.Send("test"))

		// Close the ch
		require.NoError(t, ch.Close(), "Close should not fail")

		// Test that Send fails after close
		require.False(t, ch.Send("after_close"))
	})

	t.Run("CycleChannel: Close", func(t *testing.T) {
		ch := NewCycleChannel[string](5, "initial")

		// Test that ch works before close
		require.True(t, ch.Send("test"))

		// Close the ch
		require.NoError(t, ch.Close(), "Close should not fail")

		// Test that Send fails after close
		require.False(t, ch.Send("after_close"))
	})
}
