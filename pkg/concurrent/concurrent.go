package concurrent

import (
	"github.com/zeusync/zeusync/pkg/sequence"
	"golang.org/x/sync/errgroup"
	"sync"
)

// Concurrent runs the action function for each element of the iterator in a separate goroutine.
// It waits for all goroutines to finish. If action returns an error, it returns the first error encountered.
func Concurrent[T any](i *sequence.Iterator[T], action func(T) error) error {
	errGroup := errgroup.Group{}
	next, stop := i.Pull()
	defer stop()

	for {
		value, valid := next()
		if !valid {
			break
		}

		errGroup.Go(func() error {
			return action(value)
		})
	}

	return errGroup.Wait()
}

// ParallelMute runs the action function for each element of the iterator in a separate goroutine.
// It waits for all goroutines to finish. The action function does not return an error and ignores any errors encountered.
func ParallelMute[T any](i *sequence.Iterator[T], action func(T) error) {
	wg := sync.WaitGroup{}
	next, stop := i.Pull()
	defer stop()

	for {
		value, valid := next()
		if !valid {
			break
		}

		wg.Add(1)
		go func(value T) {
			defer wg.Done()
			_ = action(value)
		}(value)
	}

	wg.Wait()
}

// ParallelMust runs the action function for each element of the iterator in a separate goroutine.
// It waits for all goroutines to finish.
func ParallelMust[T any](i *sequence.Iterator[T], action func(T)) {
	wg := sync.WaitGroup{}
	next, stop := i.Pull()
	defer stop()

	for {
		value, valid := next()
		if !valid {
			break
		}

		wg.Add(1)
		go func(value T) {
			defer wg.Done()
			action(value)
		}(value)
	}

	wg.Wait()
}

// ParallelMap applies the mapFn to each element of the iterator in parallel, preserving order.
// The workers parameter controls the number of goroutines.
func ParallelMap[T any, R any](i *sequence.Iterator[T], workers int, mapFn func(T) R) []R {
	in := i.Collect()
	out := make([]R, len(in))
	var wg sync.WaitGroup
	sem := make(chan struct{}, workers)

	for idx, val := range in {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, v T) {
			defer wg.Done()
			out[i] = mapFn(v)
			<-sem
		}(idx, val)
	}
	wg.Wait()
	return out
}

// ParallelFilter applies the filterFn to each element in parallel, returns filtered slice.
// The concurrency parameter controls the number of goroutines.
func ParallelFilter[T any](i *sequence.Iterator[T], concurrency int, filterFn func(T) bool) []T {
	in := i.Collect()
	out := make([]T, 0, len(in))
	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)
	res := make([]bool, len(in))

	for idx, val := range in {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, v T) {
			defer wg.Done()
			res[i] = filterFn(v)
			<-sem
		}(idx, val)
	}
	wg.Wait()
	for idx, keep := range res {
		if keep {
			out = append(out, in[idx])
		}
	}
	return out
}

// Merge merges multiple channels of T into a single output channel.
func Merge[T any](chs ...<-chan T) <-chan T {
	out := make(chan T)
	var wg sync.WaitGroup
	wg.Add(len(chs))
	for _, ch := range chs {
		go func(c <-chan T) {
			defer wg.Done()
			for v := range c {
				out <- v
			}
		}(ch)
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

// FanOut sends each element of the iterator to multiple handler functions concurrently.
func FanOut[T any](i *sequence.Iterator[T], handlers ...func(T)) {
	var wg sync.WaitGroup
	next, stop := i.Pull()
	defer stop()
	for {
		value, valid := next()
		if !valid {
			break
		}
		for _, handler := range handlers {
			wg.Add(1)
			go func(h func(T), v T) {
				defer wg.Done()
				h(v)
			}(handler, value)
		}
	}
	wg.Wait()
}

// Throttle limits the number of concurrent goroutines running action.
func Throttle[T any](i *sequence.Iterator[T], concurrency int, action func(T)) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)
	next, stop := i.Pull()
	defer stop()
	for {
		value, valid := next()
		if !valid {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(v T) {
			defer wg.Done()
			action(v)
			<-sem
		}(value)
	}
	wg.Wait()
}

// Race runs all actions concurrently and returns the result of the first to finish.
// The resultFn should return (result, true) if it wants to return a value, otherwise (zero, false).
func Race[T any, R any](i *sequence.Iterator[T], resultFn func(T) (R, bool)) (R, bool) {
	var once sync.Once
	var result R
	var found bool
	var wg sync.WaitGroup
	next, stop := i.Pull()
	defer stop()
	for {
		value, valid := next()
		if !valid {
			break
		}
		wg.Add(1)
		go func(v T) {
			defer wg.Done()
			if r, ok := resultFn(v); ok {
				once.Do(func() {
					result = r
					found = true
				})
			}
		}(value)
	}
	wg.Wait()
	return result, found
}

// Batch processes elements in chunks of size batchSize, each chunk in a separate goroutine.
func Batch[T any](i *sequence.Iterator[T], batchSize int, action func([]T)) {
	in := i.Collect()
	var wg sync.WaitGroup
	for idx := 0; idx < len(in); idx += batchSize {
		end := idx + batchSize
		if end > len(in) {
			end = len(in)
		}
		wg.Add(1)
		go func(chunk []T) {
			defer wg.Done()
			action(chunk)
		}(in[idx:end])
	}
	wg.Wait()
}
