package pubsub

import (
	"context"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/fnx"
)

// distributor provides a layer of indirection above queue-like
// implementations (e.g. Queue, Deque, and channels) for buffering and
// queuing objects for use by higher level pubsub mechanisms like the
// Broker.
type distributor[T any] struct {
	push fnx.Handler[T]
	pop  fnx.Future[T]
	size func() int
}

// makeDistributor builds a distributor from producer and processor
// functions.
func makeDistributor[T any](
	processor fnx.Handler[T],
	producer fnx.Future[T],
	length func() int,
) distributor[T] {
	return distributor[T]{
		push: processor,
		pop:  producer,
		size: length,
	}
}

// Len returns the length of the underlying storage for the distributor.
func (d distributor[T]) Len() int { return d.size() }

// Write pushes an object into the distributor.
func (d distributor[T]) Write(ctx context.Context, in T) error { return d.push(ctx, in) }

// Read pulls an object from the distributor.
func (d distributor[T]) Read(ctx context.Context) (T, error) { return d.pop(ctx) }

// distForChannel provides a bridge between channels and
// distributors, and has expected FIFO semantics with blocking reads
// and writes.
func distForChannel[T any](ch chan T) distributor[T] {
	c := fun.Blocking(ch)
	return distForChanOp(c)
}

// distForChanOp constructs a Distributor from the channel
// operator type constructed by the root package's Blocking() and
// NonBlocking() functions.
func distForChanOp[T any](c fun.ChanOp[T]) distributor[T] {
	return makeDistributor(c.Send().Write, c.Receive().Read, c.Len)
}
