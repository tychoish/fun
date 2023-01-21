// Package itertool provides a set of functional helpers for
// managinging and using fun.Iterator implementations, including a
// parallel Map/Reduce, Merge, and other convenient tools.
package itertool

import (
	"context"

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
