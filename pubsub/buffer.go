package pubsub

import (
	"context"
	"errors"
	"io"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
)

type Distributor[T any] struct {
	pushImpl func(context.Context, T) error
	popImpl  func(context.Context) (T, error)
}

func (d *Distributor[T]) Push(ctx context.Context, in T) error {
	return d.pushImpl(ctx, in)
}
func (d *Distributor[T]) Pop(ctx context.Context) (T, error) {
	return d.popImpl(ctx)
}

func ignorePopContext[T any](in func(T) error) func(context.Context, T) error {
	return func(_ context.Context, obj T) error { return in(obj) }
}

func DistributorLIFO[T any](d *Deque[T]) *Distributor[T] {
	return &Distributor[T]{
		pushImpl: ignorePopContext(d.ForcePushFront),
		popImpl:  d.WaitFront,
	}
}

func DistributorDeque[T any](d *Deque[T]) *Distributor[T] {
	return &Distributor[T]{
		pushImpl: ignorePopContext(d.ForcePushBack),
		popImpl:  d.WaitFront,
	}
}

func DistributorQueue[T any](q *Queue[T]) *Distributor[T] {
	return &Distributor[T]{
		pushImpl: ignorePopContext(q.Add),
		popImpl: func(ctx context.Context) (T, error) {
			msg, ok := q.Remove()
			if ok {
				return msg, nil
			}
			return q.Wait(ctx)
		},
	}
}

func DistributorBuffer[T any](d *Deque[T]) *Distributor[T] {
	return &Distributor[T]{
		pushImpl: d.WaitPushBack,
		popImpl:  d.WaitFront,
	}
}

func DistributorChannel[T any](ch chan T) *Distributor[T] {
	ec := &erc.Collector{}
	return &Distributor[T]{
		pushImpl: func(ctx context.Context, in T) error {
			// this only happens if we've already paniced
			// so we shouldn't get too many errors.
			if ec.HasErrors() {
				return ec.Resolve()
			}

			defer erc.Recover(ec)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case ch <- in:
				return ec.Resolve()
			}
		},
		popImpl: func(ctx context.Context) (T, error) {
			val, err := fun.ReadOne(ctx, ch)
			if errors.Is(err, io.EOF) {
				return val, ErrQueueClosed
			}
			return val, err
		},
	}
}
