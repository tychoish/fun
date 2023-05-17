package pubsub

import (
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/erc"
)

// Distributor provides a layer of indirection above queue-like
// implementations (e.g. Queue, Deque, and channels) for buffering and
// queuing objects for use by higher level pubsub mechanisms like the
// Broker.
type Distributor[T any] struct {
	push func(context.Context, T) error
	pop  func(context.Context) (T, error)
	size func() int
}

func (d Distributor[T]) Len() int                               { return d.size() }
func (d Distributor[T]) Send(ctx context.Context, in T) error   { return d.push(ctx, in) }
func (d Distributor[T]) Receive(ctx context.Context) (T, error) { return d.pop(ctx) }

// Iterator allows iterator-like access to a
// distributor. These iterators are blocking and destructive. The
// iterator's close method does *not* close the distributor's
// underlying structure.
func (d Distributor[T]) Iterator() fun.Iterator[T] {
	return adt.NewIterator[T](&sync.Mutex{}, &distIter[T]{dist: d})
}

func ignorePopContext[T any](in func(T) error) func(context.Context, T) error {
	return func(_ context.Context, obj T) error { return in(obj) }
}

// DistributorChannel provides a bridge between channels and
// distributors, and has expected FIFO semantics with blocking reads
// and writes.
func DistributorChannel[T any](ch chan T) Distributor[T] {
	return Distributor[T]{
		push: func(ctx context.Context, in T) (err error) {
			// this only happens if we've already paniced
			// so we shouldn't get too many errors.
			ec := &erc.Collector{}
			defer func() { err = ec.Resolve() }()
			defer erc.Recover(ec)
			ec.Add(fun.Blocking(ch).Send().Write(ctx, in))
			return
		},
		pop: func(ctx context.Context) (T, error) {
			val, err := fun.Blocking(ch).Recieve().Read(ctx)
			if err != nil && errors.Is(err, io.EOF) {
				return val, ErrQueueClosed
			}
			return val, err
		},
		size: func() int { return len(ch) },
	}
}

type distIter[T any] struct {
	dist   Distributor[T]
	closed atomic.Bool
	value  T
}

func (d *distIter[T]) Close() error {
	if !d.closed.Swap(true) {
		d.value = fun.ZeroOf[T]()
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
