package field

import (
	"github.com/zeusync/zeusync/internal/core/observability/log"
	"sync"
	"sync/atomic"
)

func generateUniqueID(source uint64) uint64 {
	return atomic.AddUint64(&source, 1)
}

func notifyAll[T any, K comparable, V func(T)](subs map[K]V, newVal T, lock sync.Locker) {
	lock.Lock()
	defer lock.Unlock()

	batchSize := 5
	batch := make([]func(T), 0, batchSize)

	for _, sub := range subs {
		batch = append(batch, sub)

		if len(batch) >= batchSize {
			notifyBatch(batch, newVal)
			batch = batch[:0]
		}
	}

	if len(batch) > 0 {
		notifyBatch(batch, newVal)
	}
}

func notifyBatch[T any](batch []func(T), val T) {
	var wg sync.WaitGroup
	wg.Add(len(batch))

	for _, cb := range batch {
		go func(callback func(T)) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					log.Provide().Fatal("field accessor subscriber panic while notification", log.Any("data", r))
				}
			}()
			callback(val)
		}(cb)
	}

	wg.Wait()
}
