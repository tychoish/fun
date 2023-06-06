package pubsub

import (
	"context"

	"github.com/tychoish/fun"
)

// Distributor provides a layer of indirection above queue-like
// implementations (e.g. Queue, Deque, and channels) for buffering and
// queuing objects for use by higher level pubsub mechanisms like the
// Broker.
type Distributor[T any] struct {
	push fun.Processor[T]
	pop  fun.Producer[T]
	size func() int
}

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

func (d Distributor[T]) Len() int                               { return d.size() }
func (d Distributor[T]) Send(ctx context.Context, in T) error   { return d.push(ctx, in) }
func (d Distributor[T]) Receive(ctx context.Context) (T, error) { return d.pop(ctx) }
func (d Distributor[T]) Producer() fun.Producer[T]              { return d.pop }
func (d Distributor[T]) Processor() fun.Processor[T]            { return d.push }

// Iterator allows iterator-like access to a
// distributor. These iterators are blocking and destructive. The
// iterator's close method does *not* close the distributor's
// underlying structure.
func (d Distributor[T]) Iterator() *fun.Iterator[T] { return d.Producer().Iterator() }

// DistributorChannel provides a bridge between channels and
// distributors, and has expected FIFO semantics with blocking reads
// and writes.
func DistributorChannel[T any](ch chan T) Distributor[T] {
	chop := fun.Blocking(ch)
	return MakeDistributor(
		chop.Send().Processor(),
		chop.Receive().Producer(),
		func() int { return len(ch) },
	)
}
