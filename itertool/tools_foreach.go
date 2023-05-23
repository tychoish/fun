package itertool

import (
	"context"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/internal"
)

// ParallelForEach processes the iterator in parallel, and is
// essentially an iterator-driven worker pool. The input iterator is
// split dynamically into iterators for every worker (determined by
// Options.NumWorkers,) with the division between workers determined
// by their processing speed (e.g. workers should not suffer from
// head-of-line blocking,) and input iterators are consumed (safely)
// as work is processed.
//
// Because there is no output in these operations
// Options.OutputBufferSize is ignored.
func ParallelForEach[T any](
	ctx context.Context,
	iter fun.Iterator[T],
	fn func(context.Context, T) error,
	opts Options,
) (err error) {
	iter = Synchronize(iter)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	catcher := &erc.Collector{}
	wg := &fun.WaitGroup{}
	defer func() { err = catcher.Resolve() }()
	defer erc.Check(catcher, iter.Close)
	defer erc.Recover(catcher)

	if opts.NumWorkers <= 0 {
		opts.NumWorkers = 1
	}

	splits := Split(opts.NumWorkers, iter)

	for idx := range splits {
		wg.Add(1)
		go forEachWorker(ctx, catcher, wg, opts, splits[idx], fn, cancel)
	}

	wg.Wait(internal.BackgroundContext)
	return
}

func forEachWorker[T any](
	ctx context.Context,
	catcher *erc.Collector,
	wg *fun.WaitGroup,
	opts Options,
	iter fun.Iterator[T],
	fn func(context.Context, T) error,
	abort func(),
) {
	defer wg.Done()
	defer erc.RecoverHook(catcher, func() {
		if ctx.Err() != nil {
			return
		}
		if !opts.ContinueOnPanic {
			abort()
			return
		}

		wg.Add(1)
		go forEachWorker(ctx, catcher, wg, opts, iter, fn, abort)
	})
	for iter.Next(ctx) {
		if err := fn(ctx, iter.Value()); err != nil {
			erc.When(catcher, opts.shouldCollectError(err), err)

			if opts.ContinueOnError {
				continue
			}

			abort()
			return
		}
	}
}
