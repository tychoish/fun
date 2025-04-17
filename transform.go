package fun

import (
	"context"
	"errors"
	"io"
	"sync"

	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/internal"
)

// Converter is a function type that converts T objects int objects of
// type O.
type Converter[T any, O any] func(context.Context, T) (O, error)

// MakeConverter builds a Transform function out of an equivalent function
// that doesn't take a context or return an error.
func MakeConverter[T any, O any](op func(T) O) Converter[T, O] {
	return func(_ context.Context, in T) (O, error) { return op(in), nil }
}

// MakeCovnerterOk builds a Transform function from a function that
// converts between types T and O, but that returns a boolean/check
// value. When the converter function returns false the
// transform function returns a ErrStreamContinue error.
func MakeCovnerterOk[T any, O any](op func(T) (O, bool)) Converter[T, O] {
	return func(_ context.Context, in T) (out O, err error) {
		var ok bool
		out, ok = op(in)
		if !ok {
			err = ErrStreamContinue
		}

		return
	}
}

// MakeConverterErr constructs a Transform function from an analogous
// function that does not take a context.
func MakeConverterErr[T any, O any](op func(T) (O, error)) Converter[T, O] {
	return func(_ context.Context, in T) (O, error) { return op(in) }
}

// Lock returns a Transform function that's executed the root function
// inside of the sope of a mutex.
func (mpf Converter[T, O]) Lock() Converter[T, O] {
	mu := &sync.Mutex{}
	return mpf.WithLock(mu)
}

// WithLock returns a Transform function inside of the scope of the
// provided mutex.
func (mpf Converter[T, O]) WithLock(mu *sync.Mutex) Converter[T, O] {
	return func(ctx context.Context, val T) (O, error) {
		defer internal.With(internal.Lock(mu))
		return mpf.Convert(ctx, val)
	}
}

// WithRecover returns a Transform function that catches a panic, converts
// the panic object to an error if needed, and aggregates that with
// the Transform function's error.
func (mpf Converter[T, O]) WithRecover() Converter[T, O] {
	return func(ctx context.Context, val T) (_ O, err error) {
		defer func() { err = ers.Join(err, ers.ParsePanic(recover())) }()
		return mpf(ctx, val)
	}
}

// Wait calls the transform function passing a context that cannot expire.
func (mpf Converter[T, O]) Wait(in T) (O, error) { return mpf.Convert(context.Background(), in) }

// Convert uses the converter function to transform a value from one
// type (T) to another (O).
func (mpf Converter[T, O]) Convert(ctx context.Context, in T) (O, error) { return mpf(ctx, in) }

// Generator processes an input generator function with the Transform
// function. Each call to the output generator returns one value from
// the input generator after processing the item with the transform
// function applied. The output generator returns any error encountered
// during these operations (input, transform, output) to its caller
// *except* ErrStreamContinue, which is respected.
func (mpf Converter[T, O]) Generator(prod Generator[T]) Generator[O] {
	return func(ctx context.Context) (out O, _ error) {
		for {
			item, err := prod(ctx)
			if err == nil {
				out, err = mpf.Convert(ctx, item)
				if err == nil {
					return out, nil
				}
			}

			switch {
			case errors.Is(err, ErrStreamContinue):
				continue
			default:
				return mpf.zeroOut(), err
			}
		}
	}
}

// Stream takes an input stream of one type and converts it to a
// stream of the another type. All errors from the original stream are
// propagated to the output stream.
func (mpf Converter[T, O]) Stream(iter *Stream[T]) *Stream[O] {
	return mpf.Generator(iter.Read).
		Stream().
		WithHook(func(st *Stream[O]) { st.AddError(iter.Close()) })
}

// Parallel runs the input stream through the transform
// operation and produces an output stream, much like
// convert. However, the Parallel implementation has
// configurable parallelism, and error handling with the
// WorkerGroupConf options.
func (mpf Converter[T, O]) Parallel(
	iter *Stream[T],
	opts ...OptionProvider[*WorkerGroupConf],
) *Stream[O] {
	conf := &WorkerGroupConf{}
	if err := JoinOptionProviders(opts...).Apply(conf); err != nil {
		return makeErrorGenerator[O](err).Stream()
	}

	output := Blocking(make(chan O))

	// this operations starts the background thread for the
	// mapper/stream, but doesn't run until the first
	// iteration, so opts doesn't have to be populated/applied
	// till later, and error handling gets much easier if we
	// wait.
	setup := mpf.WithRecover().
		mapPullProcess(output.Handler(), conf).
		ReadAll(iter).
		Group(conf.NumWorkers).
		PostHook(output.Close).
		Operation(conf.ErrorHandler).
		Go().
		Once()

	return output.Generator().
		PreHook(setup).
		Stream().
		WithHook(func(out *Stream[O]) {
			out.AddError(iter.Close())
			out.AddError(conf.ErrorResolver())
		})
}

func (Converter[T, O]) zeroOut() (out O) { return }

// mapPullProcess returns a processor which consumes "input"  items, passes
// them to the transform function, and then "sends" them with the
// provided processor function.
func (mpf Converter[T, O]) mapPullProcess(
	output Handler[O],
	opts *WorkerGroupConf,
) Handler[T] {
	return func(ctx context.Context, in T) error {
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
	}
}
