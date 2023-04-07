package itertool

import (
	"context"
	"errors"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/internal"
)

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
	iter = Synchronize(iter)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	catcher := &erc.Collector{}
	wg := &fun.WaitGroup{}
	defer func() { err = catcher.Resolve() }()
	defer erc.Check(catcher, iter.Close)
	defer erc.Recover(catcher)

	if opts.NumWorkers <= 0 {
		opts.NumWorkers = 1
	}

	splits := Split(ctx, opts.NumWorkers, iter)

	for idx := range splits {
		wg.Add(1)
		go forEachWorker(ctx, catcher, wg, opts, splits[idx], fn, cancel)
	}

	wg.Wait(internal.BackgroundContext)
	return
}

func forEachWorker[T any](
	ctx context.Context,
	catcher *erc.Collector,
	wg *fun.WaitGroup,
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

// ParallelObserve is a special case of ParallelObserve to support observer pattern
// functions. Observe functions should be short running as they do not
// take a context, and could block unexpectedly.
func ParallelObserve[T any](ctx context.Context, iter fun.Iterator[T], obfn func(T), opts Options) error {
	return ParallelForEach(ctx, iter, func(_ context.Context, in T) error { obfn(in); return nil }, opts)
}

// WorkerPool process the worker functions in the provided
// iterator. Unlike fun.WorkerPool, this implementation checks has
// options for handling errors and aborting on error.
//
// The pool follows the semantics configured by the Options, with
// regards to error handling, panic handling, and parallelism. Errors
// are collected and propagated WorkerPool output.
func WorkerPool(ctx context.Context, iter fun.Iterator[fun.WorkerFunc], opts Options) error {
	return ParallelForEach(ctx, iter,
		func(ctx context.Context, wf fun.WorkerFunc) error { return wf.Run(ctx) },
		opts,
	)
}

// ObserveWorkerPool executes the worker functions from the iterator
// in a worker pool that operates according to the Options
// configuration: including pool size, and error and panic handling.
// If a worker panics, ObserveWorkerPool will convert the panic(s)
// into an error, and pass that panic through the observe function.
func ObserveWorkerPool(ctx context.Context, iter fun.Iterator[fun.WorkerFunc], ob func(error), opts Options) {
	// use ForEach rather than Observe to thread the context
	// through to the workers less indirectly.
	if err := ParallelForEach(ctx, iter,
		func(ctx context.Context, wf fun.WorkerFunc) error {
			wf.Observe(ctx, ob)
			return nil
		}, opts,
	); err != nil {
		ob(err)
	}
}

// Options describes the runtime options to several operations
// operations. The zero value of this struct provides a usable strict
// operation.
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
	mapper func(context.Context, T) (O, error),
	opts Options,
) fun.Iterator[O] {
	out := new(internal.ChannelIterImpl[O])
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

	wg := &fun.WaitGroup{}
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
		wg.Wait(ctx)
		abort()
		close(toOutput)
	}()

	return safeOut
}

func mapWorker[T any, O any](
	ctx context.Context,
	catcher *erc.Collector,
	wg *fun.WaitGroup,
	opts Options,
	mapper func(context.Context, T) (O, error),
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
			o, err := mapper(ctx, value)
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

	out := new(internal.ChannelIterImpl[T])
	pipe := make(chan T, opts.OutputBufferSize)
	catcher := &erc.Collector{}
	out.Pipe = pipe

	gctx, abort := context.WithCancel(ctx)
	out.Closer = abort

	if opts.NumWorkers <= 0 {
		opts.NumWorkers = 1
	}

	wg := &fun.WaitGroup{}
	for i := 0; i < opts.NumWorkers; i++ {
		wg.Add(1)
		go generator(gctx, catcher, wg, opts, fn, abort, pipe)
	}
	out.WG.Add(1)
	go func() {
		defer out.WG.Done()
		defer func() { out.Error = catcher.Resolve() }()
		defer erc.Recover(catcher)
		wg.Wait(ctx)
		abort()
		// can't wait against a context here because it's canceled
		close(pipe)
	}()

	return Synchronize[T](out)
}

func generator[T any](
	ctx context.Context,
	catcher *erc.Collector,
	wg *fun.WaitGroup,
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
			expired := erc.ContextExpired(err)
			if errors.Is(err, ErrAbortGenerator) || (expired && !opts.IncludeContextExpirationErrors) {
				abort()
				return
			}

			catcher.Add(err)

			if !opts.ContinueOnError || expired {
				abort()
				return
			}
			continue
		}

		if err := internal.SendOne(ctx, internal.Blocking(true), out, value); err != nil {
			return
		}
	}
}
