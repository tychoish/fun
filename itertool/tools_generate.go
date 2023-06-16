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
	optp ...fun.OptionProvider[*fun.WorkerGroupOptions],
) *fun.Iterator[T] {
	opts := fun.WorkerGroupOptions{}
	err := fun.ApplyOptions(&opts, optp...)
	var hook = func(*fun.Iterator[T]) {}
	if opts.ErrorObserver == nil {
		ec := &erc.Collector{}
		opts.ErrorObserver = ec.Add
		opts.ErrorResolver = ec.Resolve
		hook = erc.IteratorHook[T](ec)
	}
	opts.ErrorObserver(err)
	pipe := fun.Blocking(make(chan T, opts.NumWorkers*2+1))
	fun.WhenCall(err != nil, pipe.Close)

	init := fun.Operation(func(ctx context.Context) {
		wctx, cancel := context.WithCancel(ctx)
		wg := &fun.WaitGroup{}

		fn = fn.Safe()
		var zero T

		pipe.Processor().
			ReadAll(func(ctx context.Context) (T, error) {
				if value, err := fn(ctx); err != nil {
					if opts.CanContinueOnError(err) {
						return zero, fun.ErrIteratorSkip
					}

					return zero, io.EOF
				} else {
					return value, nil
				}
			}).
			Wait(func(err error) {
				fun.WhenCall(errors.Is(err, io.EOF), cancel)
			}).
			StartGroup(wctx, wg, opts.NumWorkers)

		wg.Operation().PostHook(func() { cancel(); pipe.Close() }).Go(ctx)
	}).Once()

	return pipe.Receive().Producer().PreHook(init).IteratorWithHook(hook)
}
