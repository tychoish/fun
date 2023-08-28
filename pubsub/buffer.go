package pubsub

import (
	"context"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/ers"
)

// Distributor provides a layer of indirection above queue-like
// implementations (e.g. Queue, Deque, and channels) for buffering and
// queuing objects for use by higher level pubsub mechanisms like the
// Broker.
//
// Distributors returned by the pubsub package provide iterators that
// are destructive, and
type Distributor[T any] struct {
	push fun.Processor[T]
	pop  fun.Producer[T]
	size func() int
}

// MakeDistributor builds a distributor from producer and processor
// functions.
func MakeDistributor[T any](
	processor fun.Processor[T],
	producer fun.Producer[T],
	length func() int,
) Distributor[T] {
	return Distributor[T]{
		push: processor,
		pop:  producer,
		size: length,
	}
}

// WithInputFilter returns a copy of the distributor where all items
// pass through a filter before being written/passed to Send. When the
// filter returns true items are propagated and are skipped otherwise.
func (d Distributor[T]) WithInputFilter(filter func(T) bool) Distributor[T] {
	out := d
	out.push = out.push.Filter(filter).WithoutErrors(ers.ErrCurrentOpSkip)
	return out
}

// WithOutputFilter returns a copy of the distributor where all items
// pass through the provided filter before being delivered to
// readers/Receive. When the filter returns true items are propagated
// and are skipped otherwise.
func (d Distributor[T]) WithOutputFilter(filter func(T) bool) Distributor[T] {
	out := d
	out.pop = out.pop.Filter(filter)
	return out
}

// Len returns the length of the underlying storage for the distributor.
func (d Distributor[T]) Len() int { return d.size() }

// Send pushes an object into the distributor.
func (d Distributor[T]) Send(ctx context.Context, in T) error { return d.push(ctx, in) }

// Receive pulls an object from the distributor.
func (d Distributor[T]) Receive(ctx context.Context) (T, error) { return d.pop(ctx) }

// Processor provides convienet access to the "send" side of the
// distributor as a fun.Processor function.
func (d Distributor[T]) Processor() fun.Processor[T] { return d.push }

// Producer provides a convenient access to the "receive" side of the
// as a fun.Producer function
func (d Distributor[T]) Producer() fun.Producer[T] { return d.pop }

// Iterator allows iterator-like access to a
// distributor. These iterators are blocking and destructive. The
// iterator's close method does *not* close the distributor's
// underlying structure.
func (d Distributor[T]) Iterator() *fun.Iterator[T] { return d.Producer().Iterator() }

// DistributorChannel provides a bridge between channels and
// distributors, and has expected FIFO semantics with blocking reads
// and writes.
func DistributorChannel[T any](ch chan T) Distributor[T] { return DistributorChanOp(fun.Blocking(ch)) }

// DistributorChanOp constructs a Distributor from the channel
// operator type constructed by the root package's Blocking() and
// NonBlocking() functions
func DistributorChanOp[T any](ch fun.ChanOp[T]) Distributor[T] {
	return MakeDistributor(ch.Send().Processor(), ch.Receive().Producer(), ch.Len)
}
