package fields

import (
	"sync/atomic"
)

// generateUniqueID generates a unique ID by atomically incrementing the source counter.
// This function is thread-safe and can be used concurrently.
func generateUniqueID(source *atomic.Uint64) uint64 {
	return source.Add(1)
}
