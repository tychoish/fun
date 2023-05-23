package fun

import (
	"context"

	"github.com/tychoish/fun/internal"
)

type ProcessFunc[T any] func(context.Context, T) error

func (pf ProcessFunc[T]) Run(ctx context.Context, in T) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	return pf(ctx, in)
}

func (pf ProcessFunc[T]) Block(in T) error {
	return pf.Run(internal.BackgroundContext, in)
}

func (pf ProcessFunc[T]) Safe(ctx context.Context, in T) (err error) {
	defer func() { err = mergeWithRecover(err, recover()) }()
	return pf.Run(ctx, in)
}

func (pf ProcessFunc[T]) Observe(ctx context.Context, arg T, of Observer[error]) {
	of(pf.Safe(ctx, arg))
}

func (pf ProcessFunc[T]) Worker(in T) WorkerFunc {
	return func(ctx context.Context) error {
		return pf.Run(ctx, in)
	}
}

func (pf ProcessFunc[T]) Wait(in T, of Observer[error]) WaitFunc {
	return func(ctx context.Context) { pf.Observe(ctx, in, of) }
}
