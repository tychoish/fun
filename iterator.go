// Package fun is a zero-dependency collection of tools and idoms that
// takes advantage of generics. Iterators, error handling, a
// native-feeling Set type, and a simple pub-sub framework for
// distributing messages in fan-out patterns.
package fun

import (
	"context"
)

// Iterator provides a safe, context-respecting iterator paradigm for
// iterable objects, along with a set of consumer functions and basic
// implementations.
//
// The itertool package provides a number of tools and paradigms for
// creating and processing Iterator objects, including Generators, Map
// and Reduce, Filter as well as Split and Merge to combine or divide
// iterators.
//
// In general, Iterators cannot be safe for access from multiple
// concurrent goroutines, because it is impossible to synchronize
// calls to Next() and Value(); however, itertool.Range() and
// itertool.Split() provide support for these workloads.
type Iterator[T any] interface {
	Next(context.Context) bool
	Close() error
	Value() T
}

// Observe processes an iterator calling the observer function for
// every element in the iterator and retruning when the iterator is
// exhausted. Take care to ensure that the Observe function does not
// block.
//
// Use itertool.Observe and itertool.ParallelObserve for more advanced
// execution patterns.
func Observe[T any](ctx context.Context, iter Iterator[T], observe func(T)) {
	for iter.Next(ctx) {
		observe(iter.Value())
	}
}
