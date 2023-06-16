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
	optp ...OptionProvider[*Options],
) *fun.Iterator[T] {
	ec := &erc.Collector{}
	opts := Options{}
	ec.Add(Apply(&opts, optp...))
	opts.init()

	pipe := fun.Blocking(make(chan T, opts.NumWorkers*2+1))
	fun.WhenCall(ec.HasErrors(), pipe.Close)

	init := fun.Operation(func(ctx context.Context) {

		wctx, cancel := context.WithCancel(ctx)
		wg := &fun.WaitGroup{}
		send := pipe.Send()

		send.Processor().ReadAll(makeProducer(fn, ec.Add, opts)).
			Wait(func(err error) {
				fun.WhenCall(errors.Is(err, io.EOF), cancel)
			}).
			StartGroup(wctx, wg, opts.NumWorkers)

		wg.Operation().PostHook(func() { cancel(); pipe.Close() }).Go(ctx)
	}).Once()

	return pipe.Receive().Producer().PreHook(init).IteratorWithHook(erc.IteratorHook[T](ec))
}

func makeProducer[T any](
	fn fun.Producer[T],
	oberr fun.Observer[error],
	opts Options,
) fun.Producer[T] {
	fn = fn.Safe()
	var zero T
	return func(ctx context.Context) (T, error) {
		if value, err := fn(ctx); err != nil {
			if !opts.continueOnError(oberr, err) {
				return zero, io.EOF
			}
			return zero, fun.ErrIteratorSkip
		} else {
			return value, nil
		}
	}
}
