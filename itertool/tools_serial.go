package itertool

import (
	"context"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/internal"
)

// ForEach passes each item in the iterator through the specified
// handler function, return an error if the handler function errors.
//
// ForEach aborts on the first error and converts any panic into an
// error which is propagated with other errors.
func ForEach[T any](ctx context.Context, iter fun.Iterator[T], fn func(T) error) (err error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	catcher := &erc.Collector{}

	defer func() { err = catcher.Resolve() }()
	defer erc.Recover(catcher)
	defer erc.Check(catcher, iter.Close)

	for iter.Next(ctx) {
		if ferr := fn(iter.Value()); ferr != nil {
			catcher.Add(ferr)
			break
		}
	}

	return
}

// Filter passes all objects in an iterator through the
// specified filter function. If the filter function errors, the
// operation aborts and the error is reported by the returned
// iterator's Close method. If the include boolean is true the result
// of the function is included in the output iterator, otherwise the
// operation is skipped.
//
// Filter is equivalent to Transform with the same input and output
// types. Filter operations are processed in a different thread, but
// are not cached or buffered: to process all options, you must
// consume the output iterator.
//
// The output iterator is produced iteratively as the returned
// iterator is consumed.
func Filter[T any](ctx context.Context, iter fun.Iterator[T], fn func(T) T) fun.Iterator[T] {
	return Transform(ctx, iter, fn)
}

// Transform processes the input iterator of type T into an output
// iterator of type O. This is the same as a Filter, but with
// different input and output types.
//
// While the transformations themselves are processed in a different
// go routine, the operations are not buffered or cached and the
// output iterator must be consumed to process all of the results. For
// concurrent processing, use the Map() operation.
func Transform[T any, O any](ctx context.Context, iter fun.Iterator[T], fn func(T) O) fun.Iterator[O] {
	out := new(internal.ChannelIterImpl[O])
	pipe := make(chan O)
	out.Pipe = pipe

	var iterCtx context.Context
	iterCtx, out.Closer = context.WithCancel(ctx)

	out.WG.Add(1)
	go func() {
		catcher := &erc.Collector{}
		defer out.WG.Done()
		defer func() { out.Error = catcher.Resolve() }()
		defer erc.Recover(catcher)
		defer erc.Check(catcher, iter.Close)
		defer close(pipe)

		for iter.Next(iterCtx) {
			select {
			case <-iterCtx.Done():
			case pipe <- fn(iter.Value()):
			}
		}
	}()

	return out
}

// ProcessWork provides a serial implementation of WorkerPool with
// similar semantics. These operations will abort on the first worker
// function to error.
func ProcessWork(ctx context.Context, iter fun.Iterator[fun.WorkerFunc]) error {
	return ForEach(ctx, iter, func(wf fun.WorkerFunc) error { return wf.Run(ctx) })
}

// Observe is a special case of ForEach to support observer pattern
// functions. Observe functions should be short running as they do not
// take a context, and could block unexpectedly.
//
// Unlike fun.Observe, itertool.Observe collects errors from the
// iterator's Close method and recovers from panics in the
// observer function, propagating them in the returned error.
func Observe[T any](ctx context.Context, iter fun.Iterator[T], obfn func(T)) error {
	return ForEach(ctx, iter, func(in T) error { obfn(in); return nil })
}

// ObserveWorker executes the worker functions from the iterator, and
// passes all errors through the observe function. If a worker panics,
// ObserveWorker will abort, convert the panic into an error, and pass
// that panic through the observe function.
func ObserveWorker(ctx context.Context, iter fun.Iterator[fun.WorkerFunc], ob func(error)) {
	// use ForEach rather than Observe to thread the context
	// through to the workers less indirectly.
	if err := ForEach(ctx, iter,
		func(wf fun.WorkerFunc) error { wf.Observe(ctx, ob); return nil },
	); err != nil {
		ob(err)
	}
}

// Reduce processes an input iterator with a reduce function and
// outputs the final value. The initial value may be a zero or nil
// value.
func Reduce[T any, O any](
	ctx context.Context,
	iter fun.Iterator[T],
	reducer func(T, O) (O, error),
	initalValue O,
) (value O, err error) {
	value = initalValue
	catcher := &erc.Collector{}

	defer func() { err = catcher.Resolve() }()
	defer erc.Recover(catcher)
	defer erc.Check(catcher, iter.Close)

	for iter.Next(ctx) {
		value, err = reducer(iter.Value(), value)
		if err != nil {
			catcher.Add(err)
			return
		}
	}

	return
}
