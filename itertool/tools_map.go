package itertool

import (
	"context"
	"errors"
	"io"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
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
	mapFn func(context.Context, T) (O, error),
	optp ...OptionProvider[*Options],
) *fun.Iterator[O] {

	opts := Options{}
	Apply(&opts, optp...)
	opts.init()

	ec := &erc.Collector{}
	output := fun.Blocking(make(chan O))
	var mf mapper[T, O] = mapFn

	init := fun.Operation(func(ctx context.Context) {
		splits := iter.Split(opts.NumWorkers)

		wctx, wcancel := context.WithCancel(ctx)
		wg := &fun.WaitGroup{}

		for idx := range splits {
			// for each split, run a mapWorker

			mf.Safe().Processor(output.Send().Write, ec.Add, opts).
				ReadAll(splits[idx].Producer()).
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

type mapper[T any, O any] func(context.Context, T) (O, error)

func (mf mapper[T, O]) Safe() mapper[T, O] {
	return func(ctx context.Context, val T) (_ O, err error) {
		defer func() { err = erc.Merge(err, ers.ParsePanic(recover())) }()
		return mf(ctx, val)
	}
}

func (mf mapper[T, O]) Processor(
	output fun.Processor[O],
	ec fun.Observer[error],
	opts Options,
) fun.Processor[T] {
	return func(ctx context.Context, in T) error {
		val, err := mf(ctx, in)
		if err != nil {
			if opts.HandleAbortableErrors(ec, err) {
				return nil
			}
			return io.EOF
		}

		if !output.Check(ctx, val) {
			return io.EOF
		}

		return nil
	}
}
