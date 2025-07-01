package concurrent

import (
	"github.com/zeusync/zeusync/pkg/sequence"
	"sync"
)

// PipelineErrorMode defines error handling mode for the pipeline.
type PipelineErrorMode int

const (
	// StopAllOnError stops the entire pipeline on the first error.
	StopAllOnError PipelineErrorMode = iota
	// StopElementOnError stops only the current element's pipeline on error, others continue.
	StopElementOnError
	// IgnoreErrors ignores all errors, all elements are processed fully.
	IgnoreErrors
)

/*
Pipeline concurrently processes elements from the input iterator through a sequence of pipe functions,
each of which may transform the value and return an error. Supports configurable error handling modes:
StopAllOnError, StopElementOnError, and IgnoreErrors. Returns an iterator of the final results and an
iterator of errors encountered during processing.

Parameters:
  - in: Input iterator of type T.
  - concurrency: Maximum number of concurrent workers.
  - mode: Error handling mode for the pipeline.
  - pipeFns: Variadic list of pipe functions, each accepting (value, valid) and returning (result, error).

Returns:
  - *collections.Iterator[R]: Iterator of processed results.
  - *collections.Iterator[error]: Iterator of errors for each element.
*/
func Pipeline[T any, R any](
	in *sequence.Iterator[T],
	concurrency int,
	mode PipelineErrorMode,
	pipeFns ...func(any, bool) (any, error),
) (*sequence.Iterator[R], *sequence.Iterator[error]) {
	input := in.Collect()
	out := make([]R, len(input))
	errList := make([]error, len(input))
	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)
	var stopAll sync.Once
	stopped := false

	for idx, val := range input {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, v T) {
			defer wg.Done()
			defer func() { <-sem }()

			var cur any = v
			valid := true
			for _, fn := range pipeFns {
				if stopped {
					return
				}
				res, err := fn(cur, valid)
				if err != nil {
					errList[i] = err
					valid = false
					switch mode {
					case StopAllOnError:
						stopAll.Do(func() { stopped = true })
						return
					case StopElementOnError:
						return
					case IgnoreErrors:
						continue
					}
				}
				cur = res
			}
			if !stopped && valid {
				if r, ok := cur.(R); ok {
					out[i] = r
				}
			}
		}(idx, val)
	}
	wg.Wait()

	return sequence.From(out), sequence.From(errList)
}

type PipeObject[R any] struct {
	Result R
	Error  error
}

/*
PipelineMapped executes a series of pipeline functions concurrently over elements from the input iterator,
returning an iterator of PipeObject[R] containing both results and errors for each element. The concurrency
level and error handling mode can be customized. Uses organization-specific collections and pipeline utilities.
*/
func PipelineMapped[T any, R any](
	in *sequence.Iterator[T],
	concurrency int,
	mode PipelineErrorMode,
	pipeFns ...func(any, bool) (any, error),
) *sequence.Iterator[PipeObject[R]] {
	objs := make([]PipeObject[R], 0, len(in.Collect()))
	resIterator, errIterator := Pipeline[T, R](in, concurrency, mode, pipeFns...)

	sequence.Join(resIterator, errIterator, func(val R, err error) PipeObject[R] {
		return PipeObject[R]{Result: val, Error: nil}
	})

	return sequence.From(objs)
}
