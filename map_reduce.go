package fun

import (
	"context"
	"errors"
	"io"

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
// of the resulting iterator. The mapping operation respects the
// fun.ErrIterationSkip error, If there are more than one error (as is
// the case with a panic or with ContinueOnError semantics,) the error
// can be unwrapped or converted to a slice with the fun.Unwind
// function. Panics in the map function are converted to errors and
// always collected but may abort the operation if ContinueOnPanic is
// not set.
func Map[T any, O any](
	input *Iterator[T],
	mapFn func(context.Context, T) (O, error),
	optp ...OptionProvider[*WorkerGroupOptions],
) *Iterator[O] {
	output := Blocking(make(chan O))
	opts := &WorkerGroupOptions{}

	// this operations starts the background thread for the
	// mapper/iterator, but doesn't run until the first
	// iteration, so opts doesn't have to be populated/applied
	// till later, and error handling gets much easier if we
	// wait.
	init := Operation(func(ctx context.Context) {
		wctx, wcancel := context.WithCancel(ctx)
		wg := &WaitGroup{}
		mf := mapper[T, O](mapFn).Safe()

		splits := input.Split(opts.NumWorkers)
		for idx := range splits {
			// for each split, run a mapWorker

			mf.Processor(output.Send().Write, opts).
				ReadAll(splits[idx].Producer()).
				Wait(func(err error) {
					WhenCall(errors.Is(err, io.EOF), wcancel)
				}).
				Add(wctx, wg)
		}

		// start a background op that waits for the
		// waitgroup and closes the channel
		wg.Operation().PostHook(wcancel).PostHook(output.Close).Go(ctx)
	}).Once()

	outputIter := output.Receive().Producer().PreHook(init).Iterator()
	err := ApplyOptions(opts, optp...)
	WhenCall(opts.ErrorObserver == nil, func() { opts.ErrorObserver = outputIter.ErrorObserver().Lock() })

	outputIter.AddError(err)
	WhenCall(err != nil, output.Close)

	return outputIter
}

type mapper[T any, O any] func(context.Context, T) (O, error)

func (mf mapper[T, O]) Safe() mapper[T, O] {
	return func(ctx context.Context, val T) (_ O, err error) {
		defer func() { err = ers.Merge(err, ers.ParsePanic(recover())) }()
		return mf(ctx, val)
	}
}

func (mf mapper[T, O]) Processor(
	output Processor[O],
	opts *WorkerGroupOptions,
) Processor[T] {
	return func(ctx context.Context, in T) error {
		val, err := mf(ctx, in)
		if err != nil {
			if opts.CanContinueOnError(err) {
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

func (i *Iterator[T]) Reduce(
	reducer func(T, T) (T, error),
	initalValue T,
) Producer[T] {
	var value T = initalValue
	return func(ctx context.Context) (_ T, err error) {
		defer func() { err = ers.Merge(err, ers.ParsePanic(recover())) }()

		for {
			item, err := i.ReadOne(ctx)
			if err != nil {
				return value, nil
			}

			out, err := reducer(item, value)
			switch {
			case err == nil:
				value = out
				continue
			case errors.Is(err, ErrIteratorSkip):
				continue
			case errors.Is(err, io.EOF):
				return value, nil
			default:
				return value, err
			}
		}
	}
}
