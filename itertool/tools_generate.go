package itertool

import (
	"context"
	"errors"
	"io"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
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
	opts.init()

	pipe := fun.Blocking(make(chan T, opts.OutputBufferSize))
	ec := &erc.Collector{}

	init := fun.WaitFunc(func(ctx context.Context) {
		wctx, cancel := context.WithCancel(ctx)
		wg := &fun.WaitGroup{}
		send := pipe.Send()

		generator(ec, opts, fn, cancel, send).StartGroup(wctx, wg, opts.NumWorkers)

		wg.WaitFunc().PostHook(func() { cancel(); pipe.Close() }).Go(ctx)
	}).Once()

	return pipe.Receive().Producer().PreHook(init).IteratorWithHook(erc.IteratorHook[T](ec))
}

func generator[T any](
	catcher *erc.Collector,
	opts Options,
	fn fun.Producer[T],
	abort func(),
	out fun.Send[T],
) fun.WaitFunc {
	return func(ctx context.Context) {
		defer abort()

		for {
			value, err := func() (out T, err error) {
				defer func() {
					if r := recover(); r != nil {
						err = fun.ParsePanic(r)
					}
				}()
				return fn(ctx)
			}()

			if err != nil {
				erc.When(catcher, opts.shouldCollectError(err), err)
				if errors.Is(err, fun.ErrRecoveredPanic) {
					if opts.ContinueOnPanic {
						continue
					}

					return
				}
				switch {
				case err == nil:
					continue
				case errors.Is(err, io.EOF):
					return
				case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
					return
				case !opts.ContinueOnError:
					return
				}

				continue
			}

			if !out.Check(ctx, value) {
				return
			}
		}
	}
}
