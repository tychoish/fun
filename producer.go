package fun

import (
	"context"
	"sync"

	"github.com/tychoish/fun/internal"
)

type Producer[T any] func(context.Context) (T, error)

func MakeFuture[T any](ch <-chan T) Producer[T] {
	return func(ctx context.Context) (T, error) {
		return BlockingReceive(ch).Read(ctx)
	}
}

func (pf Producer[T]) Run(ctx context.Context) (T, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	return pf(ctx)
}

func (pf Producer[T]) Background(ctx context.Context, of Observer[T]) Worker {
	return pf.Worker(of).Future(ctx)
}

func (pf Producer[T]) Worker(of Observer[T]) Worker {
	return func(ctx context.Context) error { o, e := pf(ctx); of(o); return e }
}

func (pf Producer[T]) Safe(ctx context.Context) (_ T, err error) {
	defer func() { err = mergeWithRecover(err, recover()) }()
	return pf(ctx)
}

func (pf Producer[T]) Block() (T, error) { return pf(internal.BackgroundContext) }

func (pf Producer[T]) Wait(of Observer[T], eo Observer[error]) WaitFunc {
	return func(ctx context.Context) { o, e := pf(ctx); of(o); eo(e) }
}

func (pf Producer[T]) Check(of Observer[error]) func(context.Context) T {
	return func(ctx context.Context) T { o, e := pf(ctx); of(e); return o }
}

func (pf Producer[T]) Future(ctx context.Context) Producer[T] {
	out := make(chan T, 1)
	spf := Producer[T](pf.Safe)
	var err error
	go func() { defer close(out); o, e := spf(ctx); err = e; out <- o }()

	return func(ctx context.Context) (T, error) {
		out, chErr := Blocking(out).Receive().Read(ctx)
		err = internal.MergeErrors(err, chErr)
		return out, err
	}
}

func (pf Producer[T]) Once() Producer[T] {
	var (
		out T
		err error
	)

	once := &sync.Once{}
	return func(ctx context.Context) (T, error) {
		once.Do(func() { out, err = pf(ctx) })
		return out, err
	}
}
