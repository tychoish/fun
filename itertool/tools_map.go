package itertool

import (
	"context"
	"errors"
	"io"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
)

// Map provides an orthodox functional map implementation based around
// fun.Iterator. Operates in asynchronous/streaming manner, so that
// the output Iterator must be consumed. The zero values of Options
// provide reasonable defaults for abort-on-error and single-threaded
// map operation.
//
// If the mapper function errors, the result isn't included, but the
// errors would be aggregated and propagated to the `Close()` method
// of the resulting iterator. If there are more than one error (as is
// the case with a panic or with ContinueOnError semantics,) the error
// is an *erc.Stack object. Panics in the map function are converted
// to errors and handled according to the ContinueOnPanic option.
func Map[T any, O any](
	iter *fun.Iterator[T],
	mapper func(context.Context, T) (O, error),
	opts Options,
) *fun.Iterator[O] {
	opts.init()

	ec := &erc.Collector{}
	output := fun.Blocking(make(chan O))

	init := fun.Operation(func(ctx context.Context) {
		splits := iter.Split(opts.NumWorkers)

		wctx, wcancel := context.WithCancel(ctx)
		wg := &fun.WaitGroup{}

		for idx := range splits {
			// for each split, run a mapWorker
			mapWorker(ec, opts, mapper, splits[idx], output.Send()).
				Wait(func(err error) {
					fun.WhenCall(errors.Is(err, io.EOF), wcancel)
				}).Add(wctx, wg)
		}

		// start a background op that waits for the
		// waitgroup and closes the channel
		wg.Operation().PostHook(wcancel).PostHook(output.Close).Go(ctx)
	}).Once()

	return output.Receive().Producer().PreHook(init).
		IteratorWithHook(erc.IteratorHook[O](ec))
}

var errAbortWorkers = errors.New("abort-workers")

func mapWorker[T any, O any](
	ec *erc.Collector,
	opts Options,
	mapper func(context.Context, T) (O, error),
	input *fun.Iterator[T],
	output fun.Send[O],
) fun.Worker {
	return func(ctx context.Context) error {
		proc := input.Producer()
		for {
			value, ok := proc.Check(ctx)
			if !ok {
				return nil
			}
			o, err := func(val T) (out O, err error) {
				defer func() { err = erc.Merge(err, fun.ParsePanic(recover())) }()

				out, err = mapper(ctx, val)
				return
			}(value)
			if err != nil {
				erc.When(ec, opts.shouldCollectError(err), err)

				hadPanic := errors.Is(err, fun.ErrRecoveredPanic)

				switch {
				case hadPanic && !opts.ContinueOnPanic:
					return io.EOF
				case hadPanic && opts.ContinueOnPanic:
					continue
				case !opts.ContinueOnError || erc.IsTerminating(err):
					return io.EOF
				case opts.ContinueOnError:
					continue
				}
			}

			if !output.Check(ctx, o) {
				return nil
			}
		}
	}
}
