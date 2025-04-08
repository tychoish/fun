package itertool

import (
	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
)

// Chain, like merge, takes a sequence of streams and produces a
// combined stream.
//
// Chain is an alias for fun.JoinStreams
func Chain[T any](iters ...*fun.Stream[T]) *fun.Stream[T] { return fun.JoinStreams(iters...) }

// Merge, takes a sequence of streams and produces a combined
// stream. The input streams are processed in parallel and objects
// are emitted in an arbitrary order.
//
// Merge is an alias for fun.MergeStreams
func Merge[T any](iters *fun.Stream[*fun.Stream[T]]) *fun.Stream[T] { return fun.MergeStreams(iters) }

// Flatten converts a stream of streams to an flattened stream
// of their elements.
//
// Flatten is an alias for fun.FlattenStreams.
func Flatten[T any](iter *fun.Stream[*fun.Stream[T]]) *fun.Stream[T] { return fun.FlattenStreams(iter) }

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

// ParallelForEach processes the stream in parallel, and is
// essentially a stream-driven worker pool. The input stream is
// split dynamically into streams for every worker (determined by
// Options.NumWorkers,) with the division between workers determined
// by their processing speed (e.g. workers should not suffer from
// head-of-line blocking,) and input streams are consumed (safely)
// as work is processed.
func ParallelForEach[T any](
	iter *fun.Stream[T],
	fn fun.Handler[T],
	optp ...fun.OptionProvider[*fun.WorkerGroupConf],
) fun.Worker {
	return iter.ProcessParallel(fn, append(optp, fun.WorkerGroupConfWithErrorCollector(&erc.Collector{}))...)
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
	return fn.Parallel(optp...).Stream()
}

// Process provides a (potentially) more sensible alternate name for
// ParallelForEach, but otherwise is identical.
func Process[T any](
	iter *fun.Stream[T],
	fn fun.Handler[T],
	optp ...fun.OptionProvider[*fun.WorkerGroupConf],
) fun.Worker {
	return ParallelForEach(iter, fn, optp...)
}
