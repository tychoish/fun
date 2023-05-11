// Package fun is a zero-dependency collection of tools and idoms that
// takes advantage of generics. Iterators, error handling, a
// native-feeling Set type, and a simple pub-sub framework for
// distributing messages in fan-out patterns.
package fun

import (
	"context"
	"io"

	"github.com/tychoish/fun/internal"
)

// Iterator provides a safe, context-respecting iterator paradigm for
// iterable objects, along with a set of consumer functions and basic
// implementations.
//
// The itertool package provides a number of tools and paradigms for
// creating and processing Iterator objects, including Generators, Map
// and Reduce as well as Split and Merge to combine or divide
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
// The error returned captures any panics encountered as an error, as
// well as the output of the Close() operation.
func Observe[T any](ctx context.Context, iter Iterator[T], fn Observer[T]) (err error) {
	defer func() { err = internal.MergeErrors(err, iter.Close()) }()
	defer func() { err = mergeWithRecover(err, recover()) }()

	for {
		var item T
		item, err = IterateOne(ctx, iter)
		if err != nil {
			return nil // channel closed or complete.
		}

		if err = fn.Safe(item); err != nil {
			return err
		}
	}
}

// ObserveWorker has the same semantics as Observe, except that the
// operation is wrapped in a WaitFunc, and executed when the WaitFunc
// is called.
func ObserveWorker[T any](iter Iterator[T], fn Observer[T]) WorkerFunc {
	return func(ctx context.Context) error { return Observe(ctx, iter, fn) }
}

type readOneable[T any] interface {
	ReadOne(ctx context.Context) (T, error)
}

// IterateOne, like ReadOne reads one value from the iterator, and
// returns it. The error values are either a context cancelation error
// if the context is canceled, or io.EOF if there are no elements in
// the iterator. The semantics of fun.IterateOne and fun.ReadOne are
// the same.
//
// IterateOne does not provide atomic exclusion if multiple calls to
// the iterator or IterateOne happen concurrently; however, the
// adt.NewIterator wrapper, and most of the iterator implementations
// provided by the fun package, provide a special case which *does*
// allow for safe and atomic concurrent use with fun.IterateOne.
func IterateOne[T any](ctx context.Context, iter Iterator[T]) (T, error) {
	if si, ok := iter.(readOneable[T]); ok {
		return si.ReadOne(ctx)
	}

	if err := ctx.Err(); err != nil {
		return ZeroOf[T](), err
	}

	if iter.Next(ctx) {
		return iter.Value(), nil
	}

	return ZeroOf[T](), io.EOF
}

// IterateOneBlocking has the same semantics as IterateOne except it
// uses a blocking context, and if the iterator is blocking and there
// are no more items, IterateOneBlocking will never return. Use with
// caution, and in situations where you understand the iterator's
// implementation.
func IterateOneBlocking[T any](iter Iterator[T]) (T, error) {
	return IterateOne(internal.BackgroundContext, iter)
}

// Generator creates an iterator that produces new values, using the
// generator function provided. This implementation does not create
// any background go routines, and the iterator will produce values
// until the function returns an error or the Close() method is
// called. Any non-nil error returned by the generator function is
// propagated to the close method, as long as it is not a context
// cancellation error or an io.EOF error.
func Generator[T any](op func(context.Context) (T, error)) Iterator[T] {
	return internal.NewGeneratorIterator(op)
}

// Transform processes the input iterator of type I into an output
// iterator of type O. It's implementation uses the Generator, will
// continue producing values as long as the input iterator produces
// values, the context isn't canceled, or
func Transform[I, O any](iter Iterator[I], op func(in I) (O, error)) Iterator[O] {
	return Generator(func(ctx context.Context) (O, error) {
		item, err := IterateOne(ctx, iter)
		if err != nil {
			return ZeroOf[O](), err
		}

		return op(item)
	})
}

// Filter passes every item in the input iterator and, if the check
// function returns true propogates it to the output iterator.
// There is no buffering, and check functions should return
// quickly. For more advanced use, consider using itertool.Map()
func Filter[T any](iter Iterator[T], check func(T) bool) Iterator[T] {
	return Generator(func(ctx context.Context) (T, error) {
		for {
			item, err := IterateOne(ctx, iter)
			if err != nil {
				return ZeroOf[T](), err
			}

			if check(item) {
				return item, nil
			}
		}
	})
}

// Count returns the number of items observed by the iterator. Callers
// should still manually call Close on the iterator.
func Count[T any](ctx context.Context, iter Iterator[T]) int {
	count := 0
	for {
		if _, err := IterateOne(ctx, iter); err != nil {
			break
		}
		count++
	}
	return count
}
