package fun

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/tychoish/fun/internal"
)

// Worker represents a basic function used in worker pools and
// other similar situations
type Worker func(context.Context) error

// WorkerFuture constructs a worker from an error channel. The
// resulting worker blocks until an error is produced in the error
// channel, the error channel is closed, or the worker's context is
// canceled. If the channel is closed, the worker will return a nil
// error, and if the context is canceled, the worker will return a
// context error. In all other cases the work will propagate the error
// (or nil) recived from the channel.
func WorkerFuture(ch <-chan error) Worker {
	return func(ctx context.Context) error {
		if ch == nil {
			return nil
		}

		// val and err cannot both be non-nil at the same
		// time.
		val, err := BlockingReceive(ch).Read(ctx)
		switch {
		case errors.Is(err, io.EOF):
			return nil
		case err != nil:
			return err
		case val != nil:
			return val
		default:
			return nil
		}
	}
}

// Run is equivalent to calling the worker function directly, except
// the context passed to it is canceled when the worker function returns.
func (wf Worker) Run(ctx context.Context) error {
	wctx, cancel := context.WithCancel(ctx)
	defer cancel()
	return wf(wctx)
}

// Safe produces a worker function that converts the worker function's
// panics to errors.
func (wf Worker) Safe() Worker {
	return func(ctx context.Context) (err error) {
		defer func() { err = mergeWithRecover(err, recover()) }()
		return wf(ctx)
	}
}

// Block executes the worker function with a context that will never
// expire and returns the error. Use with caution
func (wf Worker) Block() error { return wf.Run(internal.BackgroundContext) }

// Observe runs the worker function, and observes the error (or nil
// response). Panics are converted to errors for both the worker
// function but not the observer function.
func (wf Worker) Observe(ctx context.Context, ob Observer[error]) {
	ob(wf.Safe()(ctx))
}

// Signal runs the worker function in a background goroutine and
// returns the error in an error channel, that returns when the
// worker function returns. If Signal is called with a canceled
// context the worker is still executed (with that context.)
//
// A value, possibly nil, is always sent through the channel, though
// the fun.Worker runs in a different go routine, a panic handler will
// convert panics to errors.
func (wf Worker) Signal(ctx context.Context) <-chan error {
	out := make(chan error)
	go func() { defer close(out); Blocking(out).Send().Ignore(ctx, wf.Safe()(ctx)) }()
	return out
}

// Future runs the worker function in a go routine and returns a
// new fun.Worker which will block for the context to expire or the
// background worker to complete, which returns the error from the
// background request.
//
// The underlying worker begins executing before future returns.
func (wf Worker) Future(ctx context.Context) Worker {
	out := wf.Signal(ctx)
	return WorkerFuture(out)
}

// Background starts the worker function in a go routine, passing the
// error to the provided observer function.
func (wf Worker) Background(ctx context.Context, ob Observer[error]) {
	go func() { ob(wf.Safe()(ctx)) }()
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

// Wait converts a worker function into a wait function,
// passing any error to the observer function. Only non-nil errors are
// observed.
func (wf Worker) Wait(ob Observer[error]) WaitFunc {
	return func(ctx context.Context) { wf.Observe(ctx, ob) }
}

// Must converts a Worker function into a wait function; however,
// if the worker produces an error Must converts the error into a
// panic.
func (wf Worker) Must() WaitFunc { return func(ctx context.Context) { InvariantMust(wf(ctx)) } }

// Ignore converts the worker into a WaitFunc that discards the error
// produced by the worker.
func (wf Worker) Ignore() WaitFunc { return func(ctx context.Context) { _ = wf(ctx) } }

// If returns a Worker function that runs only if the condition is
// true. The error is always nil if the condition is false. If-ed
// functions may be called more than once, and will run multiple
// times potentiall.y
func (wf Worker) If(cond bool) Worker { return wf.When(Wrapper(cond)) }

// When wraps a Worker function that will only run if the condition
// function returns true. If the condition is false the worker does
// not execute. The condition function is called in between every operation.
//
// When worker functions may be called more than once, and will run
// multiple times potentiall.
func (wf Worker) When(cond func() bool) Worker {
	return func(ctx context.Context) error {
		if cond() {
			return wf(ctx)
		}
		return nil
	}
}

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
func (wf Worker) Delay(dur time.Duration) Worker { return wf.Jitter(Wrapper(dur)) }

// Jitter wraps a Worker that runs the jitter function (jf) once
// before every execution of the resulting fucntion, and waits for the
// resulting duration before running the Worker.
//
// If the function produces a negative duration, there is no delay.
func (wf Worker) Jitter(jf func() time.Duration) Worker {
	return func(ctx context.Context) error {
		timer := time.NewTimer(internal.Max(0, jf()))
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
func (wf Worker) Lock() Worker {
	mtx := &sync.Mutex{}
	return func(ctx context.Context) error { mtx.Lock(); defer mtx.Unlock(); return wf(ctx) }
}

// Check runs the worker and returns true (ok) if there was no error,
// and false otherwise.
func (wf Worker) Check(ctx context.Context) bool { return wf(ctx) == nil }

// Chain melds two workers, calling the second worker if the first
// succeeds and the context hasn't been canceled.
func (wf Worker) Chain(next Worker) Worker {
	return func(ctx context.Context) error {
		if err := wf(ctx); err != nil {
			return err
		}
		return next.If(ctx.Err() == nil)(ctx)
	}
}
