package fnx

import (
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/fn"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/internal"
)

// Future is a function type that is a failrly common
// constructor. It's signature is used to create iterators/streams, as
// a future, and functions like a Future.
type Future[T any] func(context.Context) (T, error)

// MakeFuture constructs a future that wraps a similar
// function that does not take a context.
func MakeFuture[T any](fn func() (T, error)) Future[T] {
	return func(context.Context) (T, error) { return fn() }
}

// NewFuture returns a future as a convenience function to avoid
// the extra cast when creating new function objects.
func NewFuture[T any](fn func(ctx context.Context) (T, error)) Future[T] { return fn }

// StaticFuture returns a future function that always returns the
// provided values.
func StaticFuture[T any](val T, err error) Future[T] {
	return func(context.Context) (T, error) { return val, err }
}

// ValueFuture returns a future function that always returns the
// provided value, and a nill error.
func ValueFuture[T any](val T) Future[T] { return StaticFuture(val, nil) }

// CheckedFuture wraps a function object that uses the second ("OK")
// value to indicate that no more values will be produced. Errors
// returned from the resulting produce are always either the context
// cancellation error or io.EOF.
func CheckedFuture[T any](op func() (T, bool)) Future[T] {
	return func(context.Context) (zero T, _ error) {
		out, ok := op()
		if !ok {
			return zero, io.EOF
		}
		return out, nil
	}
}

// PtrFuture uses a function that returns a pointer to a value and
// converts that into a future that de-references and returns
// non-nil values of the pointer, and returns EOF for nil values of
// the pointer.
func PtrFuture[T any](fn func() *T) Future[T] {
	return CheckedFuture(func() (zero T, _ bool) {
		if val := fn(); val != nil {
			return *val, true
		}
		return zero, false
	})
}

// WrapFuture creates a future for the fn.Future
// function. The underlying Future's panics are converted to errors.
func WrapFuture[T any](f fn.Future[T]) Future[T] { return MakeFuture(f.RecoverPanic) }

// WithRecover returns a wrapped future with a panic handler that converts
// any panic to an error.
func (pf Future[T]) WithRecover() Future[T] {
	return func(ctx context.Context) (_ T, err error) {
		defer func() { err = erc.Join(err, erc.ParsePanic(recover())) }()
		return pf(ctx)
	}
}

// Join runs the first future until it returns an io.EOF error, and then returns the results of the second future. A
// future will be re-run until it returns a terminating error (e.g. io.EOF, ers.ErrContainerClosed, or
// ers.ErrCurrentOpAbort). If the first future returns any other error, that error is returned to the caller and the second
// future never runs.
//
// When the second function returns a terminating error, or any other error, all successive calls return io.EOF.
func (pf Future[T]) Join(next Future[T]) Future[T] {
	const (
		runFirstFunc int64 = iota
		firstFunctionErrored
		runSecondFunc
		secondFunctionErrored
		eof
	)

	var zero T
	var ferr error
	var serr error
	stage := &atomic.Int64{}
	stage.Store(runFirstFunc)

	return func(ctx context.Context) (out T, err error) {
		switch stage.Load() {
		case secondFunctionErrored:
			return zero, serr
		case firstFunctionErrored:
			return zero, ferr
		case runFirstFunc:

		RETRY_FIRST:
			for {
				out, err = pf(ctx)
				switch {
				case err == nil:
					return out, nil
				case errors.Is(err, ers.ErrCurrentOpSkip):
					continue RETRY_FIRST
				case !ers.IsTerminating(err):
					ferr = err
					stage.Store(firstFunctionErrored)
					return zero, err
				}
				break RETRY_FIRST
			}
			stage.Store(runSecondFunc)
			fallthrough
		case runSecondFunc:

		RETRY_SECOND:
			for {
				out, err = next(ctx)
				switch {
				case err == nil:
					return out, nil
				case errors.Is(err, ers.ErrCurrentOpSkip):
					continue RETRY_SECOND
				case !errors.Is(err, io.EOF):
					serr = err
					stage.Store(secondFunctionErrored)
					return zero, err
				default:
					stage.Store(eof)
					return zero, err
				}
			}

		default:
			return out, io.EOF
		}
	}
}

// Future creates a future function using the context provided and
// error observer to collect the error.
func (pf Future[T]) Future(ctx context.Context, ob fn.Handler[error]) fn.Future[T] {
	return func() T { out, err := pf(ctx); ob(err); return out }
}

// Ignore creates a future that runs the future and returns
// the value, ignoring the error.
func (pf Future[T]) Ignore(ctx context.Context) fn.Future[T] {
	return func() T { return ft.IgnoreSecond(pf(ctx)) }
}

// Must returns a future that resolves the future returning the
// constructed value and panicing if the future errors.
func (pf Future[T]) Must(ctx context.Context) fn.Future[T] {
	return func() T { return erc.Must(pf.Read(ctx)) }
}

// Force combines the semantics of Must and Wait as a future: when the
// future is resolved, the future executes with a context that never
// expires and panics in the case of an error.
func (pf Future[T]) Force() fn.Future[T] { return func() T { return ft.IgnoreSecond(pf.Wait()) } }

// Resolve calls the function returned by Force resolving the future. This ignores all errors and provides a the underlying
// future with a context that will never be canceled.
func (pf Future[T]) Resolve() T { return ft.Do(pf.Force()) }

// Wait runs the future with a context that will ever expire.
func (pf Future[T]) Wait() (T, error) { return pf(context.Background()) }

// Check converts the error into a boolean, with true indicating
// success and false indicating (but not propagating it.).
func (pf Future[T]) Check(ctx context.Context) (T, bool) { o, e := pf(ctx); return o, e == nil }

// WithErrorCheck takes an error future, and checks it before
// executing the future function. If the error future returns an
// error (any error), the future propagates that error, rather than
// running the underying future. Useful for injecting an abort into
// an existing pipleine or chain.
func (pf Future[T]) WithErrorCheck(ef fn.Future[error]) Future[T] {
	return func(ctx context.Context) (zero T, _ error) {
		if err := ef(); err != nil {
			return zero, err
		}

		out, err := pf.Read(ctx)
		if err = erc.Join(err, ef()); err != nil {
			return zero, err
		}

		return out, nil
	}
}

// Once returns a future that only executes ones, and caches the
// return values, so that subsequent calls to the output future will
// return the same values.
func (pf Future[T]) Once() Future[T] {
	var (
		out T
		err error
	)

	once := &sync.Once{}
	pf = pf.WithRecover()

	return func(ctx context.Context) (T, error) {
		once.Do(func() { out, err = pf(ctx) })
		return out, err
	}
}

// If returns a future that will execute the root future only if
// the cond value is true. Otherwise, If will return the zero value
// for T and a nil error.
func (pf Future[T]) If(cond bool) Future[T] { return pf.When(ft.Wrap(cond)) }

// After will return a Future that will block until the provided
// time is in the past, and then execute normally.
func (pf Future[T]) After(ts time.Time) Future[T] { return pf.Delay(time.Until(ts)) }

// Delay wraps a Future in a function that will always wait for the
// specified duration before running.
//
// If the value is negative, then there is always zero delay.
func (pf Future[T]) Delay(d time.Duration) Future[T] { return pf.Jitter(ft.Wrap(d)) }

// Jitter wraps a Future that runs the jitter function (jf) once
// before every execution of the resulting function, and waits for the
// resulting duration before running the Future.
//
// If the function produces a negative duration, there is no delay.
func (pf Future[T]) Jitter(jf func() time.Duration) Future[T] {
	return func(ctx context.Context) (out T, _ error) {
		timer := time.NewTimer(max(0, jf()))
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return out, ctx.Err()
		case <-timer.C:
			return pf(ctx)
		}
	}
}

// When constructs a future that will call the cond upon every
// execution, and when true, will run and return the results of the
// root future. Otherwise When will return the zero value of T and a
// nil error.
func (pf Future[T]) When(cond func() bool) Future[T] {
	return func(ctx context.Context) (out T, _ error) {
		if cond() {
			return pf(ctx)
		}
		return out, nil
	}
}

// Lock creates a future that runs the root mutex as per normal, but
// under the protection of a mutex so that there's only one execution
// of the future at a time.
func (pf Future[T]) Lock() Future[T] { return pf.WithLock(&sync.Mutex{}) }

// WithLock uses the provided mutex to protect the execution of the future.
func (pf Future[T]) WithLock(mtx *sync.Mutex) Future[T] {
	return func(ctx context.Context) (T, error) { mtx.Lock(); defer mtx.Unlock(); return pf(ctx) }
}

// WithLocker uses the provided mutex to protect the execution of the future.
func (pf Future[T]) WithLocker(mtx sync.Locker) Future[T] {
	return func(ctx context.Context) (T, error) { mtx.Lock(); defer mtx.Unlock(); return pf(ctx) }
}

// WithCancel creates a Future and a cancel function which will
// terminate the context that the root Future is running
// with. This context isn't canceled *unless* the cancel function is
// called (or the context passed to the Future is canceled.)
func (pf Future[T]) WithCancel() (Future[T], context.CancelFunc) {
	var wctx context.Context
	var cancel context.CancelFunc
	once := &sync.Once{}

	return func(ctx context.Context) (out T, _ error) {
		once.Do(func() { wctx, cancel = context.WithCancel(ctx) })
		invariant(wctx != nil, "must start the operation before calling cancel")
		return pf(wctx)
	}, func() { once.Do(func() {}); ft.CallSafe(cancel) }
}

// Limit runs the future a specified number of times, and caches the
// result of the last execution and returns that value for any
// subsequent executions.
func (pf Future[T]) Limit(in int) Future[T] {
	resolver := erc.Must(internal.LimitExec[tuple[T, error]](in))

	return func(ctx context.Context) (T, error) {
		out := resolver(func() (val tuple[T, error]) {
			val.One, val.Two = pf(ctx)
			return
		})
		return out.One, out.Two
	}
}

// Retry constructs a worker function that takes runs the underlying
// future until the error value is nil, or it encounters a
// terminating error (io.EOF, ers.ErrAbortCurrentOp, or context
// cancellation.) In all cases, unless the error value is nil
// (e.g. the retry succeeds)
//
// Context cancellation errors are returned to the caller, other
// terminating errors are not, with any other errors encountered
// during retries. ers.ErrCurrentOpSkip is always ignored and not
// aggregated. All errors are discarded if the retry operation
// succeeds in the provided number of retries.
//
// Except for ers.ErrCurrentOpSkip, which is ignored, all other errors are
// aggregated and returned to the caller only if the retry fails. It's
// possible to return a nil error and a zero value, if the future
// only returned ers.ErrCurrentOpSkip values.
func (pf Future[T]) Retry(n int) Future[T] {
	var zero T
	return func(ctx context.Context) (_ T, err error) {
		for i := 0; i < n; i++ {
			value, attemptErr := pf(ctx)
			switch {
			case attemptErr == nil:
				return value, nil
			case ers.IsTerminating(attemptErr):
				return zero, erc.Join(attemptErr, err)
			case ers.IsExpiredContext(attemptErr):
				return zero, erc.Join(attemptErr, err)
			case errors.Is(attemptErr, ers.ErrCurrentOpSkip):
				i--
				continue
			default:
				err = erc.Join(attemptErr, err)
			}
		}
		return zero, err
	}
}

type tuple[T, U any] struct {
	One T
	Two U
}

// TTL runs the future only one time per specified interval. The
// interval must me greater than 0.
func (pf Future[T]) TTL(dur time.Duration) Future[T] {
	resolver := erc.Must(internal.TTLExec[tuple[T, error]](dur))

	return func(ctx context.Context) (T, error) {
		out := resolver(func() (val tuple[T, error]) {
			val.One, val.Two = pf(ctx)
			return
		})
		return out.One, out.Two
	}
}

// PreHook configures an operation function to run before the returned
// future. If the pre-hook panics, it is converted to an error which
// is aggregated with the (potential) error from the future, and
// returned with the future's output.
func (pf Future[T]) PreHook(op Operation) Future[T] {
	return func(ctx context.Context) (out T, err error) {
		var ec erc.Collector
		ec.Push(op.WithRecover().Run(ctx))
		out, err = pf(ctx)
		ec.Push(err)
		return out, ec.Resolve()
	}
}

// PostHook appends a function to the execution of the future. If
// the function panics it is converted to an error and aggregated with
// the error of the future.
//
// Useful for calling context.CancelFunc, closers, or incrementing
// counters as necessary.
func (pf Future[T]) PostHook(op func()) Future[T] {
	return func(ctx context.Context) (o T, e error) {
		var ec erc.Collector
		o, e = pf(ctx)
		ec.Push(e)
		ec.WithRecover(op)
		e = ec.Resolve()
		return
	}
}

// WithoutErrors returns a Future function that wraps the root
// future and, after running the root future, and makes the error
// value of the future nil if the error returned is in the error
// list. The produced value in these cases is almost always the zero
// value for the type.
func (pf Future[T]) WithoutErrors(errs ...error) Future[T] {
	return pf.WithErrorFilter(erc.NewFilter().Without(errs...))
}

// WithErrorFilter passes the error of the root Future function with
// the erc.Filter.
func (pf Future[T]) WithErrorFilter(ef erc.Filter) Future[T] {
	return func(ctx context.Context) (out T, err error) { out, err = pf(ctx); return out, ef(err) }
}

// Filter creates a function that passes the output of the future to
// the filter function, which, if it returns true. is returned to the
// caller, otherwise the Future returns the zero value of type T and
// ers.ErrCurrentOpSkip error (e.g. continue), which streams and
// other future-consuming functions can respect.
func (pf Future[T]) Filter(fl func(T) bool) Future[T] {
	var zero T
	return func(ctx context.Context) (T, error) {
		val, err := pf(ctx)
		if err != nil {
			return zero, err
		}
		if !fl(val) {
			return zero, ers.ErrCurrentOpSkip
		}
		return val, nil
	}
}

// Read executes the future and returns the result.
func (pf Future[T]) Read(ctx context.Context) (T, error) { return pf(ctx) }
