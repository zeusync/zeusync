package models

// Iterator is an interface for iterating over a collection of items.
// It provides methods to move to the next item, retrieve the current item, and check for errors.
// It also implements the io.Closer interface to ensure that the iterator is properly closed after use.
// The ToSlice method converts the iterator to a slice of items.
// The Count method returns the number of items in the iterator.
type Iterator[T any] interface {
	Next() bool
	Item() T
	Error() error
	Close() error
	ToSlice() []T
	Count() int
}

// DualIterator is an interface for iterating over a collection of items with a key.
// It provides methods to move to the next item, retrieve the current item, and check for errors.
// It also implements the io.Closer interface to ensure that the iterator is properly closed after use.
// The ToSlice method converts the iterator to a slice of items.
// The Count method returns the number of items in the iterator.
type DualIterator[T any, K comparable] interface {
	Next() bool
	Item() T
	Error() error
	Close() error
	Count() int
	ToSlice() []T
	ToMap() map[K]T
}
