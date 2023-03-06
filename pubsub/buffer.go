package pubsub

import (
	"context"
	"errors"
	"io"
	"sync/atomic"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
)

// Distributor provides a layer of indirection above queue-like
// implementations (e.g. Queue, Deque and channels) for buffering and
// queuing objects for use by higher level pubsub mechanisms like the
// Broker.
//
// Implementations should provide Send/Recieve operations that are
// blocking and wait for the underlying structure to be "closed", the
// context to be canceled, or have capacity (Send), or new objects
// (Recieve). These operations *should* mutate the underlying data
// structure or service.
type Distributor[T any] interface {
	Send(context.Context, T) error
	Receive(context.Context) (T, error)
}

type distributorImpl[T any] struct {
	push func(context.Context, T) error
	pop  func(context.Context) (T, error)
}

func (d *distributorImpl[T]) Send(ctx context.Context, in T) error   { return d.push(ctx, in) }
func (d *distributorImpl[T]) Receive(ctx context.Context) (T, error) { return d.pop(ctx) }

func ignorePopContext[T any](in func(T) error) func(context.Context, T) error {
	return func(_ context.Context, obj T) error { return in(obj) }
}

// DistributorLIFO produces a Deque-based Distributor that, when the
// user attempts to Push onto a full Deque, it will remove the first
// element in the list, while all Pop operations are also to the front
// of the Deque.
func DistributorLIFO[T any](d *Deque[T]) Distributor[T] {
	return &distributorImpl[T]{
		push: ignorePopContext(d.ForcePushFront),
		pop:  d.WaitFront,
	}
}

// DistributorDeque produces a Distributor that always accepts new
// Push operations by removing the oldest element in the queue, with
// Pop operations returning the oldest elements first (FIFO).
func DistributorDeque[T any](d *Deque[T]) Distributor[T] {
	return &distributorImpl[T]{
		push: ignorePopContext(d.ForcePushBack),
		pop:  d.WaitFront,
	}
}

// DistributorQueue uses a Queue implementation for FIFO distribution
// of items. If the queue is at capacity, push operations will return
// ErrQueueFull errors, effectively allowing the user of the
// distributor to drop new messages before they enter the queue..
func DistributorQueue[T any](q *Queue[T]) Distributor[T] {
	return &distributorImpl[T]{
		push: ignorePopContext(q.Add),
		pop: func(ctx context.Context) (T, error) {
			msg, ok := q.Remove()
			if ok {
				return msg, nil
			}
			return q.Wait(ctx)
		},
	}
}

// DistributorChannel provides a bridge between channels and
// distributors, and has expected FIFO semantics with blocking reads
// and writes.
func DistributorChannel[T any](ch chan T) Distributor[T] {
	ec := &erc.Collector{}
	return &distributorImpl[T]{
		push: func(ctx context.Context, in T) (err error) {
			// this only happens if we've already paniced
			// so we shouldn't get too many errors.
			if ec.HasErrors() {
				return ec.Resolve()
			}
			defer erc.Recover(ec)
			select {
			case <-ctx.Done():
				ec.Add(ctx.Err())
			case ch <- in:
			}
			return ec.Resolve()
		},
		pop: func(ctx context.Context) (T, error) {
			val, err := fun.ReadOne(ctx, ch)
			if err != nil && errors.Is(err, io.EOF) {
				return val, ErrQueueClosed
			}
			return val, err
		},
	}
}

// DistributorIterator allows iterator-like access to a
// distributor. These iterators are blocking and destructive. The
// iterator's close method does *not* close the distributor's
// underlying structure.
func DistributorIterator[T any](dist Distributor[T]) fun.Iterator[T] {
	return &distIter[T]{dist: dist}
}

type distIter[T any] struct {
	dist   Distributor[T]
	closed atomic.Bool
	value  T
}

func (d *distIter[T]) Close() error {
	if !d.closed.Swap(true) {
		d.value = *new(T)
	}
	return nil
}

func (d *distIter[T]) Value() T { return d.value }

func (d *distIter[T]) Next(ctx context.Context) bool {
	if d.closed.Load() || ctx.Err() != nil {
		return false
	}
	val, err := d.dist.Receive(ctx)
	if err != nil {
		_ = d.Close()
		return false
	}
	d.value = val

	return true
}
