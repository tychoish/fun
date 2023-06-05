package itertool

import (
	"context"
	"errors"
	"io"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/internal"
)

// Generate creates an iterator using a generator pattern which
// produces items until the generator function returns
// io.EOF, or the context (passed to the first call to
// Next()) is canceled. Parallel operation, and continue on
// error/continue-on-panic semantics are available and share
// configuration with the Map and ParallelForEach operations.
func Generate[T any](
	fn fun.Producer[T],
	opts Options,
) *fun.Iterator[T] {
	if opts.OutputBufferSize < 0 {
		opts.OutputBufferSize = 0
	}

	if opts.NumWorkers <= 0 {
		opts.NumWorkers = 1
	}

	pipe := make(chan T, opts.OutputBufferSize)
	ec := &erc.Collector{}

	init := fun.WaitFunc(func(ctx context.Context) {
		wctx, cancel := context.WithCancel(ctx)

		wg := &fun.WaitGroup{}

		for i := 0; i < opts.NumWorkers; i++ {
			generator(ec, opts, fn, cancel, pipe).Add(wctx, wg)
		}

		wg.WaitFunc().AddHook(func() { cancel(); close(pipe) }).Go(ctx)
	}).Once()

	out := fun.Producer[T](func(ctx context.Context) (T, error) {
		init(ctx) // runs once
		return fun.Blocking(pipe).Receive().Read(ctx)
	}).GeneratorWithHook(func(iter *fun.Iterator[T]) {
		iter.AddError(ec.Resolve())
	})

	return out
}

func generator[T any](
	catcher *erc.Collector,
	opts Options,
	fn fun.Producer[T],
	abort func(),
	out chan T,
) fun.WaitFunc {
	return func(ctx context.Context) {
		defer abort()

		shouldCollectError := opts.wrapErrorCheck(func(err error) bool { return !errors.Is(err, io.EOF) })
		for {
			value, err := func() (out T, err error) {
				defer func() {
					if r := recover(); r != nil {
						err = internal.ParsePanic(r, fun.ErrRecoveredPanic)
					}
				}()
				return fn(ctx)
			}()

			if err != nil {
				if errors.Is(err, fun.ErrRecoveredPanic) {
					catcher.Add(err)
					if opts.ContinueOnPanic {
						continue
					}

					return
				}

				erc.When(catcher, shouldCollectError(err), err)
				if internal.IsTerminatingError(err) || !opts.ContinueOnError {
					return
				}

				continue
			}

			if !fun.Blocking(out).Send().Check(ctx, value) {
				return
			}
		}
	}
}
