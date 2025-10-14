package itertool

import (
	"context"
	"errors"
	"io"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/fnx"
)

// Map provides an orthodox functional map implementation based around
// fun.Stream. Operates in asynchronous/streaming manner, so that
// the output Stream must be consumed. The zero values of Options
// provide reasonable defaults for abort-on-error and single-threaded
// map operation.
//
// If the mapper function errors, the result isn't included, but the
// errors would be aggregated and propagated to the `Close()` method
// of the resulting stream. The mapping operation respects the
// fun.ErrIterationSkip error, If there are more than one error (as is
// the case with a panic or with ContinueOnError semantics,) the error
// can be unwrapped or converted to a slice with the ers.Unwind
// function. Panics in the map function are converted to errors and
// always collected but may abort the operation if ContinueOnPanic is
// not set.
func Map[T any, O any](
	input *fun.Stream[T],
	mapFn fnx.Converter[T, O],
	optp ...fun.OptionProvider[*fun.WorkerGroupConf],
) *fun.Stream[O] {
	return fun.Convert(mapFn).Parallel(input, optp...)
}

// MapReduce combines the map and reduce operations to process an
// stream (in parallel, according to configuration) into an output
// stream, and then process that stream with the reduce function.
//
// MapReduce itself returns a fun.Future function, which functions
// as a future, and the entire operation, does not begin running until
// the future is called.
//
// This works as a pull: the Reduce operation starts and
// waits for the map operation to produce a value, the map operation
// waits for the input stream to produce values.
func MapReduce[T any, O any, R any](
	input *fun.Stream[T],
	mapFn fnx.Converter[T, O],
	reduceFn func(O, R) (R, error),
	initialReduceValue R,
	optp ...fun.OptionProvider[*fun.WorkerGroupConf],
) fnx.Future[R] {
	return Reduce(Map(input, mapFn, optp...), reduceFn, initialReduceValue)
}

// Reduce processes an input stream with a reduce function and
// outputs the final value. The initial value may be a zero or nil
// value.
func Reduce[T any, O any](
	iter *fun.Stream[T],
	reduceFn func(T, O) (O, error),
	initialReduceValue O,
) fnx.Future[O] {
	// TODO: add emitter function

	return func(ctx context.Context) (value O, err error) {
		defer func() { err = erc.Join(err, erc.ParsePanic(recover())) }()
		value = initialReduceValue
		for {
			item, err := iter.Read(ctx)
			if err != nil {
				return value, nil
			}

			out, err := reduceFn(item, value)
			switch {
			case err == nil:
				value = out
				continue
			case errors.Is(err, ers.ErrCurrentOpSkip):
				continue
			case errors.Is(err, io.EOF):
				return value, nil
			default:
				return value, err
			}
		}
	}
}
