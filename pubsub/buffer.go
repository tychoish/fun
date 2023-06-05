package pubsub

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/tychoish/fun"
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
func (d Distributor[T]) Producer() fun.Producer[T]              { return d.Receive }
func (d Distributor[T]) Processor() fun.Processor[T]            { return d.Send }

// Iterator allows iterator-like access to a
// distributor. These iterators are blocking and destructive. The
// iterator's close method does *not* close the distributor's
// underlying structure.
func (d Distributor[T]) Iterator() *fun.Iterator[T] { return d.Producer().Generator() }

func ignorePopContext[T any](in func(T) error) func(context.Context, T) error {
	return func(_ context.Context, obj T) error { return filterQueueErrorsForIterator(in(obj)) }
}

func filterQueueErrorsForIterator(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrQueueClosed):
		return io.EOF
	default:
		return err
	}
}

// DistributorChannel provides a bridge between channels and
// distributors, and has expected FIFO semantics with blocking reads
// and writes.
func DistributorChannel[T any](ch chan T) Distributor[T] {
	chop := fun.Blocking(ch)
	send := chop.Send()
	recv := chop.Receive()
	return Distributor[T]{
		push: send.Write,
		pop: func(ctx context.Context) (out T, err error) {
			out, err = recv.Read(ctx)
			switch {
			case err == nil:
				return out, nil
			case errors.Is(err, ErrQueueClosed):
				fmt.Println(">>>", err)
				return out, io.EOF
			default:
				return
			}
		},
		size: func() int { return len(ch) },
	}
}
