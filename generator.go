package fun

import (
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/fn"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/internal"
)

// Generator is a function type that is a failrly common
// constructor. It's signature is used to create iterators/streams, as
// a generator, and functions like a Future.
type Generator[T any] func(context.Context) (T, error)

// MakeGenerator constructs a generator that wraps a similar
// function that does not take a context.
func MakeGenerator[T any](fn func() (T, error)) Generator[T] {
	return func(context.Context) (T, error) { return fn() }
}

// NewGenerator returns a generator as a convenience function to avoid
// the extra cast when creating new function objects.
func NewGenerator[T any](fn func(ctx context.Context) (T, error)) Generator[T] { return fn }

// StaticGenerator returns a generator function that always returns the
// provided values.
func StaticGenerator[T any](val T, err error) Generator[T] {
	return func(context.Context) (T, error) { return val, err }
}

// ValueGenerator returns a generator function that always returns the
// provided value, and a nill error.
func ValueGenerator[T any](val T) Generator[T] { return StaticGenerator(val, nil) }

// CheckedGenerator wraps a function object that uses the second ("OK")
// value to indicate that no more values will be produced. Errors
// returned from the resulting produce are always either the context
// cancellation error or io.EOF.
func CheckedGenerator[T any](op func() (T, bool)) Generator[T] {
	return func(ctx context.Context) (zero T, _ error) {
		out, ok := op()
		if !ok {
			return zero, ft.Default(ctx.Err(), io.EOF)
		}
		return out, nil
	}
}

// PtrGenerator uses a function that returns a pointer to a value and
// converts that into a generator that de-references and returns
// non-nil values of the pointer, and returns EOF for nil values of
// the pointer.
func PtrGenerator[T any](fn func() *T) Generator[T] {
	return CheckedGenerator(func() (zero T, _ bool) {
		if val := fn(); val != nil {
			return *val, true
		}
		return zero, false
	})
}

// FutureGenerator creates a generator for the fn.Future
// function. The underlying Future's panics are converted to errors.
func FutureGenerator[T any](f fn.Future[T]) Generator[T] { return MakeGenerator(f.Safe()) }

// Background constructs a worker that runs the provided Generator in a
// background thread and passes the produced value to the observe.
//
// The worker function's return value captures the procuder's error,
// and will block until the generator has completed.
func (pf Generator[T]) Background(ctx context.Context, of fn.Handler[T]) Worker {
	return pf.Worker(of).Launch(ctx)
}

// Worker passes the produced value to an observer and returns a
// worker that runs the generator, calls the observer, and returns the
// error.
func (pf Generator[T]) Worker(of fn.Handler[T]) Worker {
	return func(ctx context.Context) error { o, e := pf(ctx); of(o); return e }
}

// Operation produces a wait function, using two observers to handle the
// output of the Generator.
func (pf Generator[T]) Operation(of fn.Handler[T], eo fn.Handler[error]) Operation {
	return func(ctx context.Context) { o, e := pf(ctx); of(o); eo(e) }
}

// WithRecover returns a wrapped generator with a panic handler that converts
// any panic to an error.
func (pf Generator[T]) WithRecover() Generator[T] {
	return func(ctx context.Context) (_ T, err error) {
		defer func() { err = ers.Join(err, ers.ParsePanic(recover())) }()
		return pf(ctx)
	}
}

// Join, on successive calls, runs the first generator until it
// returns an io.EOF error, and then returns the results of the second
// generator. If either generator returns another error (context
// cancelation or otherwise,) those errors are returned.
//
// When the second function returns io.EOF, all successive calls will
// return io.EOF.
func (pf Generator[T]) Join(next Generator[T]) Generator[T] {
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
				case errors.Is(err, ErrStreamContinue):
					continue RETRY_FIRST
				case !errors.Is(err, io.EOF):
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
				case errors.Is(err, ErrStreamContinue):
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
func (pf Generator[T]) Future(ctx context.Context, ob fn.Handler[error]) fn.Future[T] {
	return func() T { out, err := pf(ctx); ob(err); return out }
}

func (pf Generator[T]) Capture() fn.Future[T] { return pf.Force() }

// Ignore creates a future that runs the generator and returns
// the value, ignoring the error.
func (pf Generator[T]) Ignore(ctx context.Context) fn.Future[T] {
	return func() T { return ft.IgnoreSecond(pf(ctx)) }
}

// Must returns a future that resolves the generator returning the
// constructed value and panicing if the generator errors.
func (pf Generator[T]) Must(ctx context.Context) fn.Future[T] {
	return func() T { return ft.Must(pf.Send(ctx)) }
}

// Force combines the semantics of Must and Wait as a future: when the
// future is resolved, the generator executes with a context that never
// expires and panics in the case of an error.
func (pf Generator[T]) Force() fn.Future[T] { return func() T { return ft.IgnoreSecond(pf.Wait()) } }

// Wait runs the generator with a context that will ever expire.
func (pf Generator[T]) Wait() (T, error) { return pf(context.Background()) }

// Check converts the error into a boolean, with true indicating
// success and false indicating (but not propagating it.).
func (pf Generator[T]) Check(ctx context.Context) (T, bool) { o, e := pf(ctx); return o, e == nil }

// WithErrorCheck takes an error future, and checks it before
// executing the generator function. If the error future returns an
// error (any error), the generator propagates that error, rather than
// running the underying generator. Useful for injecting an abort into
// an existing pipleine or chain.
func (pf Generator[T]) WithErrorCheck(ef fn.Future[error]) Generator[T] {
	return func(ctx context.Context) (zero T, _ error) {
		if err := ef(); err != nil {
			return zero, err
		}

		out, err := pf.Send(ctx)
		if err = ers.Join(err, ef()); err != nil {
			return zero, err
		}

		return out, nil
	}
}

// Once returns a generator that only executes ones, and caches the
// return values, so that subsequent calls to the output generator will
// return the same values.
func (pf Generator[T]) Once() Generator[T] {
	var (
		out T
		err error
	)

	once := &sync.Once{}

	return func(ctx context.Context) (T, error) {
		once.Do(func() { out, err = pf(ctx) })
		return out, err
	}
}

// If returns a generator that will execute the root generator only if
// the cond value is true. Otherwise, If will return the zero value
// for T and a nil error.
func (pf Generator[T]) If(cond bool) Generator[T] { return pf.When(ft.Wrap(cond)) }

// After will return a Generator that will block until the provided
// time is in the past, and then execute normally.
func (pf Generator[T]) After(ts time.Time) Generator[T] { return pf.Delay(time.Until(ts)) }

// Delay wraps a Generator in a function that will always wait for the
// specified duration before running.
//
// If the value is negative, then there is always zero delay.
func (pf Generator[T]) Delay(d time.Duration) Generator[T] { return pf.Jitter(ft.Wrap(d)) }

// Jitter wraps a Generator that runs the jitter function (jf) once
// before every execution of the resulting function, and waits for the
// resulting duration before running the Generator.
//
// If the function produces a negative duration, there is no delay.
func (pf Generator[T]) Jitter(jf func() time.Duration) Generator[T] {
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

// When constructs a generator that will call the cond upon every
// execution, and when true, will run and return the results of the
// root generator. Otherwise When will return the zero value of T and a
// nil error.
func (pf Generator[T]) When(cond func() bool) Generator[T] {
	return func(ctx context.Context) (out T, _ error) {
		if cond() {
			return pf(ctx)

		}
		return out, nil
	}
}

// Lock creates a generator that runs the root mutex as per normal, but
// under the protection of a mutex so that there's only one execution
// of the generator at a time.
func (pf Generator[T]) Lock() Generator[T] { return pf.WithLock(&sync.Mutex{}) }

// WithLock uses the provided mutex to protect the execution of the generator.
func (pf Generator[T]) WithLock(mtx *sync.Mutex) Generator[T] {
	return func(ctx context.Context) (T, error) { defer internal.With(internal.Lock(mtx)); return pf(ctx) }
}

// WithLocker uses the provided mutex to protect the execution of the generator.
func (pf Generator[T]) WithLocker(mtx sync.Locker) Generator[T] {
	return func(ctx context.Context) (T, error) { defer internal.WithL(internal.LockL(mtx)); return pf(ctx) }
}

// Send executes the generator and returns the result.
func (pf Generator[T]) Send(ctx context.Context) (T, error) { return pf(ctx) }

// ReadOne makes a Worker function that, as a future, calls the
// generator once and then passes the output, if there are no errors,
// to the processor function. Provides the inverse operation of
// Handler.ReadOne.
func (pf Generator[T]) ReadOne(proc Handler[T]) Worker {
	return func(ctx context.Context) error {
		val, err := pf.Send(ctx)
		if err != nil {
			return err
		}
		return proc(ctx, val)
	}
}

// ReadAll provides a form of iteration, by construction a future
// (Worker) that consumes the values of the generator with the
// processor until either function returns an error. ReadAll respects
// ErrStreamContinue and io.EOF.
func (pf Generator[T]) ReadAll2(proc Handler[T]) Worker { return pf.Stream().ReadAll(proc) }

// ReadAll provides a form of iteration, by construction a future
// (Worker) that consumes the values of the generator with the
// processor until either function returns an error. ReadAll respects
// ErrStreamContinue and io.EOF.
func (pf Generator[T]) ReadAll(proc fn.Handler[T]) Worker {
	return pf.Stream().ReadAll(func(_ context.Context, in T) error { proc(in); return nil })
}

// WithCancel creates a Generator and a cancel function which will
// terminate the context that the root Generator is running
// with. This context isn't canceled *unless* the cancel function is
// called (or the context passed to the Generator is canceled.)
func (pf Generator[T]) WithCancel() (Generator[T], context.CancelFunc) {
	var wctx context.Context
	var cancel context.CancelFunc
	once := &sync.Once{}

	return func(ctx context.Context) (out T, _ error) {
		once.Do(func() { wctx, cancel = context.WithCancel(ctx) })
		Invariant.IsFalse(wctx == nil, "must start the operation before calling cancel")
		return pf(wctx)
	}, func() { once.Do(func() {}); ft.SafeCall(cancel) }
}

// internal function for use in generators: the idea is to let the
// inner generator produce results until it errors and then add in the
// error from the future.
func (pf Generator[T]) wrapErrorsWithErrorFuture(ef fn.Future[error]) Generator[T] {
	return func(ctx context.Context) (zero T, err error) {
		// just call the generator once: if there's no error,
		// return the result, because otherwise we're throwing
		// away work.
		out, err := pf(ctx)
		if err == nil {
			return out, nil
		}

		// Now there was an error,

		// first error is the error produced by the future, p
		// error is the second error, which can only be a
		// panic encountered by the error future.
		ferr, perr := ef.RecoverPanic()

		if ers.IsTerminating(err) && ferr != nil {
			// if there's a real substantive error,
			// encountered when there was an terminating
			// error, then let's just return that.
			return zero, ers.Join(ferr, perr)
		}

		// otherwise, merge everything (potentially)
		return zero, ers.Join(err, ferr, perr)
	}
}

// Limit runs the generator a specified number of times, and caches the
// result of the last execution and returns that value for any
// subsequent executions.
func (pf Generator[T]) Limit(in int) Generator[T] {
	resolver := ft.Must(internal.LimitExec[tuple[T, error]](in))

	return func(ctx context.Context) (T, error) {
		out := resolver(func() (val tuple[T, error]) {
			val.One, val.Two = pf(ctx)
			return
		})
		return out.One, out.Two
	}
}

// Retry constructs a worker function that takes runs the underlying
// generator until the error value is nil, or it encounters a
// terminating error (io.EOF, ers.ErrAbortCurrentOp, or context
// cancellation.) In all cases, unless the error value is nil
// (e.g. the retry succeeds)
//
// Context cancellation errors are returned to the caller, other
// terminating errors are not, with any other errors encountered
// during retries. ErrStreamContinue is always ignored and not
// aggregated. All errors are discarded if the retry operation
// succeeds in the provided number of retries.
//
// Except for ErrStreamContinue, which is ignored, all other errors are
// aggregated and returned to the caller only if the retry fails. It's
// possible to return a nil error and a zero value, if the generator
// only returned ErrStreamContinue values.
func (pf Generator[T]) Retry(n int) Generator[T] {
	var zero T
	return func(ctx context.Context) (_ T, err error) {
		for i := 0; i < n; i++ {
			value, attemptErr := pf(ctx)
			switch {
			case attemptErr == nil:
				return value, nil
			case ers.IsTerminating(attemptErr):
				return zero, ers.Join(attemptErr, err)
			case ers.IsExpiredContext(attemptErr):
				return zero, ers.Join(attemptErr, err)
			case errors.Is(attemptErr, ErrStreamContinue):
				i--
				continue
			default:
				err = ers.Join(attemptErr, err)
			}

		}
		return zero, err
	}
}

type tuple[T, U any] struct {
	One T
	Two U
}

// TTL runs the generator only one time per specified interval. The
// interval must me greater than 0.
func (pf Generator[T]) TTL(dur time.Duration) Generator[T] {
	resolver := ft.Must(internal.TTLExec[tuple[T, error]](dur))

	return func(ctx context.Context) (T, error) {
		out := resolver(func() (val tuple[T, error]) {
			val.One, val.Two = pf(ctx)
			return
		})
		return out.One, out.Two
	}
}

// PreHook configures an operation function to run before the returned
// generator. If the pre-hook panics, it is converted to an error which
// is aggregated with the (potential) error from the generator, and
// returned with the generator's output.
func (pf Generator[T]) PreHook(op Operation) Generator[T] {
	return func(ctx context.Context) (out T, err error) {
		e := ers.WithRecoverCall(func() { op(ctx) })
		out, err = pf(ctx)
		return out, ers.Join(err, e)
	}
}

// PostHook appends a function to the execution of the generator. If
// the function panics it is converted to an error and aggregated with
// the error of the generator.
//
// Useful for calling context.CancelFunc, closers, or incrementing
// counters as necessary.
func (pf Generator[T]) PostHook(op func()) Generator[T] {
	return func(ctx context.Context) (o T, e error) {
		o, e = pf(ctx)
		e = ers.Join(ers.WithRecoverCall(op), e)
		return
	}
}

// WithoutErrors returns a Generator function that wraps the root
// generator and, after running the root generator, and makes the error
// value of the generator nil if the error returned is in the error
// list. The produced value in these cases is almost always the zero
// value for the type.
func (pf Generator[T]) WithoutErrors(errs ...error) Generator[T] {
	return pf.WithErrorFilter(ers.FilterExclude(errs...))
}

// WithErrorFilter passes the error of the root Generator function with
// the ers.Filter.
func (pf Generator[T]) WithErrorFilter(ef ers.Filter) Generator[T] {
	return func(ctx context.Context) (out T, err error) { out, err = pf(ctx); return out, ef(err) }
}

// Filter creates a function that passes the output of the generator to
// the filter function, which, if it returns true. is returned to the
// caller, otherwise the Generator returns the zero value of type T and
// ers.ErrCurrentOpSkip error (e.g. continue), which streams and
// other generator-consuming functions can respect.
func (pf Generator[T]) Filter(fl func(T) bool) Generator[T] {
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

// TODO: the stream method is the only dependency of the generator
// type on Stream. Should probably deprecate to avoid accidental

// Stream creates a stream that calls the Generator function once
// for every iteration, until it errors. Errors that are not context
// cancellation errors or io.EOF are propgated to the stream's Close
// method.
func (pf Generator[T]) Stream() *Stream[T] { return MakeStream(pf) }

func (pf Generator[T]) WithErrorHandler(handler fn.Handler[error], resolver fn.Future[error]) Generator[T] {
	Invariant.Ok(handler != nil && resolver != nil, "must cal WithErrorHandler with non-nil operators")
	return func(ctx context.Context) (T, error) {
		out, err := pf(ctx)
		handler(err)
		return out, resolver()
	}
}

// Parallel returns a wrapped generator that produces items until the
// generator function returns io.EOF, or the context. Parallel
// operation, continue on error/continue-on-panic semantics are
// available and share configuration with the ParallelProcess and Map
// operations.
//
// You must specify a number of workers in the options greater than
// one to get parallel operation. Otherwise, there is only one worker.
//
// The operation returns results from a buffer that can hold a number
// of items equal to the number of workers. Buffered items may not be
// returned to the caller in the case of early termination.
func (pf Generator[T]) Parallel(
	optp ...OptionProvider[*WorkerGroupConf],
) Generator[T] {
	opts := &WorkerGroupConf{}

	if err := JoinOptionProviders(optp...).Apply(opts); err != nil {
		return MakeGenerator(func() (zero T, _ error) { return zero, err })
	}

	if opts.ErrorHandler == nil {
		opts.ErrorHandler, opts.ErrorResolver = MAKE.ErrorCollector()
	}

	pipe := Blocking(make(chan T, opts.NumWorkers+1))
	pf = pf.WithRecover()

	return pipe.Receive().
		Generator().
		PreHook(Operation(func(ctx context.Context) {
			wctx, cancel := context.WithCancel(ctx)

			// this might cause us to propagate an error (and abort)
			// before the pipe is empty.

			pipe.Handler().ReadAll(func(ctx context.Context) (zero T, _ error) {
				value, err := pf.Send(ctx)
				if err != nil {
					if opts.CanContinueOnError(err) {
						return zero, ErrStreamContinue
					}
					return zero, io.EOF
				}
				return value, nil
			}).Operation(func(err error) { ft.WhenCall(ers.IsTerminating(err), cancel) }).
				StartGroup(wctx, opts.NumWorkers).
				PostHook(cancel).
				PostHook(pipe.Close).
				Background(ctx)
		}).Once()).
		wrapErrorsWithErrorFuture(opts.ErrorResolver)
}
