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
func Merge[T any](ctx context.Context, iters ...fun.Iterator[T]) fun.Iterator[T] {
	pipe := make(chan T)

	iter := &internal.MapIterImpl[T]{
		ChannelIterImpl: internal.ChannelIterImpl[T]{Pipe: pipe},
	}

	ctx, iter.Closer = context.WithCancel(ctx)
	for _, it := range iters {
		iter.WG.Add(1)
		go func(itr fun.Iterator[T]) {
			defer iter.WG.Done()
			for itr.Next(ctx) {
				select {
				case <-ctx.Done():
					return
				case pipe <- itr.Value():
					continue
				}
			}
		}(it)
	}

	// when all workers conclude, close the pipe.
	go func() { fun.Wait(ctx, &iter.WG); close(pipe) }()

	return iter
}

// Slice produces an iterator for an arbitrary slice.
func Slice[T any](in []T) fun.Iterator[T] {
	return &internal.SliceIterImpl[T]{
		Vals:  in,
		Index: -1,
	}
}

// Channel produces an iterator for a specified channel. The
// iterator does not start any background threads.
func Channel[T any](pipe <-chan T) fun.Iterator[T] {
	return &internal.ChannelIterImpl[T]{Pipe: pipe}
}

// Split produces an arbitrary number of iterators which divide the
// input. The division is lazy and depends on the rate of consumption
// of output iterators, but every item from the input iterator is sent
// to exactly one output iterator, each of which can be safely used
// from a different go routine.
func Split[T any](ctx context.Context, numSplits int, input fun.Iterator[T]) []fun.Iterator[T] {
	if numSplits <= 0 {
		return nil
	}

	pipe := CollectChannel(ctx, input)

	output := make([]fun.Iterator[T], numSplits)

	for idx := range output {
		output[idx] = Channel(pipe)
	}

	return output
}

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
func Range[T any](ctx context.Context, iter fun.Iterator[T]) RangeFunction[T] {
	mtx := &sync.Mutex{}

	return func(ctx context.Context, out *T) bool {
		mtx.Lock()
		defer mtx.Unlock()

		if iter.Next(ctx) {
			val := iter.Value()
			*out = val
			return true
		}

		*out = *new(T)
		return false
	}
}
