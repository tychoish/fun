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
	fn func(context.Context) (T, error),
	opts Options,
) fun.Iterator[T] {
	if opts.OutputBufferSize < 0 {
		opts.OutputBufferSize = 0
	}

	if opts.NumWorkers <= 0 {
		opts.NumWorkers = 1
	}

	wg := &fun.WaitGroup{}
	pipe := make(chan T, opts.OutputBufferSize)
	catcher := &erc.Collector{}

	out := &internal.GeneratorIterator[T]{
		Error: catcher.Resolve,
	}

	init := fun.WaitFunc(func(ctx context.Context) {
		ctx, cancel := context.WithCancel(ctx)
		out.Closer = cancel

		for i := 0; i < opts.NumWorkers; i++ {
			generator(catcher, opts, fn, cancel, pipe).Add(ctx, wg)
		}
		go func() {
			// waits for the contest to be canceled
			// (because Close(), or called in a defer in
			// generator()) AND all workers to exit. It's
			// blocking(background context), but safe
			// because logically once the context in this
			// function expires, it must espire shortly
			// there after.
			//
			// a blocking/deadlock in the generator
			// function is possible, and there's not much
			// to be done there.
			fun.WaitMerge(Variadic(fun.WaitContext(ctx), wg.Wait)).Block()
			// once everyone is done, we can safely close
			// the pipe.
			close(pipe)
		}()
	}).Once()

	out.Operation = func(ctx context.Context) (T, error) { init(ctx); return fun.Blocking(pipe).Recieve().Read(ctx) }

	return Synchronize[T](out)
}

func generator[T any](
	catcher *erc.Collector,
	opts Options,
	fn func(context.Context) (T, error),
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

				if errors.Is(err, io.EOF) || erc.ContextExpired(err) || !opts.ContinueOnError {
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
