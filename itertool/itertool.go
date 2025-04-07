// Package itertool provides a set of functional helpers for
// managinging and using fun.Streams, including a parallel
// processing, generators, Map/Reduce, Merge, and other convenient
// tools.
package itertool

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync/atomic"
	"time"

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
	iter *fun.Stream[T],
	fn fun.Processor[T],
	optp ...fun.OptionProvider[*fun.WorkerGroupConf],
) error {
	return ParallelForEach(ctx, iter, fn, optp...)
}

// compile-time assertions that both worker types support the "safe"
// interface needed for the Worker() tool.
var _ interface{ WithRecover() fun.Worker } = new(fun.Worker)
var _ interface{ WithRecover() fun.Worker } = new(fun.Operation)

// Worker takes streams of fun.Worker or fun.Operation lambdas
// and processes them in according to the configuration.
//
// All operations functions are processed using their respective
// Safe() methods, which means that the functions themselves will
// never panic, and the ContinueOnPanic option will not impact the
// outcome of the operation (unless the stream returns a nil
// operation.)
//
// This operation is particularly powerful in combination with the
// stream for a pubsub.Distributor, interfaces which provide
// synchronized, blocking, and destructive (e.g. so completed
// workloads do not remain in memory) containers.
//
// Worker is implemented using ParallelForEach.
func Worker[OP fun.Worker | fun.Operation](
	ctx context.Context,
	iter *fun.Stream[OP],
	optp ...fun.OptionProvider[*fun.WorkerGroupConf],
) error {
	return Process(ctx, iter, func(ctx context.Context, op OP) error {
		return any(op).(interface{ WithRecover() fun.Worker }).WithRecover().Run(ctx)
	}, optp...)
}

// ParallelForEach processes the stream in parallel, and is
// essentially a stream-driven worker pool. The input stream is
// split dynamically into streams for every worker (determined by
// Options.NumWorkers,) with the division between workers determined
// by their processing speed (e.g. workers should not suffer from
// head-of-line blocking,) and input streams are consumed (safely)
// as work is processed.
func ParallelForEach[T any](
	ctx context.Context,
	iter *fun.Stream[T],
	fn fun.Processor[T],
	optp ...fun.OptionProvider[*fun.WorkerGroupConf],
) error {
	return iter.ProcessParallel(fn, append(optp, fun.WorkerGroupConfWithErrorCollector(&erc.Collector{}))...).Run(ctx)
}

// Generate creates a stream using a generator pattern which
// produces items until the generator function returns
// io.EOF, or the context (passed to the first call to
// Next()) is canceled. Parallel operation, and continue on
// error/continue-on-panic semantics are available and share
// configuration with the Map and ParallelForEach operations.
func Generate[T any](
	fn fun.Generator[T],
	optp ...fun.OptionProvider[*fun.WorkerGroupConf],
) *fun.Stream[T] {
	return fn.GenerateParallel(optp...)
}

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
	mapFn fun.Transform[T, O],
	optp ...fun.OptionProvider[*fun.WorkerGroupConf],
) *fun.Stream[O] {
	return fun.Map(input, mapFn, optp...)
}

// MapReduce combines the map and reduce operations to process an
// stream (in parallel, according to configuration) into an output
// stream, and then process that stream with the reduce function.
//
// MapReduce itself returns a fun.Producer function, which functions
// as a future, and the entire operation, does not begin running until
// the producer function is called, and the
//
// This works as a pull: the Reduce operation starts and
// waits for the map operation to produce a value, the map operation
// waits for the input stream to produce values.
func MapReduce[T any, O any, R any](
	input *fun.Stream[T],
	mapFn fun.Transform[T, O],
	reduceFn func(O, R) (R, error),
	initialReduceValue R,
	optp ...fun.OptionProvider[*fun.WorkerGroupConf],
) fun.Generator[R] {
	return func(ctx context.Context) (R, error) {
		return Reduce(ctx, Map(input, mapFn, optp...), reduceFn, initialReduceValue)
	}
}

// Reduce processes an input stream with a reduce function and
// outputs the final value. The initial value may be a zero or nil
// value.
func Reduce[T any, O any](
	ctx context.Context,
	iter *fun.Stream[T],
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
		case errors.Is(err, fun.ErrStreamContinue):
			continue
		case errors.Is(err, io.EOF):
			return value, nil
		default:
			return value, err
		}
	}
}

// Contains processes a stream of compareable type returning true
// after the first element that equals item, and false otherwise.
func Contains[T comparable](ctx context.Context, item T, iter *fun.Stream[T]) bool {
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

// Uniq iterates over a stream of comparable items, and caches them
// in a map, returning the first instance of each equivalent object,
// and skipping subsequent items
func Uniq[T comparable](iter *fun.Stream[T]) *fun.Stream[T] {
	set := dt.NewMap(map[T]struct{}{})

	return fun.Generator[T](func(ctx context.Context) (out T, _ error) {
		for iter.Next(ctx) {
			if val := iter.Value(); !set.Check(val) {
				set.SetDefault(val)
				return val, nil
			}
		}
		return out, io.EOF
	}).Stream().WithHook(func(out *fun.Stream[T]) {
		out.AddError(iter.Close())
	})
}

// DropZeroValues processes a stream removing all zero values.
func DropZeroValues[T comparable](iter *fun.Stream[T]) *fun.Stream[T] {
	return fun.Generator[T](func(ctx context.Context) (out T, _ error) {
		for {
			item, err := iter.ReadOne(ctx)
			if err != nil {
				return out, err
			}

			if !ft.IsZero(item) {
				return item, nil
			}
		}
	}).Stream().WithHook(func(stream *fun.Stream[T]) {
		stream.AddError(iter.Close())
	})
}

// Chain, like merge, takes a sequence of streams and produces a
// combined stream.
//
// Chain is an alias for fun.JoinStreams
func Chain[T any](iters ...*fun.Stream[T]) *fun.Stream[T] {
	return fun.JoinStreams(iters...)
}

// Merge, takes a sequence of streams and produces a combined
// stream. The input streams are processed in parallel and objects
// are emitted in an arbitrary order.
//
// Merge is an alias for fun.MergeStreams
func Merge[T any](iters *fun.Stream[*fun.Stream[T]]) *fun.Stream[T] {
	return fun.MergeStreams(iters)
}

// Flatten converts a stream of streams to an flattened stream
// of their elements.
//
// Flatten is an alias for fun.FlattenStreams.
func Flatten[T any](iter *fun.Stream[*fun.Stream[T]]) *fun.Stream[T] {
	return fun.FlattenStreams(iter)
}

// FlattenSlices converts a stream of slices to an flattened
// stream of their elements.
func FlattenSlices[T any](iter *fun.Stream[[]T]) *fun.Stream[T] {
	return Flatten(fun.Converter(fun.SliceStream[T]).Process(iter))
}

// MergeSlices converts a stream of slices to an flattened
// stream of their elements.
func MergeSlices[T any](iter *fun.Stream[[]T]) *fun.Stream[T] {
	return Merge(fun.Converter(fun.SliceStream[T]).Process(iter))
}

// Monotonic creates a stream that produces increasing numbers
// until a specified maximum.
func Monotonic(maxVal int) *fun.Stream[int] { return fun.MAKE.Counter(maxVal) }

// JSON takes a stream of line-oriented JSON and marshals those
// documents into objects in the form of a stream.
func JSON[T any](in io.Reader) *fun.Stream[T] {
	var zero T
	return fun.ConvertStream(fun.MAKE.LinesWithSpaceTrimed(in), fun.ConverterErr(func(in string) (out T, err error) {
		defer func() { err = ers.Join(err, ers.ParsePanic(recover())) }()
		if err = json.Unmarshal([]byte(in), &out); err != nil {
			return zero, err
		}
		return out, err
	}))
}

// Indexed produces a stream that keeps track of and reports the
// sequence/index id of the item in the iteration sequence.
func Indexed[T any](iter *fun.Stream[T]) *fun.Stream[dt.Pair[int, T]] {
	idx := &atomic.Int64{}
	idx.Store(-1)
	return fun.ConvertStream(iter, fun.Converter(func(in T) dt.Pair[int, T] { return dt.MakePair(int(idx.Add(1)), in) }))
}

// RateLimit wraps a stream with a rate-limiter to ensure that the
// output stream will produce no more than <num> items in any given
// <window>.
func RateLimit[T any](iter *fun.Stream[T], num int, window time.Duration) *fun.Stream[T] {
	queue := &dt.List[time.Time]{}
	timer := time.NewTimer(0)
	var zero T

	fun.Invariant.IsTrue(num > 0, "rate must be greater than zero")

	return fun.NewGenerator(func(ctx context.Context) (T, error) {

	START:
		now := time.Now()
		if queue.Len() < num {
			queue.PushBack(now)
			return iter.ReadOne(ctx)
		}
		for queue.Len() > 0 && now.After(queue.Front().Value().Add(window)) {
			queue.Front().Drop()
		}
		if queue.Len() < num {
			queue.PushBack(now)
			return iter.ReadOne(ctx)
		}

		sleepUntil := time.Until(queue.Front().Value().Add(window))
		fun.Invariant.IsTrue(sleepUntil >= 0, "the next sleep must be in the future")

		timer.Reset(sleepUntil)
		select {
		case <-timer.C:
			goto START
		case <-ctx.Done():
			return zero, ctx.Err()
		}

	}).Lock().Stream()
}
