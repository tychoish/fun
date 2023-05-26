package itertool

import (
	"context"
	"errors"
	"sync"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/internal"
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
	iter fun.Iterator[T],
	mapper func(context.Context, T) (O, error),
	opts Options,
) fun.Iterator[O] {
	if opts.NumWorkers <= 0 {
		opts.NumWorkers = 1
	}

	ec := &erc.Collector{}
	wg := &fun.WaitGroup{}
	out := &internal.GeneratorIterator[O]{}
	output := make(chan O)
	shutdown := &sync.Once{}
	out.Closer = func() {
		fun.WaitFunc(wg.Wait).Block()
		shutdown.Do(func() { close(output) })
	}

	setup := &sync.Once{}
	out.Operation = func(ctx context.Context) (O, error) {
		setup.Do(func() {
			input := collectChannel(ctx, opts.NumWorkers, iter)

			wctx, wcancel := context.WithCancel(ctx)
			for i := 0; i < opts.NumWorkers; i++ {
				worker := mapWorker(ec, opts, mapper, input, output)
				worker.Wait(func(err error) {
					if errors.Is(err, errAbortAllMapWorkers) {
						wcancel()
					}
				}).Add(wctx, wg)
			}
			go func() {
				out.Closer()
				wcancel()
			}()
		})

		out, err := fun.Blocking(output).Receive().Read(ctx)
		if err == nil {
			return out, nil
		}
		if ec.HasErrors() {
			return out, ec.Resolve()
		}
		return out, err
	}
	return Synchronize[O](out)
}

var errAbortAllMapWorkers = errors.New("abort-all-map-workers")

func mapWorker[T any, O any](
	catcher *erc.Collector,
	opts Options,
	mapper func(context.Context, T) (O, error),
	fromInput chan T,
	toOutput chan O,
) fun.WorkerFunc {
	return func(ctx context.Context) error {
	ITEM:
		for {
			value, ok := fun.Blocking(fromInput).Receive().Check(ctx)
			if !ok {
				return nil
			}
			o, err := func(val T) (out O, err error) {
				defer func() {
					if r := recover(); r != nil {
						err = internal.ParsePanic(r, fun.ErrRecoveredPanic)
					}
				}()

				out, err = mapper(ctx, val)
				return
			}(value)
			if err == nil {
				if err := fun.Blocking(toOutput).Send().Write(ctx, o); err != nil {
					return nil
				}
				continue ITEM
			}

			// handle errors
			if errors.Is(err, fun.ErrRecoveredPanic) {
				catcher.Add(err)
				if opts.ContinueOnPanic {
					continue ITEM
				}
				return errAbortAllMapWorkers
			}

			erc.When(catcher, opts.shouldCollectError(err), err)

			if opts.ContinueOnError {
				continue ITEM
			}

			return errAbortAllMapWorkers
		}
	}
}
