// Package itertool provides a set of functional helpers for
// managinging and using fun.Iterator implementations, including a
// parallel Map/Reduce, Merge, and other convenient tools.
package itertool

import (
	"context"
	"sync"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/internal"
)

// Merge combines a set of related iterators into a single
// iterator. Starts a thread to consume from each iterator and does
// not otherwise guarantee the iterator's order.
func Merge[T any](iters ...fun.Iterator[T]) fun.Iterator[T] {
	iter := &internal.GeneratorIterator[T]{}

	pipe := internal.MnemonizeContext(func(ctx context.Context) <-chan T {
		pipe := make(chan T)
		wg := &fun.WaitGroup{}
		wctx, cancel := context.WithCancel(ctx)

		for idx := range iters {
			it := iters[idx]
			fun.WorkerFunc(func(ctx context.Context) error {
				for {
					if wctx.Err() != nil {
						return nil
					}

					value, err := fun.IterateOne(ctx, it)
					if err != nil {
						return nil
					}
					err = fun.Blocking(pipe).Send().Write(ctx, value)
					if err != nil {
						return nil
					}
				}
			}).Add(wctx, wg, func(err error) {})
		}

		go func() { wg.Wait(wctx); close(pipe) }()

		iter.Closer = func() { cancel() }

		return pipe
	})

	iter.Operation = func(ctx context.Context) (T, error) {
		return fun.ReadOne(ctx, pipe(ctx))
	}

	return iter
}

// Split produces an arbitrary number of iterators which divide the
// input. The division is lazy and depends on the rate of consumption
// of output iterators, but every item from the input iterator is sent
// to exactly one output iterator, each of which can be safely used
// from a different go routine.
//
// The input iterator is not closed after the output iterators are
// exhausted. There is one background go routine that reads items off
// of the input iterator, which starts when the first output iterator
// is advanced: be aware that canceling this context will effectively
// cancel all iterators.
func Split[T any](numSplits int, input fun.Iterator[T]) []fun.Iterator[T] {
	if numSplits <= 0 {
		return nil
	}

	pipe := internal.MnemonizeContext(func(ctx context.Context) <-chan T {
		pipe := make(chan T)
		wg := &fun.WaitGroup{}
		fun.WorkerFunc(func(ctx context.Context) error {
			for {
				value, err := fun.IterateOne(ctx, input)
				if err != nil {
					return nil
				}
				if !fun.Blocking(pipe).Send().Check(ctx, value) {
					return nil
				}
			}
		}).Add(ctx, wg, func(error) {})
		go func() { fun.WaitFunc(wg.Wait).Block(); close(pipe) }()

		return pipe
	})

	output := make([]fun.Iterator[T], numSplits)

	for idx := range output {
		output[idx] = &internal.GeneratorIterator[T]{
			Operation: func(ctx context.Context) (T, error) {
				return fun.ReadOne(ctx, pipe(ctx))
			},
		}
	}

	return output
}

// RangeFunction describes a function that operates similar to the
// range keyword in the language specification, but that bridges the
// gap between fun.Iterators and range statements.
type RangeFunction[T any] func(context.Context, *T) bool

// Range produces a function that can be used like an iterator, but
// that is safe for concurrent use from multiple go
// routines. (e.g. the output of the function synchronizes the output
// of Next() and Value()): for example:
//
//	var out type
//	for rf(ctx, &out) {
//	     // do work
//	}
//
// Range does not provide a convenient way to close or access the
// error state of the iterator, which you must synchronize on your
// own. The safety of range assumes that you do not interact with the
// iterator outside of the range function.
func Range[T any](iter fun.Iterator[T]) RangeFunction[T] {
	mtx := &sync.Mutex{}

	return func(ctx context.Context, out *T) bool {
		mtx.Lock()
		defer mtx.Unlock()

		val, err := fun.IterateOne(ctx, iter)
		if err != nil {
			*out = fun.ZeroOf[T]()
			return false
		}
		*out = val
		return true
	}
}
