package fun

import (
	"context"
	"errors"
	"io"
	"sync"

	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
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
	mpf Transform[T, O],
	optp ...OptionProvider[*WorkerGroupConf],
) *Iterator[O] {
	return mpf.ProcessParallel(input, optp...)
}

type Transform[T any, O any] func(context.Context, T) (O, error)

func Converter[T any, O any](op func(T) O) Transform[T, O] {
	return func(_ context.Context, in T) (O, error) { return op(in), nil }
}
func ConverterOK[T any, O any](op func(T) (O, bool)) Transform[T, O] {
	return func(ctx context.Context, in T) (out O, err error) {
		var ok bool
		out, ok = op(in)
		if !ok {
			err = ErrIteratorSkip
		}

		return
	}
}

func ConverterErr[T any, O any](op func(T) (O, error)) Transform[T, O] {
	return func(ctx context.Context, in T) (O, error) { return op(in) }
}

func (mpf Transform[T, O]) Convert(iter *Iterator[T]) *Iterator[O] {
	return mpf.Producer(iter).Iterator()
}

func (mpf Transform[T, O]) ProcessParallel(
	iter *Iterator[T],
	optp ...OptionProvider[*WorkerGroupConf],
) *Iterator[O] {
	output := Blocking(make(chan O))
	opts := &WorkerGroupConf{}

	// this operations starts the background thread for the
	// mapper/iterator, but doesn't run until the first
	// iteration, so opts doesn't have to be populated/applied
	// till later, and error handling gets much easier if we
	// wait.
	init := Operation(func(ctx context.Context) {
		wctx, wcancel := context.WithCancel(ctx)
		wg := &WaitGroup{}
		mf := mpf.Safe()
		splits := iter.Split(opts.NumWorkers)
		for idx := range splits {
			// for each split, run a mapWorker

			mf.Processor(output.Send().Write, opts).
				ReadAll(splits[idx].Producer()).
				Operation(func(err error) {
					ft.WhenCall(errors.Is(err, io.EOF), wcancel)
				}).
				Add(wctx, wg)
		}

		// start a background op that waits for the
		// waitgroup and closes the channel
		wg.Operation().PostHook(wcancel).PostHook(output.Close).Go(ctx)
	}).Once()

	outputIter := output.Producer().PreHook(init).Iterator()

	err := JoinOptionProviders(optp...).Apply(opts)
	ft.WhenCall(opts.ErrorObserver == nil, func() { opts.ErrorObserver = outputIter.ErrorObserver().Lock() })
	outputIter.AddError(err)

	ft.WhenCall(err != nil, output.Close)
	return outputIter
}

func (mpf Transform[T, O]) Producer(iter *Iterator[T]) Producer[O] {
	var zero O
	return Producer[O](func(ctx context.Context) (out O, _ error) {
		for {
			if item, err := iter.ReadOne(ctx); err == nil {
				if out, err = mpf(ctx, item); err == nil {
					return out, nil
				} else if errors.Is(err, ErrIteratorSkip) {
					continue
				}

				return zero, err
			} else if !errors.Is(err, ErrIteratorSkip) {
				return zero, err
			}
		}
	})
}

func (mpf Transform[T, O]) Pipe() (in Processor[T], out Producer[O]) {
	pipe := Blocking(make(chan T, 1))
	prod := pipe.Producer()
	return pipe.Processor(), func(ctx context.Context) (O, error) {
		return mpf(ctx, ft.Must(prod(ctx)))
	}
}

func (mpf Transform[T, O]) Block() func(T) (O, error) {
	return func(in T) (O, error) { return mpf(context.Background(), in) }
}

func (mpf Transform[T, O]) BlockCheck() func(T) (O, bool) {
	mpfb := mpf.Block()
	return func(in T) (O, bool) {
		return ers.SafeOK(func() (O, error) { return mpfb(in) })
	}
}

func (mpf Transform[T, O]) Lock() Transform[T, O] {
	mu := &sync.Mutex{}
	return mpf.WithLock(mu)
}

func (mpf Transform[T, O]) WithLock(mu *sync.Mutex) Transform[T, O] {
	return func(ctx context.Context, val T) (O, error) {
		mu.Lock()
		defer mu.Unlock()
		return mpf(ctx, val)
	}
}

func (mpf Transform[T, O]) Safe() Transform[T, O] {
	return func(ctx context.Context, val T) (_ O, err error) {
		defer func() { err = ers.Join(err, ers.ParsePanic(recover())) }()
		return mpf(ctx, val)
	}
}

func (mpf Transform[T, O]) Worker(in Producer[T], out Processor[O]) Worker {
	return func(ctx context.Context) error {
		first, err := in(ctx)
		if err != nil {
			return err
		}
		second, err := mpf(ctx, first)
		if err != nil {
			return err
		}
		return out(ctx, second)
	}
}

func (mpf Transform[T, O]) Processor(
	output Processor[O],
	opts *WorkerGroupConf,
) Processor[T] {
	return Processor[T](func(ctx context.Context, in T) error {
		val, err := mpf(ctx, in)
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
	})
}
