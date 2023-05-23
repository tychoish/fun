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

	out := &internal.GeneratorIterator[T]{}

	pipe := internal.MnemonizeContext(func(ctx context.Context) <-chan T {
		pipe := make(chan T, opts.OutputBufferSize)
		catcher := &erc.Collector{}
		gctx, abort := context.WithCancel(ctx)
		wg := &fun.WaitGroup{}

		for i := 0; i < opts.NumWorkers; i++ {
			worker := generator(catcher, opts, fn, abort, pipe)
			worker.Add(gctx, wg, catcher.Add)
		}

		go func() { fun.WaitFunc(wg.Wait).Block(); close(pipe) }()
		out.Closer = func() {
			abort()
			out.Error = catcher.Resolve()
		}

		return pipe
	})

	out.Operation = func(ctx context.Context) (T, error) {
		return fun.ReadOne(ctx, pipe(ctx))
	}

	return Synchronize[T](out)
}

func generator[T any](
	catcher *erc.Collector,
	opts Options,
	fn func(context.Context) (T, error),
	abort func(),
	out chan T,
) fun.WorkerFunc {
	return func(ctx context.Context) error {
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

					return nil
				}

				erc.When(catcher, shouldCollectError(err), err)

				if errors.Is(err, io.EOF) || erc.ContextExpired(err) || !opts.ContinueOnError {
					return nil
				}

				continue
			}

			if !fun.Blocking(out).Send().Check(ctx, value) {
				return nil
			}
		}
	}
}
