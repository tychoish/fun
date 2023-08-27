package pubsub

import (
	"context"

	"github.com/tychoish/fun"
)

// Distributor provides a layer of indirection above queue-like
// implementations (e.g. Queue, Deque, and channels) for buffering and
// queuing objects for use by higher level pubsub mechanisms like the
// Broker.
//
// Distributors redturned by the pubsub package provide iterators that
// are destructive:
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

func (d Distributor[T]) Filtered(filter func(T) bool) Distributor[T] {
	out := d
	out.push = out.push.Filter(filter)
	out.pop = out.pop.Filter(filter)
	return out
}

func (d Distributor[T]) WithInputFilter(filter func(T) bool) Distributor[T] {
	out := d
	out.push = out.push.Filter(filter)
	return out
}

func (d Distributor[T]) WithOutputFilter(filter func(T) bool) Distributor[T] {
	out := d
	out.push = out.push.Filter(filter)
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
func DistributorChannel[T any](ch chan T) Distributor[T] {
	return DistributorChanOp(fun.Blocking(ch))
}

// DistributorChanOp constructs a Distributor from the channel
// operator type constructed by the root package's Blocking() and
// NonBlocking() functions
func DistributorChanOp[T any](chop fun.ChanOp[T]) Distributor[T] {
	return MakeDistributor(
		chop.Send().Processor(),
		chop.Receive().Producer(),
		func() int { return len(chop.Channel()) },
	)
}
