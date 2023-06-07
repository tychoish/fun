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

	init := fun.Operation(func(ctx context.Context) {
		wctx, cancel := context.WithCancel(ctx)
		wg := &fun.WaitGroup{}
		send := pipe.Send()

		generator(ec, opts, fn, cancel, send).StartGroup(wctx, wg, opts.NumWorkers)

		wg.Operation().PostHook(func() { cancel(); pipe.Close() }).Go(ctx)
	}).Once()

	return pipe.Receive().Producer().PreHook(init).IteratorWithHook(erc.IteratorHook[T](ec))
}

func generator[T any](
	catcher *erc.Collector,
	opts Options,
	fn fun.Producer[T],
	abort func(),
	out fun.Send[T],
) fun.Operation {
	return func(ctx context.Context) {
		defer abort()

		for {
			if value, err := func() (out T, err error) {
				defer func() { err = erc.Merge(err, fun.ParsePanic(recover())) }()
				return fn(ctx)
			}(); err != nil {
				erc.When(catcher, opts.shouldCollectError(err), err)

				if errors.Is(err, fun.ErrRecoveredPanic) {
					if opts.ContinueOnPanic {
						continue
					}
					return
				}

				if erc.ContextExpired(err) || errors.Is(err, io.EOF) || !opts.ContinueOnError {
					return
				}
			} else if !out.Check(ctx, value) {
				return
			}
		}
	}
}
