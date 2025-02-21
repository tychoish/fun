package fun

import (
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
)

// Processor are generic functions that take an argument (and a
// context) and return an error. They're the type of function used by
// the itertool.Process/itertool.ParallelForEach and useful in other
// situations as a compliment to fun.Worker and Operation.
//
// In general the implementations of the methods for processing
// functions are wrappers around their similarly named fun.Worker
// analogues.
type Processor[T any] func(context.Context, T) error

// MakeProcessor converts a function with the Processor signature
// (minus the context) for easy conversion.
func MakeProcessor[T any](fn func(T) error) Processor[T] {
	return func(_ context.Context, in T) error { return fn(in) }
}

// MakeHandlerProcessor converts a handler-type function to a processor.
func MakeHandlerProcessor[T any](fn func(T)) Processor[T] {
	return func(_ context.Context, in T) error { fn(in); return nil }
}

// NewProcessors returns a processor as a convenience function to
// avoid the extra cast when creating new function objects.
func NewProcessor[T any](fn func(context.Context, T) error) Processor[T] { return fn }

// Processify provides an easy converter/constructor for
// Processor-type function.
//
// Deprecated: use NewProcessor for this case.
func Processify[T any](fn func(context.Context, T) error) Processor[T] { return NewProcessor(fn) }

// ProcessifyHandler converts a handler-type function to a processor.
//
// Deprecated: use MakeHandlerProcessor for this case.
func ProcessifyHandler[T any](fn Handler[T]) Processor[T] { return MakeHandlerProcessor(fn) }

// ProcessorGroup takes a collection of Processor functions and merges
// them into a single chain, eliding any nil processors.
func ProcessorGroup[T any](pfs ...Processor[T]) Processor[T] {
	var pf Processor[T]
	for _, fn := range pfs {
		switch {
		case fn == nil:
			continue
		case pf == nil:
			pf = fn
		default:
			pf = pf.merge(fn)
		}
	}
	return pf
}

// Run executes the Processors but creates a context within the
// function (decended from the context provided in the arguments,)
// that is canceled when Run() returns to avoid leaking well behaved
// resources outside of the scope of the function execution. Run can
// also be passed as a Processor func.
func (pf Processor[T]) Run(ctx context.Context, in T) error { return pf.Worker(in).Run(ctx) }

// Add begins running the process in a different goroutine, using the
// provided arguments to manage the operation.
func (pf Processor[T]) Add(ctx context.Context, wg *WaitGroup, eh Handler[error], op T) {
	pf.Operation(eh, op).Add(ctx, wg)
}

// Block runs the Processor with a context that will never be
// canceled.
//
// Deprecated: use Wait() instead.
func (pf Processor[T]) Block(in T) error { return pf.Wait(in) }

// Wait runs the Processor with a context that will never be
// canceled.
func (pf Processor[T]) Wait(in T) error { return pf.Worker(in).Wait() }

// Ignore runs the process function and discards the error.
func (pf Processor[T]) Ignore(ctx context.Context, in T) { ers.Ignore(pf(ctx, in)) }

// Check processes the input and returns true when the error is nil,
// and false when there was an error.
func (pf Processor[T]) Check(ctx context.Context, in T) bool { return ers.Ok(pf(ctx, in)) }

// Force processes the input, but discards the error and uses a
// context that will not expire.
func (pf Processor[T]) Force(in T) { pf.Worker(in).Ignore().Block() }

// WithRecover runs the producer, converted all panics into errors. WithRecover is
// itself a processor.
func (pf Processor[T]) WithRecover() Processor[T] {
	return func(ctx context.Context, in T) error { return pf.Worker(in).WithRecover().Run(ctx) }
}

// After produces a Processor that will execute after the provided
// timestamp.
func (pf Processor[T]) After(ts time.Time) Processor[T] { return pf.Delay(time.Until(ts)) }

// Delay wraps a Processor in a function that will always wait for the
// specified duration before running.
//
// If the value is negative, then there is always zero delay.
func (pf Processor[T]) Delay(dur time.Duration) Processor[T] { return pf.Jitter(ft.Wrapper(dur)) }

// Jitter wraps a Processor that runs the jitter function (jf) once
// before every execution of the resulting function, and waits for the
// resulting duration before running the processor.
//
// If the function produces a negative duration, there is no delay.
func (pf Processor[T]) Jitter(jf func() time.Duration) Processor[T] {
	return func(ctx context.Context, in T) error {
		timer := time.NewTimer(max(0, jf()))
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return pf(ctx, in)
		}
	}
}

// If runs the processor function if, and only if the condition is
// true. Otherwise the function does not run and the processor returns
// nil.
//
// The resulting processor can be used more than once.
func (pf Processor[T]) If(c bool) Processor[T] { return pf.When(ft.Wrapper(c)) }

// When returns a processor function that runs if the conditional function
// returns true, and does not run otherwise. The conditional function
// is evaluated every time the returned processor is run.
func (pf Processor[T]) When(c func() bool) Processor[T] {
	return func(ctx context.Context, in T) error {
		if c() {
			return pf(ctx, in)
		}
		return nil
	}
}

// Filter returns a wrapping processor that takes a function a
// function that only calls the processor when the filter function
// returns true, and returns ers.ErrCurrentOpSkip otherwise.
func (pf Processor[T]) Filter(fl func(T) bool) Processor[T] {
	return func(ctx context.Context, in T) error {
		if !fl(in) {
			return ers.ErrCurrentOpSkip
		}
		return pf.Run(ctx, in)
	}
}

// Join combines a sequence of processors on the same input, calling
// each function in order, as long as there is no error and the
// context does not expire. Context expiration errors are not
// propagated.
func (pf Processor[T]) Join(pfs ...Processor[T]) Processor[T] {
	for idx := range pfs {
		pf = pf.merge(pfs[idx])
	}
	return pf
}

func (pf Processor[T]) merge(next Processor[T]) Processor[T] {
	return func(ctx context.Context, in T) error {
		if err := pf(ctx, in); err != nil {
			return err
		}
		return next.If(ctx.Err() == nil).Run(ctx, in)
	}
}

// Iterator creates a worker function that processes the iterator with
// the processor function, merging/collecting all errors, and
// respecting the worker's context. The processing does not begin
// until the worker is called.
func (pf Processor[T]) Iterator(iter *Iterator[T]) Worker { return iter.Process(pf) }

// Background processes an item in a separate goroutine and returns a
// worker that will block until the underlying operation is complete.
func (pf Processor[T]) Background(ctx context.Context, op T) Worker {
	return pf.Worker(op).Launch(ctx)
}

// Capture creates a handler function that like, Processor.Force,
// passes a background context and ignores the processors error.
func (pf Processor[T]) Capture() Handler[T] { return pf.Force }

// Operation converts a processor into a worker that will process the input
// provided when executed.
func (pf Processor[T]) Operation(of Handler[error], in T) Operation {
	return pf.Worker(in).Operation(of)
}

// Handler converts a processor into an observer, handling the error
// with the error observer and using the provided context.
func (pf Processor[T]) Handler(ctx context.Context, oe Handler[error]) Handler[T] {
	return func(in T) { oe(pf(ctx, in)) }
}

// Worker converts the processor into a worker, passing the provide
// input into the root processor function. The Processor is not run
// until the worker is called.
func (pf Processor[T]) Worker(in T) Worker {
	return func(ctx context.Context) error { return pf(ctx, in) }
}

// Once make a processor that can only run once. Subsequent calls to
// the processor return the cached error of the original run.
func (pf Processor[T]) Once() Processor[T] {
	once := &sync.Once{}
	var err error
	return func(ctx context.Context, in T) error {
		once.Do(func() { err = pf(ctx, in) })
		return err
	}
}

// Lock wraps the Processor and protects its execution with a mutex.
func (pf Processor[T]) Lock() Processor[T] { return pf.WithLock(&sync.Mutex{}) }

// WithLock wraps the Processor and ensures that the mutex is always
// held while the root Processor is called.
func (pf Processor[T]) WithLock(mtx sync.Locker) Processor[T] {
	return func(ctx context.Context, arg T) error {
		mtx.Lock()
		defer mtx.Unlock()
		return pf(ctx, arg)
	}
}

// Limit ensures that the processor is called at most n times.
func (pf Processor[T]) Limit(n int) Processor[T] {
	resolver := limitExec[error](n)

	return func(ctx context.Context, arg T) error {
		return resolver(func() error { return pf(ctx, arg) })
	}
}

// TTL returns a Processor that runs once in the specified window, and
// returns the error from the last run in between this interval. While
// the executions of the underlying function happen in isolation,
// in between, the processor is concurrently accessible.
func (pf Processor[T]) TTL(dur time.Duration) Processor[T] {
	resolver := ttlExec[error](dur)

	return func(ctx context.Context, arg T) error {
		return resolver(func() error { return pf(ctx, arg) })
	}
}

// WithCancel creates a Processor and a cancel function which will
// terminate the context that the root proccessor is running
// with. This context isn't canceled *unless* the cancel function is
// called (or the context passed to the Processor is canceled.)
func (pf Processor[T]) WithCancel() (Processor[T], context.CancelFunc) {
	var wctx context.Context
	var cancel context.CancelFunc
	once := &sync.Once{}

	return func(ctx context.Context, in T) error {
		once.Do(func() { wctx, cancel = context.WithCancel(ctx) })
		Invariant.IsFalse(wctx == nil, "must start the operation before calling cancel")
		return pf(wctx, in)
	}, func() { once.Do(func() {}); ft.SafeCall(cancel) }
}

// PreHook creates an amalgamated Processor that runs the operation
// before the root processor. If the operation panics that panic is
// converted to an error and merged with the processor's error. Use
// with Operation.Once() to create an "init" function that runs once
// before a processor is called the first time.
func (pf Processor[T]) PreHook(op Operation) Processor[T] {
	return func(ctx context.Context, in T) error {
		return ers.Join(ers.WithRecoverCall(func() { op(ctx) }), pf(ctx, in))
	}
}

// PostHook produces an amalgamated processor that runs after the
// processor completes. Panics are caught, converted to errors, and
// aggregated with the processors error. The hook operation is
// unconditionally called after the processor function (except in the
// case of a processor panic.)
func (pf Processor[T]) PostHook(op func()) Processor[T] {
	return func(ctx context.Context, in T) error {
		return ers.Join(ft.Flip(pf(ctx, in), ers.WithRecoverCall(op)))
	}
}

// WithErrorFilter uses an ers.Filter to process the error respose from
// the processor.
func (pf Processor[T]) WithErrorFilter(ef ers.Filter) Processor[T] {
	return func(ctx context.Context, in T) error { return ef(pf(ctx, in)) }
}

// WithoutErrors returns a producer that will convert a non-nil error
// of the provided types to a nil error.
func (pf Processor[T]) WithoutErrors(errs ...error) Processor[T] {
	return pf.WithErrorFilter(ers.FilterExclude(errs...))
}

// WithErrorCheck takes an error future, and checks it before
// executing the processor operation. If the error future returns an
// error (any error), the processor propagates that error, rather than
// running the underying processor. Useful for injecting an abort into
// an existing pipleine or chain.
//
// The error future is called before running the underlying processor,
// to short circuit the operation, and also a second time when
// processor has returned in case an error has occurred during the
// operation of the processor.
func (pf Processor[T]) WithErrorCheck(ef Future[error]) Processor[T] {
	return func(ctx context.Context, in T) error {
		if err := ef(); err != nil {
			return err
		}
		return ers.Join(pf(ctx, in), ef())
	}
}

// ReadOne returns a future (Worker) that calls the processor function
// on the output of the provided producer function. ReadOne uses the
// fun.Pipe() operation for the underlying implementation.
func (pf Processor[T]) ReadOne(prod Producer[T]) Worker { return Pipe(prod, pf) }

// ReadAll reads elements from the producer until an error is
// encountered and passes them to a producer, until the first error is
// encountered. The worker is blocking.
func (pf Processor[T]) ReadAll(prod Producer[T]) Worker {
	return func(ctx context.Context) error {

	LOOP:
		for {
			out, err := prod(ctx)
			if err == nil {
				err = pf(ctx, out)
			}

			switch {
			case err == nil || errors.Is(err, ErrIteratorSkip):
				continue LOOP
			case ers.Is(err, io.EOF, ers.ErrCurrentOpAbort):
				return nil
			default:
				return err
			}
		}
	}
}

// Parallel takes a variadic number of items and returns a worker that
// processes them concurrently. All panics are converted to errors and
// all errors are aggregated.
func (pf Processor[T]) Parallel(ops ...T) Worker {
	return SliceIterator(ops).ProcessParallel(pf,
		WorkerGroupConfNumWorkers(len(ops)),
		WorkerGroupConfContinueOnError(),
		WorkerGroupConfContinueOnPanic(),
	)
}

// Retry makes a worker function that takes runs the processor
// function with the provied input until the return value is nil, or
// it encounters a terminating error (io.EOF, ers.ErrAbortCurrentOp,
// or context cancellation.)
//
// Context cancellation errors are returned to the caller, other
// terminating errors are not, with any other errors encountered
// during retries. ErrIteratorSkip is always ignored and not
// aggregated. All errors are discarded if the retry operation
// succeeds in the provided number of retries.
//
// Except for ErrIteratorSkip, which is ignored, all other errors are
// aggregated and returned to the caller only if the retry fails. It's
// possible to return a nil error without successfully completing the
// underlying operation, if the processor only returned
// ErrIteratorSkip values.
func (pf Processor[T]) Retry(n int, in T) Worker { return pf.Worker(in).Retry(n) }

////////////////////////////////////////////////////////////////////////

type tuple[T, U any] struct {
	One T
	Two U
}

func limitExec[T any](in int) func(func() T) T {
	Invariant.IsTrue(in > 0, "limit must be greater than zero;", in)
	counter := &atomic.Int64{}
	mtx := &sync.Mutex{}

	var output T
	return func(op func() T) T {
		if counter.CompareAndSwap(int64(in), int64(in)) {
			return output
		}

		mtx.Lock()
		defer mtx.Unlock()
		num := counter.Load()

		if num < int64(in) {
			output = op()
			counter.Store(min(int64(in), num+1))
		}

		return output
	}
}

func ttlExec[T any](dur time.Duration) func(op func() T) T {
	Invariant.IsTrue(dur >= 0, "ttl must not be negative;", dur)

	if dur == 0 {
		return func(op func() T) T { return op() }
	}

	var (
		lastAt time.Time
		output T
	)
	mtx := &sync.RWMutex{}

	return func(op func() T) (out T) {
		if out, ok := func() (T, bool) {
			mtx.RLock()
			defer mtx.RUnlock()
			return output, !lastAt.IsZero() && time.Since(lastAt) < dur
		}(); ok {
			return out
		}

		mtx.Lock()
		defer mtx.Unlock()

		since := time.Since(lastAt)
		if lastAt.IsZero() || since >= dur {
			output = op()
			lastAt = time.Now()
		}

		return output
	}
}
