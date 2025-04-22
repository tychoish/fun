package fun

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/fn"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/internal"
)

// Handler are generic functions that take an argument (and a context)
// and return an error. They're the type of function used in various
// stream methods to implement worker pools, service management, and
// stream processing.
type Handler[T any] func(context.Context, T) error

// MakeHandler converts a function with the Handler signature
// (minus the context) for easy conversion.
func MakeHandler[T any](fn func(T) error) Handler[T] {
	return func(_ context.Context, in T) error { return fn(in) }
}

// FromHandler up-converts a fn.Handler (e.g. a more simple handler
// function that doesn't return an error or take a context,) into a
// fun.Handler. the underlying function runs with a panic handler (so
// fn.Handler panics are converted to errors.)
func FromHandler[T any](f fn.Handler[T]) Handler[T] { return MakeHandler(f.Safe()) }

// NewHandler returns a Handler Function. This is a convenience
// function to avoid the extra cast when creating new function
// objects.
func NewHandler[T any](fn func(context.Context, T) error) Handler[T] { return fn }

// JoinHandlers takes a collection of Handler functions and merges
// them into a single chain, eliding any nil processors.
func JoinHandlers[T any](pfs ...Handler[T]) Handler[T] {
	var pf Handler[T]
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

// Wait runs the Handler with a context that will never be
// canceled.
func (pf Handler[T]) Wait(in T) error { return pf(context.Background(), in) }

// Ignore runs the process function and discards the error.
func (pf Handler[T]) Ignore(ctx context.Context, in T) { ft.IgnoreError(pf(ctx, in)) }

// Check processes the input and returns true when the error is nil,
// and false when there was an error.
func (pf Handler[T]) Check(ctx context.Context, in T) bool { return ers.IsOk(pf(ctx, in)) }

// Force processes the input, but discards the error and uses a
// context that will not expire.
func (pf Handler[T]) Force(in T) { ft.Ignore(pf.Wait(in)) }

func (pf Handler[T]) Must(ctx context.Context, in T) { Invariant.Must(pf.Read(ctx, in)) }

// WithRecover runs the producer, converted all panics into errors. WithRecover is
// itself a processor.
func (pf Handler[T]) WithRecover() Handler[T] {
	return func(ctx context.Context, in T) (err error) {
		defer func() { err = erc.Join(err, erc.ParsePanic(recover())) }()
		return pf(ctx, in)
	}
}

// After produces a Handler that will execute after the provided
// timestamp.
func (pf Handler[T]) After(ts time.Time) Handler[T] { return pf.Delay(time.Until(ts)) }

// Delay wraps a Handler in a function that will always wait for the
// specified duration before running.
//
// If the value is negative, then there is always zero delay.
func (pf Handler[T]) Delay(dur time.Duration) Handler[T] { return pf.Jitter(ft.Wrap(dur)) }

// Jitter wraps a Handler that runs the jitter function (jf) once
// before every execution of the resulting function, and waits for the
// resulting duration before running the processor.
//
// If the function produces a negative duration, there is no delay.
func (pf Handler[T]) Jitter(jf func() time.Duration) Handler[T] {
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
func (pf Handler[T]) If(c bool) Handler[T] { return pf.When(ft.Wrap(c)) }

// When returns a processor function that runs if the conditional function
// returns true, and does not run otherwise. The conditional function
// is evaluated every time the returned processor is run.
func (pf Handler[T]) When(c func() bool) Handler[T] {
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
func (pf Handler[T]) Filter(fl func(T) bool) Handler[T] {
	return func(ctx context.Context, in T) error {
		if !fl(in) {
			return ers.ErrCurrentOpSkip
		}
		return pf.Read(ctx, in)
	}
}

// Join combines a sequence of processors on the same input, calling
// each function in order, as long as there is no error and the
// context does not expire. Context expiration errors are not
// propagated.
func (pf Handler[T]) Join(pfs ...Handler[T]) Handler[T] {
	for idx := range pfs {
		pf = pf.merge(pfs[idx])
	}
	return pf
}

func (pf Handler[T]) merge(next Handler[T]) Handler[T] {
	return func(ctx context.Context, in T) error {
		if err := pf(ctx, in); err != nil {
			return err
		}
		return next.If(ctx.Err() == nil).Read(ctx, in)
	}
}

// Capture creates a handler function that like, Handler.Force,
// passes a background context and ignores the processors error.
func (pf Handler[T]) Capture() fn.Handler[T] { return pf.Force }

// Handler converts a processor into an observer, handling the error
// with the error observer and using the provided context.
func (pf Handler[T]) Handler(ctx context.Context, oe fn.Handler[error]) fn.Handler[T] {
	return func(in T) { oe(pf(ctx, in)) }
}

// Once make a processor that can only run once. Subsequent calls to
// the processor return the cached error of the original run.
func (pf Handler[T]) Once() Handler[T] {
	once := &sync.Once{}
	var err error
	return func(ctx context.Context, in T) error {
		once.Do(func() { err = pf(ctx, in) })
		return err
	}
}

// Lock wraps the Handler and protects its execution with a mutex.
func (pf Handler[T]) Lock() Handler[T] { return pf.WithLock(&sync.Mutex{}) }

// WithLock wraps the Handler and ensures that the mutex is always
// held while the root Handler is called.
func (pf Handler[T]) WithLock(mtx *sync.Mutex) Handler[T] {
	return func(ctx context.Context, arg T) error {
		mtx.Lock()
		defer mtx.Unlock()
		return pf(ctx, arg)
	}
}

func (pf Handler[T]) WithLocker(mtx sync.Locker) Handler[T] {
	return func(ctx context.Context, in T) error {
		defer internal.WithL(internal.LockL(mtx))
		return pf.Read(ctx, in)
	}
}

// Limit ensures that the processor is called at most n times.
func (pf Handler[T]) Limit(n int) Handler[T] {
	resolver := ft.Must(internal.LimitExec[error](n))

	return func(ctx context.Context, arg T) error {
		return resolver(func() error { return pf(ctx, arg) })
	}
}

// TTL returns a Handler that runs once in the specified window, and
// returns the error from the last run in between this interval. While
// the executions of the underlying function happen in isolation,
// in between, the processor is concurrently accessible.
func (pf Handler[T]) TTL(dur time.Duration) Handler[T] {
	resolver := ft.Must(internal.TTLExec[error](dur))

	return func(ctx context.Context, arg T) error {
		return resolver(func() error { return pf(ctx, arg) })
	}
}

// WithCancel creates a Handler and a cancel function which will
// terminate the context that the root proccessor is running
// with. This context isn't canceled *unless* the cancel function is
// called (or the context passed to the Handler is canceled.)
func (pf Handler[T]) WithCancel() (Handler[T], context.CancelFunc) {
	var wctx context.Context
	var cancel context.CancelFunc
	once := &sync.Once{}

	return func(ctx context.Context, in T) error {
		once.Do(func() { wctx, cancel = context.WithCancel(ctx) })
		Invariant.IsFalse(wctx == nil, "must start the operation before calling cancel")
		return pf(wctx, in)
	}, func() { once.Do(func() {}); ft.SafeCall(cancel) }
}

// PreHook creates an amalgamated Handler that runs the operation
// before the root processor. If the operation panics that panic is
// converted to an error and merged with the processor's error. Use
// with Operation.Once() to create an "init" function that runs once
// before a processor is called the first time.
func (pf Handler[T]) PreHook(op Operation) Handler[T] {
	return func(ctx context.Context, in T) error {
		return erc.Join(ft.WithRecoverCall(func() { op(ctx) }), pf(ctx, in))
	}
}

// PostHook produces an amalgamated processor that runs after the
// processor completes. Panics are caught, converted to errors, and
// aggregated with the processors error. The hook operation is
// unconditionally called after the processor function (except in the
// case of a processor panic.)
func (pf Handler[T]) PostHook(op func()) Handler[T] {
	return func(ctx context.Context, in T) error {
		return erc.Join(ft.Flip(pf(ctx, in), ft.WithRecoverCall(op)))
	}
}

// WithErrorFilter uses an erc.Filter to process the error respose from
// the processor.
func (pf Handler[T]) WithErrorFilter(ef erc.Filter) Handler[T] {
	return func(ctx context.Context, in T) error { return ef(pf(ctx, in)) }
}

// WithoutErrors returns a producer that will convert a non-nil error
// of the provided types to a nil error.
func (pf Handler[T]) WithoutErrors(errs ...error) Handler[T] {
	return pf.WithErrorFilter(erc.FilterExclude(errs...))
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
func (pf Handler[T]) WithErrorCheck(ef fn.Future[error]) Handler[T] {
	return func(ctx context.Context, in T) error {
		if err := ef(); err != nil {
			return err
		}
		return erc.Join(pf(ctx, in), ef())
	}
}

// Read executes the Handler once.
func (pf Handler[T]) Read(ctx context.Context, in T) error { return pf(ctx, in) }

// ReadAll reads elements from the producer until an error is
// encountered and passes them to a producer, until the first error is
// encountered. The worker is blocking.
func (pf Handler[T]) ReadAll(st *Stream[T]) Worker {
	return func(ctx context.Context) (err error) {
		defer func() { err = erc.Join(err, st.Close(), erc.ParsePanic(recover())) }()

		for {
			item, err := st.Read(ctx)
			if err == nil {
				err = pf(ctx, item)
			}

			switch {
			case err == nil:
				continue
			case errors.Is(err, ErrStreamContinue):
				continue
			case ers.IsTerminating(err):
				return nil
			case ers.IsExpiredContext(err):
				return err
			default:
				return err
			}
		}
	}
}
