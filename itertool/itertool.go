// Package itertool provides a set of functional helpers for
// managinging and using fun.Iterators, including a parallel
// processing, generators, Map/Reduce, Merge, and other convenient
// tools.
package itertool

import (
	"context"
	"encoding/json"
	"errors"
	"io"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
)

// Process provides a (potentially) more sensible alternate name for
// ParallelForEach, but otherwise is identical.
func Process[T any](
	ctx context.Context,
	iter *fun.Iterator[T],
	fn fun.Processor[T],
	optp ...fun.OptionProvider[*fun.WorkerGroupConf],
) error {
	return ParallelForEach(ctx, iter, fn, optp...)
}

// compile-time assertions that both worker types support the "safe"
// interface needed for the Worker() tool.
var _ interface{ Safe() fun.Worker } = new(fun.Worker)
var _ interface{ Safe() fun.Worker } = new(fun.Operation)

// Worker takes iterators of fun.Worker or fun.Operation lambdas
// and processes them in according to the configuration.
//
// All operations functions are processed using their respective
// Safe() methods, which means that the functions themselves will
// never panic, and the ContinueOnPanic option will not impact the
// outcome of the operation (unless the iterator returns a nil
// operation.)
//
// This operation is particularly powerful in combination with the
// iterator for a pubsub.Distributor, interfaces which provide
// synchronized, blocking, and destructive (e.g. so completed
// workloads do not remain in memory) containers.
//
// Worker is implemented using ParallelForEach.
func Worker[OP fun.Worker | fun.Operation](
	ctx context.Context,
	iter *fun.Iterator[OP],
	optp ...fun.OptionProvider[*fun.WorkerGroupConf],
) error {
	return Process(ctx, iter, func(ctx context.Context, op OP) error {
		return any(op).(interface{ Safe() fun.Worker }).Safe()(ctx)
	}, optp...)
}

// ParallelForEach processes the iterator in parallel, and is
// essentially an iterator-driven worker pool. The input iterator is
// split dynamically into iterators for every worker (determined by
// Options.NumWorkers,) with the division between workers determined
// by their processing speed (e.g. workers should not suffer from
// head-of-line blocking,) and input iterators are consumed (safely)
// as work is processed.
func ParallelForEach[T any](
	ctx context.Context,
	iter *fun.Iterator[T],
	fn fun.Processor[T],
	optp ...fun.OptionProvider[*fun.WorkerGroupConf],
) error {
	return iter.ProcessParallel(fn, append(optp, fun.WorkerGroupConfWithErrorCollector(&erc.Collector{}))...).Run(ctx)
}

// Generate creates an iterator using a generator pattern which
// produces items until the generator function returns
// io.EOF, or the context (passed to the first call to
// Next()) is canceled. Parallel operation, and continue on
// error/continue-on-panic semantics are available and share
// configuration with the Map and ParallelForEach operations.
func Generate[T any](
	fn fun.Producer[T],
	optp ...fun.OptionProvider[*fun.WorkerGroupConf],
) *fun.Iterator[T] {
	return fn.GenerateParallel(optp...)
}

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
	input *fun.Iterator[T],
	mapFn func(context.Context, T) (O, error),
	optp ...fun.OptionProvider[*fun.WorkerGroupConf],
) *fun.Iterator[O] {
	return fun.Map(input, mapFn, optp...)
}

// MapReduce combines the map and reduce operations to process an
// iterator (in parallel, according to configuration) into an output
// iterator, and then process that iterator with the reduce function.
//
// MapReduce itself returns a fun.Producer function, which functions
// as a future, and the entire operation, does not begin running until
// the producer function is called, and the
//
// This works as a pull: the Reduce operation starts and
// waits for the map operation to produce a value, the map operation
// waits for the input iterator to produce values.
func MapReduce[T any, O any, R any](
	input *fun.Iterator[T],
	mapFn func(context.Context, T) (O, error),
	reduceFn func(O, R) (R, error),
	initialReduceValue R,
	optp ...fun.OptionProvider[*fun.WorkerGroupConf],
) fun.Producer[R] {
	return func(ctx context.Context) (R, error) {
		return Reduce(ctx, Map(input, mapFn, optp...), reduceFn, initialReduceValue)
	}
}

// Reduce processes an input iterator with a reduce function and
// outputs the final value. The initial value may be a zero or nil
// value.
func Reduce[T any, O any](
	ctx context.Context,
	iter *fun.Iterator[T],
	reducer func(T, O) (O, error),
	initalValue O,
) (value O, err error) {
	defer func() { err = ers.Join(err, ers.ParsePanic(recover())) }()
	value = initalValue
	for {
		item, err := iter.ReadOne(ctx)
		if err != nil {
			return value, nil
		}

		out, err := reducer(item, value)
		switch {
		case err == nil:
			value = out
			continue
		case errors.Is(err, fun.ErrIteratorSkip):
			continue
		case errors.Is(err, io.EOF):
			return value, nil
		default:
			return value, err
		}
	}
}

// Contains processes an iterator of compareable type returning true
// after the first element that equals item, and false otherwise.
func Contains[T comparable](ctx context.Context, item T, iter *fun.Iterator[T]) bool {
	for {
		v, err := iter.ReadOne(ctx)
		if err != nil {
			break
		}
		if v == item {
			return true
		}
	}

	return false
}

// Uniq iterates over an iterator of comparable items, and caches them
// in a map, returning the first instance of each equivalent object,
// and skipping subsequent items
func Uniq[T comparable](iter *fun.Iterator[T]) *fun.Iterator[T] {
	set := dt.Mapify(map[T]struct{}{})

	return fun.Producer[T](func(ctx context.Context) (out T, _ error) {
		for iter.Next(ctx) {
			if val := iter.Value(); !set.Check(val) {
				set.SetDefault(val)
				return val, nil
			}
		}
		return out, io.EOF
	}).Iterator()
}

// DropZeroValues processes an iterator removing all zero values.
func DropZeroValues[T comparable](iter *fun.Iterator[T]) *fun.Iterator[T] {
	return fun.Producer[T](func(ctx context.Context) (out T, _ error) {
		for {
			item, err := iter.ReadOne(ctx)
			if err != nil {
				return out, err
			}

			if !ft.IsZero(item) {
				return item, nil
			}
		}
	}).Iterator()
}

// Chain, like merge, takes a sequence of iterators and produces a
// combined iterator. Chain processes each iterator provided in
// sequence, where merge reads from all iterators in at once.
func Chain[T any](iters ...*fun.Iterator[T]) *fun.Iterator[T] {
	pipe := fun.Blocking(make(chan T))

	init := fun.Operation(func(ctx context.Context) {
		defer pipe.Close()
		iteriter := dt.Sliceify(iters).Iterator()

		// use direct iteration because this function has full
		// ownership of the iterator and this is the easiest
		// way to make sure that the outer iterator aborts
		// when the context is canceled.
		for iteriter.Next(ctx) {
			iter := iteriter.Value()

			// iterate using the helper, so we get more
			// atomic iteration, and the ability to
			// respond to cancellation/blocking from the
			// outgoing function.
			for {
				val, err := iter.ReadOne(ctx)
				if err == nil {
					if pipe.Send().Check(ctx, val) {
						continue
					}
				}
				break
			}
		}
	}).Launch().Once()

	return pipe.Receive().Producer().PreHook(init).Iterator()
}

// Monotonic creates an iterator that produces increasing numbers
// until a specified maximum.
func Monotonic(max int) *fun.Iterator[int] { return fun.HF.Counter(max) }

// JSON takes a stream of line-oriented JSON and marshals those
// documents into objects in the form of an iterator.
func JSON[T any](in io.Reader) *fun.Iterator[T] {
	var zero T
	return fun.ConvertIterator(fun.HF.Lines(in), fun.ConverterErr(func(in string) (out T, err error) {
		defer func() { err = ers.Join(err, ers.ParsePanic(recover())) }()
		if err = json.Unmarshal([]byte(in), &out); err != nil {
			return zero, err
		}
		return out, err
	}))
}
