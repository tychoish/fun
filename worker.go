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

// Worker represents a basic function used in worker pools and
// other similar situations
type Worker func(context.Context) error

// MakeWorker converts a non-context worker function into a worker for
// compatibility with tooling.
func MakeWorker(fn func() error) Worker { return func(context.Context) error { return fn() } }

// WorkerFuture constructs a worker from an error channel. The
// resulting worker blocks until an error is produced in the error
// channel, the error channel is closed, or the worker's context is
// canceled. If the channel is closed, the worker will return a nil
// error, and if the context is canceled, the worker will return a
// context error. In all other cases the work will propagate the error
// (or nil) received from the channel.
//
// You can call the resulting worker function more than once: if there
// are multiple errors produced or passed to the channel, they will be
// propogated; however, after the channel is closed subsequent calls
// to the worker function will return nil.
func WorkerFuture(ch <-chan error) Worker {
	pipe := BlockingReceive(ch)
	return func(ctx context.Context) error {
		if ch == nil || pipe.ch == nil {
			return nil
		}

		// val and err cannot both be non-nil at the same
		// time.
		val, err := pipe.Read(ctx)
		switch {
		case errors.Is(err, io.EOF):
			pipe.ch = nil
			return nil
		case val != nil:
			// actual error
			return val
		case err != nil:
			// context error?
			return err
		default:
			return nil
		}
	}
}

// Pipe sends the output of a producer into the processor as if it
// were a pipe. As a Worker this operation is delayed until the worker
// is called.
//
// If both operations succeed, then the worker will return nil. If
// either function returns an error, it is cached, and successive
// calls to the worker will return the same error. The only limitation
// is that there is no way to distinguish between an error encountered
// by the Producer and one by the processor.
//
// If the producer returns ErrIteratorSkip, it will be called again.
func Pipe[T any](from Producer[T], to Processor[T]) Worker {
	hasErrored := &atomic.Bool{}
	var (
		err error
		val T
	)

	return func(ctx context.Context) error {
		if hasErrored.Load() {
			return err
		}

	RETRY:
		for {
			val, err = from(ctx)
			switch {
			case err == nil:
				break RETRY
			case errors.Is(err, ErrIteratorSkip):
				continue
			default:
				hasErrored.Store(true)
				return err
			}
		}

		if err = to(ctx, val); err != nil {
			hasErrored.Store(true)
			return err
		}
		return nil
	}

}

// Run is equivalent to calling the worker function directly.
func (wf Worker) Run(ctx context.Context) error { return wf(ctx) }

// WithRecover produces a worker function that converts the worker function's
// panics to errors.
func (wf Worker) WithRecover() Worker {
	return func(ctx context.Context) (err error) {
		defer func() { err = ers.Join(err, ers.ParsePanic(recover())) }()
		return wf(ctx)
	}
}

// Observe runs the worker function, and observes the error (or nil
// response). Panics are not caught.
func (wf Worker) Observe(ctx context.Context, ob Handler[error]) { ob(wf.Run(ctx)) }

// Signal runs the worker function in a background goroutine and
// returns the error in an error channel, that returns when the
// worker function returns. If Signal is called with a canceled
// context the worker is still executed (with that context.)
//
// A value, possibly nil, is always sent through the channel. Panics
// are not caught or handled.
func (wf Worker) Signal(ctx context.Context) <-chan error {
	out := Blocking(make(chan error))
	go func() { defer out.Close(); out.Send().Ignore(ctx, wf.Run(ctx)) }()
	return out.Channel()
}

// Launch runs the worker function in a go routine and returns a new
// fun.Worker which will block for the context to expire or the
// background worker to completes, which returns the error
// from the background request.
//
// The underlying worker begins executing before future returns.
func (wf Worker) Launch(ctx context.Context) Worker {
	out := wf.Signal(ctx)
	return WorkerFuture(out)
}

// Background starts the worker function in a go routine, passing the
// error to the provided observer function.
func (wf Worker) Background(ctx context.Context, ob Handler[error]) Operation {
	return wf.Launch(ctx).Operation(ob)
}

// Once wraps the Worker in a function that will execute only
// once. The return value (error) is cached, and can be accessed many
// times without re-running the worker.
func (wf Worker) Once() Worker {
	once := &sync.Once{}
	var err error
	return func(ctx context.Context) error {
		once.Do(func() { err = wf(ctx) })
		return err
	}
}

// Operation converts a worker function into a wait function,
// passing any error to the observer function. Only non-nil errors are
// observed.
func (wf Worker) Operation(ob Handler[error]) Operation {
	return func(ctx context.Context) { wf.Observe(ctx, ob) }
}

// Block executes the worker function with a context that will never
// expire and returns the error. Use with caution.
//
// Deprecated: use Wait() instead.
func (wf Worker) Block() error { return wf.Wait() }

// Wait runs the worker with a background context and returns its
// error.
func (wf Worker) Wait() error { return wf.Run(context.Background()) }

// Must converts a Worker function into a wait function; however,
// if the worker produces an error Must converts the error into a
// panic.
func (wf Worker) Must() Operation { return func(ctx context.Context) { Invariant.Must(wf(ctx)) } }

// Ignore converts the worker into a Operation that discards the error
// produced by the worker.
func (wf Worker) Ignore() Operation { return func(ctx context.Context) { ers.Ignore(wf(ctx)) } }

// If returns a Worker function that runs only if the condition is
// true. The error is always nil if the condition is false. If-ed
// functions may be called more than once, and will run multiple
// times potentiall.y
func (wf Worker) If(cond bool) Worker { return wf.When(ft.Wrapper(cond)) }

// When wraps a Worker function that will only run if the condition
// function returns true. If the condition is false the worker does
// not execute. The condition function is called in between every operation.
//
// When worker functions may be called more than once, and will run
// multiple times potentially.
func (wf Worker) When(cond func() bool) Worker {
	return func(ctx context.Context) error { return ft.WhenDo(cond(), wf.futureOp(ctx)) }
}
func (wf Worker) futureOp(ctx context.Context) func() error { return func() error { return wf(ctx) } }

// After returns a Worker that blocks until the timestamp provided is
// in the past. Additional calls to this worker will run
// immediately. If the timestamp is in the past the resulting worker
// will run immediately.
func (wf Worker) After(ts time.Time) Worker {
	return wf.Jitter(func() time.Duration { return time.Until(ts) })
}

// Delay wraps a Worker in a function that will always wait for the
// specified duration before running.
//
// If the value is negative, then there is always zero delay.
func (wf Worker) Delay(dur time.Duration) Worker { return wf.Jitter(ft.Wrapper(dur)) }

// Jitter wraps a Worker that runs the jitter function (jf) once
// before every execution of the resulting function, and waits for the
// resulting duration before running the Worker.
//
// If the function produces a negative duration, there is no delay.
func (wf Worker) Jitter(jf func() time.Duration) Worker {
	return func(ctx context.Context) error {
		timer := time.NewTimer(max(0, jf()))
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return wf(ctx)
		}
	}
}

// Limit produces a worker than runs exactly n times. Each execution
// is isolated from every other, but once the limit is exceeded, the
// result of the *last* worker to execute is cached concurrent access
// to that value is possible.
func (wf Worker) Limit(n int) Worker {
	resolver := limitExec[error](n)

	return func(ctx context.Context) error { return resolver(func() error { return wf(ctx) }) }
}

// TTL produces a worker that will only run once during every
// specified duration, when called more than once. During the interval
// between calls, the previous error is returned. While each execution
// of the root worker is protected by a mutex, the resulting worker
// can be used in parallel during the intervals between calls.
func (wf Worker) TTL(dur time.Duration) Worker {
	resolver := ttlExec[error](dur)
	return func(ctx context.Context) error { return resolver(func() error { return wf(ctx) }) }
}

// Lock produces a Worker that will be executed within the scope of a
// (managed) mutex.
func (wf Worker) Lock() Worker { return wf.WithLock(&sync.Mutex{}) }

// Lock produces a Worker that will be executed within the scope of
// the provided mutex.
func (wf Worker) WithLock(mtx sync.Locker) Worker {
	return func(ctx context.Context) error { mtx.Lock(); defer mtx.Unlock(); return wf(ctx) }
}

// Check runs the worker and returns true (ok) if there was no error,
// and false otherwise.
func (wf Worker) Check(ctx context.Context) bool { return wf(ctx) == nil }

// While runs the Worker in a continuous while loop, returning only if
// the underlying worker returns an error or if the context is
// cancled.
func (wf Worker) While() Worker {
	return func(ctx context.Context) error {
		for {
			if err, cerr := wf(ctx), ctx.Err(); err != nil || cerr != nil {
				return ft.Default(err, cerr)
			}
		}
	}
}

// Interval runs the worker with a timer that resets to the provided
// duration. The worker runs immediately, and then the time is reset
// to the specified interval after the base worker has. Which is to
// say that the runtime of the worker's operation is effectively added
// to the interval.
//
// The interval worker will run until the context is canceled or the
// worker returns an error.
func (wf Worker) Interval(dur time.Duration) Worker {
	return func(ctx context.Context) error {
		timer := time.NewTimer(0)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				if err := wf(ctx); err != nil {
					return err
				}
				timer.Reset(dur)
			}
		}
	}
}

// Join combines a sequence of workers, calling the workers in order,
// as long as there is no error and the context does not
// expire. Context expiration errors are not propagated.
func (wf Worker) Join(wfs ...Worker) Worker {
	for idx := range wfs {
		wf = wf.merge(wfs[idx])
	}

	return wf
}

func (wf Worker) merge(next Worker) Worker {
	return func(ctx context.Context) error {
		if err := wf(ctx); err != nil {
			return err
		}
		return next.If(ctx.Err() == nil).Run(ctx)
	}
}

// WithCancel creates a Worker and a cancel function which will
// terminate the context that the root Worker is running
// with. This context isn't canceled *unless* the cancel function is
// called (or the context passed to the Worker is canceled.)
func (wf Worker) WithCancel() (Worker, context.CancelFunc) {
	var wctx context.Context
	var cancel context.CancelFunc
	once := &sync.Once{}

	return func(ctx context.Context) error {
		once.Do(func() { wctx, cancel = context.WithCancel(ctx) })
		Invariant.IsFalse(wctx == nil, "must start the operation before calling cancel")
		return wf(wctx)
	}, func() { once.Do(func() {}); ft.SafeCall(cancel) }
}

// PreHook returns a Worker that runs an operatio unconditionally
// before running the underlying worker. If the hook function panics
// it is converted to an error and aggregated with the worker's
// error.
func (wf Worker) PreHook(op Operation) Worker {
	return func(ctx context.Context) error { return ers.Join(ers.WithRecoverCall(func() { op(ctx) }), wf(ctx)) }
}

// PostHook runs hook operation  after the worker function
// completes. If the hook panics it is converted to an error and
// aggregated with workers's error.
func (wf Worker) PostHook(op func()) Worker {
	return func(ctx context.Context) error { return ers.Join(ft.Flip(wf(ctx), ers.WithRecoverCall(op))) }
}

// WithErrorCheck takes an error future, and checks it before
// executing the worker function. If the error future returns an
// error (any error), the worker propagates that error, rather than
// running the underying producer. Useful for injecting an abort into
// an existing pipleine or chain.
//
// The error future is called before running the underlying worker,
// to short circuit the operation, and also a second time when
// worker has returned in case an error has occurred during the
// operation of the worker.
func (wf Worker) WithErrorCheck(ef Future[error]) Worker {
	return func(ctx context.Context) error {
		if err := ef(); err != nil {
			return err
		}
		return ers.Join(wf.Run(ctx), ef())
	}
}

// StartGroup starts n copies of the worker operation and returns a
// future/worker that returns the aggregated errors from all workers
//
// The operation is fundamentally continue-on-error. To get
// abort-on-error semantics, use the Filter() method on the input
// worker, that cancels the context on when it sees an error.
func (wf Worker) StartGroup(ctx context.Context, n int) Worker {
	wg := &WaitGroup{}

	eh, ep := HF.ErrorCollector()

	wf.Operation(eh).StartGroup(ctx, wg, n)

	return func(ctx context.Context) (err error) { wg.Wait(ctx); return ep() }
}

// Group makes a worker that runs n copies of the underlying worker,
// in different go routines and aggregates their output. Work does not
// start until the resulting worker is called
func (wf Worker) Group(n int) Worker {
	return func(ctx context.Context) error { return wf.StartGroup(ctx, n).Run(ctx) }
}

// Filter wraps the worker with a Worker that passes the output of the
// root Worker's error and returns the output of the filter.
//
// The ers package provides a number of filter implementations but any
// function in the following form works:
//
//	func(error) error
func (wf Worker) WithErrorFilter(ef ers.Filter) Worker {
	return func(ctx context.Context) error { return ef(wf(ctx)) }
}

// WithoutErrors returns a worker that will return nil if the error
// returned by the worker is one of the errors passed to
// WithoutErrors.
func (wf Worker) WithoutErrors(errs ...error) Worker {
	filter := ers.FilterExclude(errs...)
	return func(ctx context.Context) error { return filter(wf(ctx)) }
}

// Retry constructs a worker function that takes runs the underlying
// worker until the return value is nil, or it encounters a
// terminating error (io.EOF, ers.ErrAbortCurrentOp, or context
// cancellation.) Context cancellation errors are returned to the
// caller with any other errors encountered in previous retries, other
// terminating errors are not. All errors are discarded if the retry
// operation succeeds in the provided number of retries.
//
// Except for ErrIteratorSkip, which is ignored, all other errors are
// aggregated and returned to the caller only if the retry fails.
func (wf Worker) Retry(n int) Worker {
	return func(ctx context.Context) (err error) {
		for i := 0; i < n; i++ {
			attemptErr := wf(ctx)
			switch {
			case attemptErr == nil:
				return nil
			case ers.IsExpiredContext(attemptErr):
				return ers.Join(attemptErr, err)
			case errors.Is(attemptErr, ErrIteratorSkip):
				continue
			case ers.IsTerminating(attemptErr):
				return nil
			default:
				err = ers.Join(attemptErr, err)
			}
		}
		return err
	}
}
