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
	"github.com/tychoish/fun/intish"
)

// Producer is a function type that is a failrly common
// constructor. It's signature is used to create iterators, as a
// generator, and functions like a Future.
type Producer[T any] func(context.Context) (T, error)

// MakeProducer constructs a producer that wraps a similar
// function that does not take a context.
func MakeProducer[T any](fn func() (T, error)) Producer[T] {
	return func(context.Context) (T, error) { return fn() }
}

// ConsistentProducer constructs a wrapper around a similar function
// type that does not return an error or take a context. The resulting
// function will never error.
func ConsistentProducer[T any](fn func() T) Producer[T] {
	return func(context.Context) (T, error) { return fn(), nil }
}

// StaticProducer returns a producer function that always returns the
// provided values.
func StaticProducer[T any](val T, err error) Producer[T] {
	return func(context.Context) (T, error) { return val, err }
}

// ValueProducer returns a producer function that always returns the
// provided value, and a nill error.
func ValueProducer[T any](val T) Producer[T] {
	return StaticProducer(val, nil)
}

func CheckProducer[T any](op func() (T, bool)) Producer[T] {
	var zero T
	return func(ctx context.Context) (_ T, err error) {
		out, ok := op()
		if !ok {
			return zero, ft.Default(ctx.Err(), io.EOF)
		}
		return out, nil
	}
}

// Run executes the producer and returns the result
//
// Deprecated: Use the Resolve() helper.
func (pf Producer[T]) Run(ctx context.Context) (T, error) { return pf.Resolve(ctx) }

// Run executes the producer and returns the result
func (pf Producer[T]) Resolve(ctx context.Context) (T, error) { return pf(ctx) }

// Background constructs a worker that runs the provided Producer in a
// background thread and passes the produced value to the observe.
//
// The worker function's return value captures the procuder's error,
// and will block until the producer has completed.
func (pf Producer[T]) Background(ctx context.Context, of Handler[T]) Worker {
	return pf.Worker(of).Launch(ctx)
}

// Worker passes the produced value to an observer and returns a
// worker that runs the producer, calls the observer, and returns the
// error.
func (pf Producer[T]) Worker(of Handler[T]) Worker {
	return func(ctx context.Context) error { o, e := pf(ctx); of(o); return e }
}

// Operation produces a wait function, using two observers to handle the
// output of the Producer.
func (pf Producer[T]) Operation(of Handler[T], eo Handler[error]) Operation {
	return func(ctx context.Context) { o, e := pf(ctx); of(o); eo(e) }
}

// WithRecover returns a wrapped producer with a panic handler that converts
// any panic to an error.
func (pf Producer[T]) WithRecover() Producer[T] {
	return func(ctx context.Context) (_ T, err error) {
		defer func() { err = ers.Join(err, ers.ParsePanic(recover())) }()
		return pf(ctx)
	}
}

// Ignore runs the producer function and returns the value, ignoring
// the error.
func (pf Producer[T]) Ignore(ctx context.Context) Future[T] {
	return func() T { return ft.IgnoreSecond(pf(ctx)) }
}

// Join, on successive calls, runs the first producer until it
// returns an io.EOF error, and then returns the results of the second
// producer. If either producer returns another error (context
// cancelation or otherwise,) those errors are returned.
//
// When the second function returns io.EOF, all successive calls will
// return io.EOF.
func (pf Producer[T]) Join(next Producer[T]) Producer[T] {
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

		RETRY:
			for {
				out, err = pf(ctx)
				switch {
				case err == nil:
					return out, nil
				case errors.Is(err, ErrIteratorSkip):
					continue RETRY
				case !errors.Is(err, io.EOF):
					ferr = err
					stage.Store(firstFunctionErrored)
					return zero, err
				}
				break RETRY
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
				case errors.Is(err, ErrIteratorSkip):
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

// Must runs the producer returning the constructed value and panicing
// if the producer errors.
func (pf Producer[T]) Must(ctx context.Context) T { return ft.Must(pf(ctx)) }

// Block runs the producer with a context that will ever expire.
func (pf Producer[T]) Block() (T, error) { return pf(context.Background()) }

// Force combines the semantics of Must and Block: runs the producer
// with a context that never expires and panics in the case of an
// error.
func (pf Producer[T]) Force() T { return ft.Must(pf.Block()) }

// Check uses the error observer to consume the error from the
// Producer and returns a function that takes a context and returns a value.
func (pf Producer[T]) Check(ctx context.Context) (T, bool) { o, e := pf(ctx); return o, e == nil }
func (pf Producer[T]) CheckForce() (T, bool)               { return pf.Check(context.Background()) }

// Launch runs the producer in the background, when function is
// called, and returns a producer which, when called, blocks until the
// original producer returns.
func (pf Producer[T]) Launch(ctx context.Context) Producer[T] {
	out := make(chan T, 1)
	var err error
	go func() { defer close(out); o, e := pf.WithRecover().Run(ctx); err = e; out <- o }()

	return func(ctx context.Context) (T, error) {
		out, chErr := Blocking(out).Receive().Read(ctx)
		err = ers.Join(err, chErr)
		return out, err
	}
}

// Future creates a future function using the context provided and
// error observer to collect the error.
func (pf Producer[T]) Future(ctx context.Context, ob Handler[error]) Future[T] {
	return func() T { out, err := pf(ctx); ob(err); return out }
}

// Once returns a producer that only executes ones, and caches the
// return values, so that subsequent calls to the output producer will
// return the same values.
func (pf Producer[T]) Once() Producer[T] {
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

// Iterator creates an iterator that calls the Producer function once
// for every iteration, until it errors. Errors that are not context
// cancellation errors or io.EOF are propgated to the iterators Close
// method.
func (pf Producer[T]) Iterator() *Iterator[T] {
	return pf.IteratorWithErrorCollector(HF.ErrorCollector())
}

func (pf Producer[T]) IteratorWithErrorCollector(ec Handler[error], er Future[error]) *Iterator[T] {
	op, cancel := pf.WithCancel()
	iter := &Iterator[T]{operation: op}
	iter.closer.op = cancel
	iter.err.handler = ec
	iter.err.future = er
	return iter
}

// IteratorWithHook constructs an Iterator from the producer. The
// provided hook function will run during the Iterators Close()
// method.
func (pf Producer[T]) IteratorWithHook(hook func(*Iterator[T])) *Iterator[T] {
	iter := pf.Iterator()
	closer := iter.closer.op
	iter.closer.op = ft.Once(func() { hook(iter); closer() })
	return iter
}

// If returns a producer that will execute the root producer only if
// the cond value is true. Otherwise, If will return the zero value
// for T and a nil error.
func (pf Producer[T]) If(cond bool) Producer[T] { return pf.When(ft.Wrapper(cond)) }

// After will return a Producer that will block until the provided
// time is in the past, and then execute normally.
func (pf Producer[T]) After(ts time.Time) Producer[T] { return pf.Delay(time.Until(ts)) }

// Delay wraps a Producer in a function that will always wait for the
// specified duration before running.
//
// If the value is negative, then there is always zero delay.
func (pf Producer[T]) Delay(d time.Duration) Producer[T] { return pf.Jitter(ft.Wrapper(d)) }

// Jitter wraps a Producer that runs the jitter function (jf) once
// before every execution of the resulting function, and waits for the
// resulting duration before running the Producer.
//
// If the function produces a negative duration, there is no delay.
func (pf Producer[T]) Jitter(jf func() time.Duration) Producer[T] {
	return func(ctx context.Context) (out T, _ error) {
		timer := time.NewTimer(intish.Max(0, jf()))
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return out, ctx.Err()
		case <-timer.C:
			return pf(ctx)
		}
	}
}

// When constructs a producer that will call the cond upon every
// execution, and when true, will run and return the results of the
// root producer. Otherwise When will return the zero value of T and a
// nil error.
func (pf Producer[T]) When(cond func() bool) Producer[T] {
	return func(ctx context.Context) (out T, _ error) {
		if cond() {
			return pf(ctx)

		}
		return out, nil
	}
}

// Lock creates a producer that runs the root mutex as per normal, but
// under the protection of a mutex so that there's only one execution
// of the producer at a time.
func (pf Producer[T]) Lock() Producer[T] { return pf.WithLock(&sync.Mutex{}) }

// WithLock uses the provided mutex to protect the execution of the producer.
func (pf Producer[T]) WithLock(mtx sync.Locker) Producer[T] {
	return func(ctx context.Context) (T, error) {
		mtx.Lock()
		defer mtx.Unlock()
		return pf(ctx)
	}
}

// SendOne makes a Worker function that, as a future, calls the
// producer once and then passes the output, if there are no errors,
// to the processor function. Provides the inverse operation of
// Processor.ReadOne.
func (pf Producer[T]) SendOne(proc Processor[T]) Worker { return proc.ReadOne(pf) }

// SendAll provides a form of iteration, by construction a future
// (Worker) that consumes the values of the producer with the
// processor until either function returns an error. SendAll respects
// ErrIteratorSkip and io.EOF
func (pf Producer[T]) SendAll(proc Processor[T]) Worker { return proc.ReadAll(pf) }

// WithCancel creates a Producer and a cancel function which will
// terminate the context that the root Producer is running
// with. This context isn't canceled *unless* the cancel function is
// called (or the context passed to the Producer is canceled.)
func (pf Producer[T]) WithCancel() (Producer[T], context.CancelFunc) {
	var wctx context.Context
	var cancel context.CancelFunc
	once := &sync.Once{}

	return func(ctx context.Context) (out T, _ error) {
		once.Do(func() { wctx, cancel = context.WithCancel(ctx) })
		Invariant.IsFalse(wctx == nil, "must start the operation before calling cancel")
		return pf(wctx)
	}, func() { once.Do(func() {}); ft.SafeCall(cancel) }
}

// Limit runs the producer a specified number of times, and caches the
// result of the last execution and returns that value for any
// subsequent executions.
func (pf Producer[T]) Limit(in int) Producer[T] {
	resolver := limitExec[tuple[T, error]](in)

	return func(ctx context.Context) (T, error) {
		out := resolver(func() (val tuple[T, error]) {
			val.One, val.Two = pf(ctx)
			return
		})
		return out.One, out.Two
	}
}

// Retry constructs a worker function that takes runs the underlying
// producer until the error value is nil, or it encounters a
// terminating error (io.EOF, ers.ErrAbortCurrentOp, or context
// cancellation.) In all cases, unless the error value is nil
// (e.g. the retry succeeds)
//
// Context cancellation errors are returned to the caller, other
// terminating errors are not, with any other errors encountered
// during retries. ErrIteratorSkip is always ignored and not
// aggregated. All errors are discarded if the retry operation
// succeeds in the provided number of retries.
//
// Except for ErrIteratorSkip, which is ignored, all other errors are
// aggregated and returned to the caller only if the retry fails. It's
// possible to return a nil error and a zero value, if the producer
// only returned ErrIteratorSkip values.
func (pf Producer[T]) Retry(n int) Producer[T] {
	var zero T
	return func(ctx context.Context) (_ T, err error) {
		for i := 0; i < n; i++ {
			value, attemptErr := pf(ctx)
			switch {
			case attemptErr == nil:
				return value, nil
			case ers.IsTerminating(attemptErr):
				return zero, ers.Join(attemptErr, err)
			case errors.Is(attemptErr, ErrIteratorSkip):
				continue
			default:
				err = ers.Join(attemptErr, err)
			}

		}
		return zero, err
	}
}

// TTL runs the producer only one time per specified interval. The
// interval must me greater than 0.
func (pf Producer[T]) TTL(dur time.Duration) Producer[T] {
	resolver := ttlExec[tuple[T, error]](dur)

	return func(ctx context.Context) (T, error) {
		out := resolver(func() (val tuple[T, error]) {
			val.One, val.Two = pf(ctx)
			return
		})
		return out.One, out.Two
	}
}

// PreHook configures an operation function to run before the returned
// producer. If the pre-hook panics, it is converted to an error which
// is aggregated with the (potential) error from the producer, and
// returned with the producer's output.
func (pf Producer[T]) PreHook(op Operation) Producer[T] {
	return func(ctx context.Context) (out T, err error) {
		e := ers.Check(func() { op(ctx) })
		out, err = pf(ctx)
		return out, ers.Join(err, e)
	}
}

// PostHook appends a function to the execution of the producer. If
// the function panics it is converted to an error and aggregated with
// the error of the producer.
//
// Useful for calling context.CancelFunc, closers, or incrementing
// counters as necessary.
func (pf Producer[T]) PostHook(op func()) Producer[T] {
	return func(ctx context.Context) (o T, e error) {
		o, e = pf(ctx)
		e = ers.Join(ers.Check(op), e)
		return
	}
}

// WithoutErrors returns a Producer function that wraps the root
// producer and, after running the root producer, and makes the error
// value of the producer nil if the error returned is in the error
// list. The produced value in these cases is almost always the zero
// value for the type.
func (pf Producer[T]) WithoutErrors(errs ...error) Producer[T] {
	return pf.WithErrorFilter(ers.FilterExclude(errs...))
}

// WithErrorFilter passes the error of the root Producer function with
// the ers.Filter.
func (pf Producer[T]) WithErrorFilter(ef ers.Filter) Producer[T] {
	return func(ctx context.Context) (out T, err error) { out, err = pf(ctx); return out, ef(err) }
}

// Filter creates a function that passes the output of the producer to
// the filter function, which, if it returns true. is returned to the
// caller, otherwise the Producer returns the zero value of type T and
// ers.ErrCurrentOpSkip error (e.g. continue), which iterators and
// other producer-consuming functions can respect.
func (pf Producer[T]) Filter(fl func(T) bool) Producer[T] {
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

// ParallelGenerate creates an iterator using a generator pattern which
// produces items until the generator function returns
// io.EOF, or the context (passed to the first call to
// Next()) is canceled. Parallel operation, continue on
// error/continue-on-panic semantics are available and share
// configuration with the ParallelProcess and Map operations.
func (pf Producer[T]) GenerateParallel(
	optp ...OptionProvider[*WorkerGroupConf],
) *Iterator[T] {
	opts := &WorkerGroupConf{}
	initErr := JoinOptionProviders(optp...).Apply(opts)

	pipe := Blocking(make(chan T, opts.NumWorkers*2+1))

	init := Operation(func(ctx context.Context) {
		wctx, cancel := context.WithCancel(ctx)
		wg := &WaitGroup{}

		pf = pf.WithRecover()
		var zero T
		pipe.Processor().
			ReadAll(func(ctx context.Context) (T, error) {
				value, err := pf(ctx)
				if err != nil {
					if opts.CanContinueOnError(err) {
						return zero, ErrIteratorSkip
					}

					return zero, io.EOF
				}
				return value, nil
			}).
			Operation(func(err error) {
				ft.WhenCall(ers.Is(err, io.EOF, ers.ErrCurrentOpAbort), cancel)
			}).
			StartGroup(wctx, wg, opts.NumWorkers)

		wg.Operation().PostHook(func() { cancel(); pipe.Close() }).Background(ctx)
	}).Once()

	iter := pipe.Receive().Producer().PreHook(init).Iterator()

	ft.WhenCall(opts.ErrorHandler == nil, func() { opts.ErrorHandler = iter.ErrorHandler().Lock() })

	opts.ErrorHandler(initErr)

	ft.WhenCall(initErr != nil, pipe.Close)

	return iter
}
