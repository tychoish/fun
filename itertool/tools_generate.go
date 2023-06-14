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

	fn = fn.Safe()

	oberr := fun.Observer[error](ec.Add).Filter(opts.ErrorFilter())
	init := fun.Operation(func(ctx context.Context) {

		wctx, cancel := context.WithCancel(ctx)
		wg := &fun.WaitGroup{}
		send := pipe.Send()

		generator(fn, send, oberr, opts).
			Wait(func(err error) {
				fun.WhenCall(errors.Is(err, io.EOF), cancel)
			}).
			StartGroup(wctx, wg, opts.NumWorkers)

		wg.Operation().PostHook(func() { cancel(); pipe.Close() }).Go(ctx)
	}).Once()

	return pipe.Receive().Producer().PreHook(init).IteratorWithHook(erc.IteratorHook[T](ec))
}

func generator[T any](
	fn fun.Producer[T],
	out fun.ChanSend[T],
	oberr fun.Observer[error],
	opts Options,
) fun.Worker {
	return func(ctx context.Context) error {
		for {
			if value, err := fn(ctx); err != nil {
				if opts.HandleErrors(oberr, err) {
					continue
				}
				return io.EOF
			} else if !out.Check(ctx, value) {
				return io.EOF
			}
		}
	}
}
