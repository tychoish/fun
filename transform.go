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

// Transform is a function type that converts T objects int objects of
// type O.
type Transform[T any, O any] func(context.Context, T) (O, error)

// Converter builds a Transform function out of an equivalent function
// that doesn't take a context or return an error.
func Converter[T any, O any](op func(T) O) Transform[T, O] {
	return func(_ context.Context, in T) (O, error) { return op(in), nil }
}

// ConverterOK builds a Transform function from a function that
// converts between types T and O, but that returns a boolean/check
// value. When false the transform function returns a ErrIteratorSkip
// error.
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

// ConverterErr constructs a Transform function from an analogous
// function that does not take a context.
func ConverterErr[T any, O any](op func(T) (O, error)) Transform[T, O] {
	return func(_ context.Context, in T) (O, error) { return op(in) }
}

// Convert takes an iterator and runs the transformer over every item,
// producing a new iterator with the output values. The processing
// operation respects ErrIteratorSkip.
func (mpf Transform[T, O]) Convert(iter *Iterator[T]) *Iterator[O] {
	return mpf.Producer(iter.ReadOne).Iterator()
}

// ProcessParallel runs the input iterator through the transform
// operation and produces an output iterator, much like
// convert. However, the ProcessParallel implementation has
// configurable parallelism, and error handling with the
// WorkerGroupConf options.
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

			mf.mapPullProcess(output.Send().Write, opts).
				ReadAll(splits[idx].Producer()).
				Operation(func(err error) {
					ft.WhenCall(ers.Is(err, io.EOF, ers.ErrAbortCurrentOp), wcancel)
				}).
				Add(wctx, wg)
		}

		// start a background op that waits for the
		// waitgroup and closes the channel
		wg.Operation().PostHook(wcancel).PostHook(output.Close).Background(ctx)
	}).Once()

	outputIter := output.Producer().PreHook(init).IteratorWithHook(func(out *Iterator[O]) { out.AddError(iter.Close()) })

	err := JoinOptionProviders(optp...).Apply(opts)
	ft.WhenCall(opts.ErrorHandler == nil, func() { opts.ErrorHandler = outputIter.ErrorHandler().Lock() })

	outputIter.AddError(err)

	ft.WhenCall(err != nil, output.Close)
	return outputIter
}

// Producer processes an input iterator with the Transform
// function. Each call to the producer returns one value from the
// input producer after processing the item with the transform
// function. The Producer returns any error encountered during these
// operations (input, transform, output) to its caller *except*
// ErrIteratorSkip, which is respected.
func (mpf Transform[T, O]) Producer(prod Producer[T]) Producer[O] {
	var zero O
	return Producer[O](func(ctx context.Context) (out O, _ error) {
		for {
			item, err := prod(ctx)
			if err == nil {
				out, err = mpf(ctx, item)
				if err == nil {
					return out, nil
				}
			}

			switch {
			case err == nil || errors.Is(err, ErrIteratorSkip):
				continue
			default:
				return zero, err
			}
		}
	})
}

// Pipe creates a Processor (input)/ Producer (output) pair that has
// data processed by the Transform function. The pipe has a buffer of
// one item and is never closed, and both input and output operations
// are blocking. The closer function will abort the connection and
// cause all successive operations to return io.EOF.
func (mpf Transform[T, O]) Pipe() (in Processor[T], out Producer[O], closer func()) {
	// components
	ch := make(chan T, 1)
	pipe := Blocking(ch)
	prod := pipe.Producer()

	// definitions
	closer = ft.Once(pipe.Close)
	in = pipe.Processor()
	out = func(ctx context.Context) (O, error) { return mpf(ctx, ft.Must(prod(ctx))) }

	return in, out, closer
}

// Block calls the transform function passing a context that cannot expire.
func (mpf Transform[T, O]) Block() func(T) (O, error) {
	return func(in T) (O, error) { return mpf(context.Background(), in) }
}

// BlockCheck calls the function with a context that cannot be
// canceled. The second value is true as long as the transform
// function returns a nil error and false in all other cases
func (mpf Transform[T, O]) BlockCheck() func(T) (O, bool) {
	mpfb := mpf.Block()
	return func(in T) (O, bool) {
		return ers.SafeOK(func() (O, error) { return mpfb(in) })
	}
}

// Lock returns a Transform function that's executed the root function
// inside of the sope of a mutex.
func (mpf Transform[T, O]) Lock() Transform[T, O] {
	mu := &sync.Mutex{}
	return mpf.WithLock(mu)
}

// WithLock returns a Transform function inside of the scope of the
// provided mutex.
func (mpf Transform[T, O]) WithLock(mu *sync.Mutex) Transform[T, O] {
	return func(ctx context.Context, val T) (O, error) {
		mu.Lock()
		defer mu.Unlock()
		return mpf(ctx, val)
	}
}

// Safe returns a Transform function that catches a panic, converts
// the panic object to an error if needed, and aggregates that with
// the Transform function's error.
func (mpf Transform[T, O]) Safe() Transform[T, O] {
	return func(ctx context.Context, val T) (_ O, err error) {
		defer func() { err = ers.Join(err, ers.ParsePanic(recover())) }()
		return mpf(ctx, val)
	}
}

// Worker collects a Produer and Processor pair, and returns a worker
// that processes the input collected by the  Processorand returns it
// to the Producer. This operation runs until the producer,
// transformer, or processor returns an error. ErrIteratorSkip errors
// are respected, while io.EOF errors cause the Worker to abort but
// are not propogated to the caller.
func (mpf Transform[T, O]) Worker(in Producer[T], out Processor[O]) Worker {
	return func(ctx context.Context) error {
		var (
			input  T
			output O
			err    error
		)

		for {
			input, err = in(ctx)
			if err == nil {
				output, err = mpf(ctx, input)
				if err == nil {
					err = out(ctx, output)
				}
			}

			switch {
			case err == nil || errors.Is(err, ErrIteratorSkip):
				continue
			case ers.Is(err, io.EOF, ers.ErrAbortCurrentOp):
				return nil
			default:
				return err
			}
		}

	}
}

// mapPullProcess returns a processor which consumes "input"  items, passes
// them to the transform function, and then "sends" them with the
// provided processor function.
func (mpf Transform[T, O]) mapPullProcess(
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
