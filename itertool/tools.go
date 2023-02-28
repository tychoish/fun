package itertool

import (
	"context"
	"errors"
	"sync"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/internal"
)

// CollectChannel converts and iterator to a channel. The iterator is
// not closed.
func CollectChannel[T any](ctx context.Context, iter fun.Iterator[T]) <-chan T {
	out := make(chan T)
	go func() {
		defer close(out)
		for iter.Next(ctx) {
			select {
			case <-ctx.Done():
				return
			case out <- iter.Value():
				continue
			}
		}
	}()
	return out
}

// CollectSlice converts an iterator to the slice of it's values, and
// closes the iterator at the when the iterator has been exhausted..
//
// In the case of an error in the underlying iterator the output slice
// will have the values encountered before the error.
func CollectSlice[T any](ctx context.Context, iter fun.Iterator[T]) ([]T, error) {
	out := []T{}
	for iter.Next(ctx) {
		out = append(out, iter.Value())
	}

	return out, iter.Close()
}

// ForEach passes each item in the iterator through the specified
// handler function, return an error if the handler function errors.
func ForEach[T any](
	ctx context.Context,
	iter fun.Iterator[T],
	fn func(context.Context, T) error,
) (err error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	catcher := &erc.Collector{}

	defer func() { err = catcher.Resolve() }()
	defer erc.Recover(catcher)
	defer erc.Check(catcher, iter.Close)

	for iter.Next(ctx) {
		if ferr := fn(ctx, iter.Value()); ferr != nil {
			catcher.Add(ferr)
			break
		}
	}

	return
}

// ParallelForEach processes the iterator in parallel, and is
// essentially an iterator-driven worker pool. The input iterator is
// split dynamically into iterators for every worker (determined by
// Options.NumWorkers,) with the division between workers determined
// by their processing speed (e.g. workers should not suffer from
// head-of-line blocking,) and input iterators are consumed (safely)
// as work is processed.
//
// Because there is no output in these operations
// Options.OutputBufferSize is ignored.
func ParallelForEach[T any](
	ctx context.Context,
	iter fun.Iterator[T],
	fn func(context.Context, T) error,
	opts Options,
) (err error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	catcher := &erc.Collector{}
	wg := &sync.WaitGroup{}
	defer func() { err = catcher.Resolve() }()
	defer erc.Recover(catcher)
	defer erc.Check(catcher, iter.Close)

	if opts.NumWorkers <= 0 {
		opts.NumWorkers = 1
	}

	splits := Split(ctx, opts.NumWorkers, iter)

	for idx := range splits {
		wg.Add(1)
		go forEachWorker(ctx, catcher, wg, opts, splits[idx], fn, cancel)
	}

	wg.Wait()
	return
}

func forEachWorker[T any](
	ctx context.Context,
	catcher *erc.Collector,
	wg *sync.WaitGroup,
	opts Options,
	iter fun.Iterator[T],
	fn func(context.Context, T) error,
	abort func(),
) {
	defer wg.Done()
	defer erc.RecoverHook(catcher, func() {
		if ctx.Err() != nil {
			return
		}
		if !opts.ContinueOnPanic {
			abort()
			return
		}

		wg.Add(1)
		go forEachWorker(ctx, catcher, wg, opts, iter, fn, abort)
	})
	for iter.Next(ctx) {
		if err := fn(ctx, iter.Value()); err != nil {
			if opts.IncludeContextExpirationErrors || !erc.ContextExpired(err) {
				// when the option is true, include all errors.
				// when false, only include not cancellation errors.
				catcher.Add(err)
			}
			if opts.ContinueOnError {
				continue
			}
			abort()
			return
		}
	}
}

// ProcessingFunction represents the underlying type used by serveral
// processing tools. Use the ProcessingFunction constructors to
// be able to write business logic more ergonomically.
//
// While the generic for ProcessingFunction allows for IN and OUT to
// be different types, they may be of the same type for Filtering
// operations.
type ProcessingFunction[IN any, OUT any] func(ctx context.Context, input IN) (output OUT, include bool, err error)

// Mapper wraps a simple function to provide a
// ProcessingFunction that will include the output of all mapped
// functions, but propagates errors (which usually abort processing)
// to the larger operation.
func Mapper[IN any, OUT any](fn func(context.Context, IN) (OUT, error)) ProcessingFunction[IN, OUT] {
	return func(ctx context.Context, input IN) (output OUT, include bool, err error) {
		out, err := fn(ctx, input)
		return out, true, err
	}
}

// Checker wraps a function, and *never* propogates an error
// to the calling operation but will include or exclude output based
// on the second boolean output.
func Checker[IN any, OUT any](fn func(context.Context, IN) (OUT, bool)) ProcessingFunction[IN, OUT] {
	return func(ctx context.Context, input IN) (output OUT, include bool, err error) {
		out, ok := fn(ctx, input)
		return out, ok, nil
	}
}

// Collector makes adds all errors to the error collector (instructing
// the iterator to skip these results,) but does not return an error
// to the outer processing operation. This would be the same as using
// "ContinueOnError" with the Map() operation.
func Collector[IN any, OUT any](ec *erc.Collector, fn func(context.Context, IN) (OUT, error)) ProcessingFunction[IN, OUT] {
	return func(ctx context.Context, input IN) (output OUT, include bool, err error) {
		out, err := fn(ctx, input)
		if err != nil {
			ec.Add(err)
			return *new(OUT), false, nil
		}
		return out, true, nil
	}
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
func Filter[T any](
	ctx context.Context,
	iter fun.Iterator[T],
	fn ProcessingFunction[T, T],
) fun.Iterator[T] {
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
func Transform[T any, O any](
	ctx context.Context,
	iter fun.Iterator[T],
	fn ProcessingFunction[T, O],
) fun.Iterator[O] {
	out := new(internal.MapIterImpl[O])
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
			o, include, err := fn(iterCtx, iter.Value())
			if err != nil {
				catcher.Add(err)
				break
			}
			if !include {
				continue
			}
			select {
			case <-iterCtx.Done():
			case pipe <- o:
			}
		}
	}()
	return out
}

// Observe is a special case of ForEach to support observer pattern
// functions. Observe functions should be short running as they do not
// take a context, and could block unexpectedly.
//
// Unlike fun.Observe, itertool.Observe collects errors from the
// iterator's Close method and recovers from panics in the
// observer function, propagating them in the returned error.
func Observe[T any](ctx context.Context, iter fun.Iterator[T], obfn func(T)) error {
	return ForEach(ctx, iter, func(_ context.Context, in T) error { obfn(in); return nil })
}

// ParallelObserve is a special case of ParallelObserve to support observer pattern
// functions. Observe functions should be short running as they do not
// take a context, and could block unexpectedly.
func ParallelObserve[T any](
	ctx context.Context,
	iter fun.Iterator[T],
	obfn func(T),
	opts Options,
) error {
	return ParallelForEach(ctx, iter, func(_ context.Context, in T) error { obfn(in); return nil }, opts)
}

// Options describes the runtime options to the Map, Generate,
// ParallelForEach, or ParallelObserve operations. The zero value of
// this struct provides a usable strict operation.
type Options struct {
	// ContinueOnPanic forces the entire IteratorMap operation to
	// halt when a single map function panics. All panics are
	// converted to errors and propagated to the output iterator's
	// Close() method.
	ContinueOnPanic bool
	// ContinueOnError allows a map or generate function to return
	// an error and allow the work of the broader operation to
	// continue. Errors are aggregated propagated to the output
	// iterator's Close() method.
	ContinueOnError bool
	// NumWorkers describes the number of parallel workers
	// processing the incoming iterator items and running the map
	// function. All values less than 1 are converted to 1. Any
	// value greater than 1 will result in out-of-sequence results
	// in the output iterator.
	NumWorkers int
	// OutputBufferSize controls how buffered the output pipe on the
	// iterator should be. Typically this should be zero, but
	// there are workloads for which a moderate buffer may be
	// useful.
	OutputBufferSize int
	// IncludeContextExpirationErrors changes the default handling
	// of context cancellation errors. By default all errors
	// rooted in context cancellation are not propagated to the
	// Close() method, however, when true, these errors are
	// captured. All other error handling semantics
	// (e.g. ContinueOnError) are applicable.
	IncludeContextExpirationErrors bool
}

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
	ctx context.Context,
	iter fun.Iterator[T],
	mapper ProcessingFunction[T, O],
	opts Options,
) fun.Iterator[O] {
	out := new(internal.MapIterImpl[O])
	safeOut := Synchronize[O](out)
	toOutput := make(chan O)
	catcher := &erc.Collector{}
	out.Pipe = toOutput

	abortCtx, abort := context.WithCancel(ctx)
	var iterCtx context.Context
	iterCtx, out.Closer = context.WithCancel(abortCtx)

	if opts.NumWorkers <= 0 {
		opts.NumWorkers = 1
	}

	fromInput := make(chan T, opts.NumWorkers)

	out.WG.Add(1)
	signal := make(chan struct{})

	go func() {
		defer close(signal)
		defer out.WG.Done()
		defer erc.Recover(catcher)
		defer erc.Check(catcher, iter.Close)
		defer close(fromInput)
		defer erc.Recover(catcher)

		for iter.Next(abortCtx) {
			select {
			case <-abortCtx.Done():
				return
			case fromInput <- iter.Value():
				continue
			}
		}
	}()

	wg := &sync.WaitGroup{}
	for i := 0; i < opts.NumWorkers; i++ {
		wg.Add(1)
		go mapWorker(iterCtx, catcher, wg, opts, mapper, abort, fromInput, toOutput)
	}

	out.WG.Add(1)
	go func() {
		defer out.WG.Done()
		defer func() { out.Error = catcher.Resolve() }()
		defer erc.Recover(catcher)
		<-signal
		fun.Wait(ctx, wg)
		abort()
		close(toOutput)
	}()

	return safeOut
}

func mapWorker[T any, O any](
	ctx context.Context,
	catcher *erc.Collector,
	wg *sync.WaitGroup,
	opts Options,
	mapper ProcessingFunction[T, O],
	abort func(),
	fromInput <-chan T,
	toOutput chan<- O,
) {
	defer wg.Done()
	defer erc.RecoverHook(catcher, func() {
		if !opts.ContinueOnPanic {
			abort()
			return
		}

		wg.Add(1)
		go mapWorker(ctx, catcher, wg, opts, mapper, abort, fromInput, toOutput)
	})
	for {
		select {
		case <-ctx.Done():
			return
		case value, ok := <-fromInput:
			if !ok {
				return
			}
			o, include, err := mapper(ctx, value)
			if !include {
				continue
			}
			if err != nil {
				if opts.IncludeContextExpirationErrors || !erc.ContextExpired(err) {
					// when the option is true, include all errors.
					// when false, only include not cancellation errors.
					catcher.Add(err)
				}
				if opts.ContinueOnError {
					continue
				}
				abort()
				return
			}
			select {
			case <-ctx.Done():
				return
			case toOutput <- o:
			}
		}
	}
}

// Reduce processes an input iterator with a reduce function and
// outputs the final value. The initial value may be a zero or nil
// value.
func Reduce[T any, O any](
	ctx context.Context,
	iter fun.Iterator[T],
	reducer func(context.Context, T, O) (O, error),
	initalValue O,
) (value O, err error) {
	value = initalValue
	catcher := &erc.Collector{}

	defer func() { err = catcher.Resolve() }()
	defer erc.Recover(catcher)
	defer erc.Check(catcher, iter.Close)

	for iter.Next(ctx) {
		value, err = reducer(ctx, iter.Value(), value)
		if err != nil {
			catcher.Add(err)
			return
		}
	}

	return
}

// ErrAbortGenerator is a sentinel error returned by generators to
// abort. This error is never propagated to calling functions.
var ErrAbortGenerator = errors.New("abort generator signal")

// Generate creates an iterator using a generator pattern which
// produces items until the context is canceled or the generator
// function returns ErrAbortGenerator. Parallel operation is also
// available and Generate shares configuration and semantics with the
// Map operation.
func Generate[T any](
	ctx context.Context,
	fn func(context.Context) (T, error),
	opts Options,
) fun.Iterator[T] {
	if opts.OutputBufferSize < 0 {
		opts.OutputBufferSize = 0
	}

	out := new(internal.MapIterImpl[T])
	pipe := make(chan T, opts.OutputBufferSize)
	catcher := &erc.Collector{}
	out.Pipe = pipe

	gctx, abort := context.WithCancel(ctx)
	out.Closer = abort

	if opts.NumWorkers <= 0 {
		opts.NumWorkers = 1
	}

	wg := &sync.WaitGroup{}
	for i := 0; i < opts.NumWorkers; i++ {
		wg.Add(1)
		go generator(gctx, catcher, wg, opts, fn, abort, pipe)
	}
	out.WG.Add(1)
	go func() {
		defer out.WG.Done()
		defer func() { out.Error = catcher.Resolve() }()
		defer erc.Recover(catcher)
		wg.Wait()
		abort()
		// can't wait against a context here because it's canceled
		close(pipe)
	}()

	return Synchronize[T](out)
}

func generator[T any](
	ctx context.Context,
	catcher *erc.Collector,
	wg *sync.WaitGroup,
	opts Options,
	fn func(context.Context) (T, error),
	abort func(),
	out chan<- T,
) {
	defer wg.Done()
	defer erc.RecoverHook(catcher, func() {
		if !opts.ContinueOnPanic {
			abort()
			return
		}

		wg.Add(1)
		go generator(ctx, catcher, wg, opts, fn, abort, out)
	})
	for {
		value, err := fn(ctx)
		if err != nil {
			if errors.Is(err, ErrAbortGenerator) {
				abort()
				return
			}
			if opts.IncludeContextExpirationErrors || !erc.ContextExpired(err) {
				// when the option is true, include all errors.
				// when false, only include not cancellation errors.
				catcher.Add(err)
			}
			if opts.ContinueOnError {
				continue
			}
			abort()
			return
		}

		select {
		case <-ctx.Done():
			return
		case out <- value:
			continue
		}
	}
}
