package sequence

import (
	"iter"
	"sort"
)

// Iterator is a generic, immutable, chainable iterator for any type T.
type Iterator[T any] struct {
	seq iter.Seq[T]
}

// From creates a new Iterator from a slice of T.
func From[T any](data []T) *Iterator[T] {
	return &Iterator[T]{
		seq: func(yield func(T) bool) {
			for _, v := range data {
				yield(v)
			}
		},
	}
}

func FromMap[T any, K comparable](data map[K]T) *Iterator[T] {
	return &Iterator[T]{
		seq: func(yield func(T) bool) {
			for _, v := range data {
				yield(v)
			}
		},
	}
}

// Seq returns the underlying sequence function for the iterator.
// This allows direct access to the iterator's sequence for advanced use cases.
func (i *Iterator[T]) Seq() iter.Seq[T] {
	return i.seq
}

// Pull pulls the next element from the iterator and returns it along with a boolean indicating whether the element was valid.
func (i *Iterator[T]) Pull() (next func() (T, bool), stop func()) {
	return iter.Pull(i.Seq())
}

// FromChannel creates an Iterator from a channel of T.
func FromChannel[T any](ch <-chan T) *Iterator[T] {
	return &Iterator[T]{
		seq: func(yield func(T) bool) {
			for v := range ch {
				yield(v)
			}
		},
	}
}

// Collect exhausts the iterator and returns a slice of all elements.
func (i *Iterator[T]) Collect() []T {
	var out []T
	i.seq(func(v T) bool {
		out = append(out, v)
		return true
	})
	return out
}

// Sort returns a new Iterator with elements sorted according to the provided less function.
// The less function should return true if a < b.
// Example: it.Sort(func(a, b MyStruct) bool { return a.Field < b.Field })
func (i *Iterator[T]) Sort(less func(a, b T) bool) *Iterator[T] {
	data := i.Collect()
	sort.SliceStable(data, func(a, b int) bool {
		return less(data[a], data[b])
	})
	return From(data)
}

// Each applies the action to every element in the iterator.
func (i *Iterator[T]) Each(action func(T)) *Iterator[T] {
	return &Iterator[T]{
		seq: func(yield func(T) bool) {
			i.seq(func(v T) bool {
				action(v)
				return true
			})
		},
	}
}

// Filter returns a new Iterator containing only elements that satisfy the predicate.
func (i *Iterator[T]) Filter(pred func(T) bool) *Iterator[T] {
	return &Iterator[T]{
		seq: func(yield func(T) bool) {
			i.seq(func(v T) bool {
				if pred(v) {
					return yield(v)
				}
				return true
			})
		},
	}
}

// Reduce reduces the iterator to a single value using the reducer function and initial value.
func (i *Iterator[T]) Reduce(init T, reducer func(T, T) T) T {
	acc := init
	i.seq(func(v T) bool {
		acc = reducer(acc, v)
		return true
	})
	return acc
}

// Find returns the first element matching the predicate, or false if not found.
func (i *Iterator[T]) Find(pred func(T) bool) (T, bool) {
	var zero T
	found := false
	i.seq(func(v T) bool {
		if pred(v) {
			zero = v
			found = true
			return false
		}
		return true
	})
	return zero, found
}

// Any returns true if any element matches the predicate.
func (i *Iterator[T]) Any(pred func(T) bool) bool {
	found := false
	i.seq(func(v T) bool {
		if pred(v) {
			found = true
			return false
		}
		return true
	})
	return found
}

// All returns true if all elements match the predicate.
func (i *Iterator[T]) All(pred func(T) bool) bool {
	all := true
	i.seq(func(v T) bool {
		if !pred(v) {
			all = false
			return false
		}
		return true
	})
	return all
}

// Take returns a new Iterator with the first n elements.
func (i *Iterator[T]) Take(n int) *Iterator[T] {
	return &Iterator[T]{
		seq: func(yield func(T) bool) {
			count := 0
			i.seq(func(v T) bool {
				if count < n {
					count++
					return yield(v)
				}
				return false
			})
		},
	}
}

// Drop returns a new Iterator skipping the first n elements.
func (i *Iterator[T]) Drop(n int) *Iterator[T] {
	return &Iterator[T]{
		seq: func(yield func(T) bool) {
			count := 0
			i.seq(func(v T) bool {
				if count < n {
					count++
					return true
				}
				return yield(v)
			})
		},
	}
}

// Distinct returns a new Iterator with duplicate elements removed (using a map for equality).
func (i *Iterator[T]) Distinct() *Iterator[T] {
	return &Iterator[T]{
		seq: func(yield func(T) bool) {
			seen := make(map[any]struct{})
			i.seq(func(v T) bool {
				key := any(v)
				if _, ok := seen[key]; !ok {
					seen[key] = struct{}{}
					return yield(v)
				}
				return true
			})
		},
	}
}

// Partition splits elements into two slices based on a predicate.
func (i *Iterator[T]) Partition(pred func(T) bool) (matches, rest []T) {
	i.seq(func(v T) bool {
		if pred(v) {
			matches = append(matches, v)
		} else {
			rest = append(rest, v)
		}
		return true
	})
	return
}

// First returns the first element, or false if empty.
func (i *Iterator[T]) First() (T, bool) {
	var zero T
	found := false
	i.seq(func(v T) bool {
		zero = v
		found = true
		return false
	})
	return zero, found
}

// Last returns the last element, or false if empty.
func (i *Iterator[T]) Last() (T, bool) {
	var zero T
	found := false
	i.seq(func(v T) bool {
		zero = v
		found = true
		return true
	})
	return zero, found
}

// Count returns the number of elements in the iterator.
func (i *Iterator[T]) Count() int {
	count := 0
	i.seq(func(_ T) bool {
		count++
		return true
	})
	return count
}

// Reverse returns a new Iterator with elements in reverse order (eager).
func (i *Iterator[T]) Reverse() *Iterator[T] {
	data := i.Collect()
	for l, r := 0, len(data)-1; l < r; l, r = l+1, r-1 {
		data[l], data[r] = data[r], data[l]
	}
	return From(data)
}

// Flatten flattens an iterator of slices into a single iterator.
func Flatten[T any](it *Iterator[[]T]) *Iterator[T] {
	return &Iterator[T]{
		seq: func(yield func(T) bool) {
			it.seq(func(slice []T) bool {
				for _, v := range slice {
					if !yield(v) {
						return false
					}
				}
				return true
			})
		},
	}
}

// Chain concatenates multiple iterators into one.
func Chain[T any](iters ...*Iterator[T]) *Iterator[T] {
	return &Iterator[T]{
		seq: func(yield func(T) bool) {
			for _, it := range iters {
				it.seq(func(v T) bool {
					return yield(v)
				})
			}
		},
	}
}

// GroupBy groups elements by a key function, returning a map from key to slice of T.
func GroupBy[T any, K comparable](it *Iterator[T], keyFn func(T) K) map[K][]T {
	groups := make(map[K][]T)
	it.seq(func(v T) bool {
		k := keyFn(v)
		groups[k] = append(groups[k], v)
		return true
	})
	return groups
}

// ToMap builds a map from the iterator using key and value selector functions.
func ToMap[T any, K comparable, V any](it *Iterator[T], keyFn func(T) K, valFn func(T) V) map[K]V {
	m := make(map[K]V, it.Count())
	it.seq(func(v T) bool {
		m[keyFn(v)] = valFn(v)
		return true
	})
	return m
}

// ToSet builds a set (map[T]struct{}) from the iterator.
func ToSet[T comparable](it *Iterator[T]) map[T]struct{} {
	set := make(map[T]struct{})
	it.seq(func(v T) bool {
		set[v] = struct{}{}
		return true
	})
	return set
}

// ToArray applies the callback function to each element of the iterator and returns a slice of the results.
// It transforms elements from type T to type S using the provided callback.
func ToArray[T any, S any](it *Iterator[T], callback func(T) S) []S {
	arr := make([]S, 0, it.Count())
	it.seq(func(v T) bool {
		arr = append(arr, callback(v))
		return true
	})
	return arr
}

/*
Transform applies a mutation function to each element of a slice and returns a new slice with the transformed elements.

Parameters:

	data   - input slice of any type.
	mutate - function to apply to each element.

Returns:

	A new slice containing the mutated elements.
*/
func Transform[T any](data []T, mutate func(T) T) []T {
	transformed := make([]T, len(data))
	for idx, value := range data {
		transformed[idx] = mutate(value)
	}
	return transformed
}

// TransformIterator returns an iterator that yields the index and mutated value
// for each element in the input slice. The mutate function is applied to each
// element before yielding. Iteration stops if the yield function returns false.
func TransformIterator[T any](data []T, mutate func(T) T) iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		for idx, value := range data {
			if !yield(idx, mutate(value)) {
				return
			}
		}
	}
}

// Join returns an iterator that yields join(v, w) for each pair of elements from first and second.
// If one iterator is shorter, zero value is used for missing elements.
// The resulting iterator has length equal to the longer of the two.
func Join[T any, S any, R any](first *Iterator[T], second *Iterator[S], join func(T, S) R) *Iterator[R] {
	return &Iterator[R]{
		seq: func(yield func(R) bool) {
			firstCh := make(chan T)
			secondCh := make(chan S)
			go func() {
				first.seq(func(v T) bool { firstCh <- v; return true })
				close(firstCh)
			}()
			go func() {
				second.seq(func(v S) bool { secondCh <- v; return true })
				close(secondCh)
			}()
			var zeroT T
			var zeroS S
			for {
				var v T
				var w S
				var ok1, ok2 bool
				v, ok1 = <-firstCh
				w, ok2 = <-secondCh
				if !ok1 && !ok2 {
					break
				}
				if !ok1 {
					v = zeroT
				}
				if !ok2 {
					w = zeroS
				}
				if !yield(join(v, w)) {
					break
				}
			}
		},
	}
}
