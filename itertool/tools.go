package itertool

import (
	"context"
	"errors"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/internal"
)

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
	// SkipErrorCheck, when specified, should return true if the
	// error should be skipped and false otherwise.
	SkipErrorCheck func(error) bool
}

func (opts Options) shouldCollectError(err error) bool {
	return err != nil &&
		(opts.SkipErrorCheck == nil || !opts.SkipErrorCheck(err)) &&
		(opts.IncludeContextExpirationErrors || !erc.ContextExpired(err))
}

func (opts Options) wrapErrorCheck(newCheck func(error) bool) func(error) bool {
	oldCheck := opts.SkipErrorCheck
	return func(err error) bool {
		return (oldCheck == nil || oldCheck(err)) &&
			(newCheck == nil || newCheck(err))
	}
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
			erc.When(catcher, opts.shouldCollectError(err), err)

			if opts.ContinueOnError {
				continue
			}

			abort()
			return
		}
	}
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

		for {
			val, err := fun.IterateOne(abortCtx, iter)
			if err != nil {
				return
			}

			select {
			case <-abortCtx.Done():
				return
			case fromInput <- val:
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
				erc.When(catcher, opts.shouldCollectError(err), err)

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
	shouldCollectError := opts.wrapErrorCheck(func(err error) bool { return !errors.Is(err, ErrAbortGenerator) })

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
			erc.When(catcher, shouldCollectError(err), err)

			if errors.Is(err, ErrAbortGenerator) || erc.ContextExpired(err) || !opts.ContinueOnError {
				abort()
				return
			}

			continue
		}

		if !fun.Blocking(out).Check(ctx, value) {
			return
		}
	}
}
