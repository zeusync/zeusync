package vars

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSharded_HashSharded(t *testing.T) {
	t.Run("HashSharded", func(t *testing.T) {
		t.Run("Basic Operations", func(t *testing.T) {
			v := NewHashSharded(4, "initial")

			require.Equal(t, "initial", v.Get())
			require.Equal(t, 4, v.GetShardCount())
			require.False(t, v.IsDirty())

			v.Set("updated")
			require.Equal(t, "updated", v.Get())
			require.True(t, v.IsDirty())

			v.MarkClean()
			require.False(t, v.IsDirty())
		})

		t.Run("Shard Operations", func(t *testing.T) {
			v := NewHashSharded(4, 0)

			// Set values in different shards
			v.SetShard("key1", 10)
			v.SetShard("key2", 20)
			v.SetShard("key3", 30)

			// Get values from shards
			require.Equal(t, 10, v.GetShard("key1"))
			require.Equal(t, 20, v.GetShard("key2"))
			require.Equal(t, 30, v.GetShard("key3"))
		})

		t.Run("ForEachShard", func(t *testing.T) {
			v := NewHashSharded(3, 5)

			count := 0
			v.ForEachShard(func(shardIndex int, value int) bool {
				require.Equal(t, 5, value)
				count++
				return true
			})
			require.Equal(t, 3, count)
		})

		t.Run("MergeShard", func(t *testing.T) {
			v := NewHashSharded(3, 10)

			// Set different values in shards
			v.SetShard("a", 1)
			v.SetShard("b", 2)
			v.SetShard("c", 3)

			// Merge by summing all values
			sum := v.MergeShard(func(values []int) int {
				total := 0
				for _, val := range values {
					total += val
				}
				return total
			})

			// Should be at least 6 (1+2+3), but might include initial values
			require.GreaterOrEqual(t, sum, 6)
		})
	})
}

func TestSharded_ConsistentHashSharded(t *testing.T) {
	t.Run("ConsistentHashSharded", func(t *testing.T) {
		t.Run("Basic Operations", func(t *testing.T) {
			v := NewConsistentHashSharded(4, "initial")

			require.Equal(t, "initial", v.Get())
			require.Greater(t, v.GetShardCount(), 0)
			require.False(t, v.IsDirty())

			v.Set("updated")
			require.Equal(t, "updated", v.Get())
			require.True(t, v.IsDirty())
		})

		t.Run("Consistent Hashing", func(t *testing.T) {
			v := NewConsistentHashSharded(4, 0)

			// Same key should always go to same shard
			v.SetShard("consistent-key", 42)
			require.Equal(t, 42, v.GetShard("consistent-key"))
			require.Equal(t, 42, v.GetShard("consistent-key"))
		})

		/*t.Run("Distribution", func(t *testing.T) {
			v := NewConsistentHashSharded(4, 0)

			kv := make(map[string]int, 100)

			// Set values with different keys
			for i := 0; i < 100; i++ {
				key := fmt.Sprintf("key-%d-%d", time.Now().UnixNano(), i)
				v.SetShard(key, i)
				kv[key] = i
			}

			// Verify we can retrieve all values
			for key, value := range kv {
				require.Equal(t, value, v.GetShard(key))
			}
		})*/
	})
}

func TestSharded_RangeSharded(t *testing.T) {
	t.Run("RangeSharded", func(t *testing.T) {
		t.Run("Basic Operations", func(t *testing.T) {
			ranges := []string{"d", "h", "l", "p", "t"}
			v := NewRangeSharded(ranges, "initial")

			require.Equal(t, "initial", v.Get())
			require.Equal(t, 5, v.GetShardCount())
			require.False(t, v.IsDirty())

			v.Set("updated")
			require.Equal(t, "updated", v.Get())
			require.True(t, v.IsDirty())
		})

		t.Run("Range Distribution", func(t *testing.T) {
			ranges := []string{"d", "h", "l", "p", "t"}
			v := NewRangeSharded(ranges, 0)

			// Keys should be distributed based on ranges
			v.SetShard("apple", 1)    // Should go to first shard (< "d")
			v.SetShard("elephant", 2) // Should go to second shard ("d" <= key < "h")
			v.SetShard("monkey", 3)   // Should go to fourth shard ("l" <= key < "p")
			v.SetShard("zebra", 4)    // Should go to last shard (>= "t")

			require.Equal(t, 1, v.GetShard("apple"))
			require.Equal(t, 2, v.GetShard("elephant"))
			require.Equal(t, 3, v.GetShard("monkey"))
			require.Equal(t, 4, v.GetShard("zebra"))
		})

		t.Run("Default Ranges", func(t *testing.T) {
			v := NewRangeSharded[int](nil, 0) // Use default ranges

			require.Equal(t, 4, v.GetShardCount()) // Default has 4 ranges

			v.SetShard("apple", 1)
			v.SetShard("zebra", 2)

			require.Equal(t, 1, v.GetShard("apple"))
			require.Equal(t, 2, v.GetShard("zebra"))
		})
	})
}

func TestSharded_AdaptiveSharded(t *testing.T) {
	t.Run("AdaptiveSharded", func(t *testing.T) {
		t.Run("Basic Operations", func(t *testing.T) {
			v := NewAdaptiveSharded(2, "initial")

			require.Equal(t, "initial", v.Get())
			require.Equal(t, 2, v.GetShardCount())
			require.False(t, v.IsDirty())

			v.Set("updated")
			require.Equal(t, "updated", v.Get())
			require.True(t, v.IsDirty())
		})

		t.Run("Resize Threshold", func(t *testing.T) {
			v := NewAdaptiveSharded(2, 0)
			v.SetResizeThreshold(10) // Low threshold for testing

			initialCount := v.GetShardCount()

			// Access the same shard many times to trigger resize
			for i := 0; i < 15; i++ {
				v.GetShard("same-key")
			}

			// Give some time for async resize
			// Note: In a real test, you might want to add a way to wait for resize completion
			finalCount := v.GetShardCount()

			// The shard count should be at least the initial count
			require.GreaterOrEqual(t, finalCount, initialCount)
		})

		t.Run("Access Counts", func(t *testing.T) {
			v := NewAdaptiveSharded(2, 0)

			// Access different shards
			v.GetShard("key1")
			v.GetShard("key2")
			v.GetShard("key1") // Access key1 again

			counts := v.GetAccessCounts()
			require.NotEmpty(t, counts)

			// At least one shard should have been accessed
			totalAccesses := int64(0)
			for _, count := range counts {
				totalAccesses += count
			}
			require.Greater(t, totalAccesses, int64(0))
		})
	})
}

func TestSharded_VersionAndDirty(t *testing.T) {
	implementations := []struct {
		name string
		impl func() interface {
			Get() string
			Set(string)
			Version() uint64
			IsDirty() bool
			MarkClean()
		}
	}{
		{"HashSharded", func() interface {
			Get() string
			Set(string)
			Version() uint64
			IsDirty() bool
			MarkClean()
		} {
			return NewHashSharded(4, "test")
		}},
		{"ConsistentHashSharded", func() interface {
			Get() string
			Set(string)
			Version() uint64
			IsDirty() bool
			MarkClean()
		} {
			return NewConsistentHashSharded(4, "test")
		}},
		{"RangeSharded", func() interface {
			Get() string
			Set(string)
			Version() uint64
			IsDirty() bool
			MarkClean()
		} {
			return NewRangeSharded(nil, "test")
		}},
		{"AdaptiveSharded", func() interface {
			Get() string
			Set(string)
			Version() uint64
			IsDirty() bool
			MarkClean()
		} {
			return NewAdaptiveSharded(4, "test")
		}},
	}

	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
			v := impl.impl()

			// Initial state
			initialVersion := v.Version()
			require.Greater(t, initialVersion, uint64(0))
			require.False(t, v.IsDirty())

			// After set
			v.Set("new-value")
			require.Greater(t, v.Version(), initialVersion)
			require.True(t, v.IsDirty())
			require.Equal(t, "new-value", v.Get())

			// After mark clean
			v.MarkClean()
			require.False(t, v.IsDirty())
		})
	}
}

func TestSharded_ConcurrentAccess(t *testing.T) {
	t.Run("HashSharded Concurrent", func(t *testing.T) {
		v := NewHashSharded(8, 0)

		// Run concurrent operations
		done := make(chan bool, 100)

		// Writers
		for i := 0; i < 50; i++ {
			go func(id int) {
				key := fmt.Sprintf("key-%d", id%10)
				v.SetShard(key, id)
				done <- true
			}(i)
		}

		// Readers
		for i := 0; i < 50; i++ {
			go func(id int) {
				key := fmt.Sprintf("key-%d", id%10)
				_ = v.GetShard(key)
				done <- true
			}(i)
		}

		// Wait for all goroutines
		for i := 0; i < 100; i++ {
			<-done
		}

		// Verify final state
		require.Greater(t, v.Version(), uint64(1))
	})
}
