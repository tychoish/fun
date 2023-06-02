package fun

import (
	"context"
	"sync"
	"time"

	"github.com/tychoish/fun/internal"
)

// Worker represents a basic function used in worker pools and
// other similar situations
type Worker func(context.Context) error

// ObserveWorker has the same semantics as Observe, except that the
// operation is wrapped in a WaitFunc, and executed when the WaitFunc
// is called.
func ObserveWorker[T any](iter Iterator[T], fn Observer[T]) Worker {
	return func(ctx context.Context) error { return Observe(ctx, iter, fn) }
}

// Run is equivalent to calling the worker function directly, except
// the context passed to it is canceled when the worker function returns.
func (wf Worker) Run(ctx context.Context) error {
	wctx, cancel := context.WithCancel(ctx)
	defer cancel()
	return wf(wctx)
}

// Safe runs the worker function and converts the worker function to a
// panic to an error.
func (wf Worker) Safe(ctx context.Context) (err error) {
	defer func() { err = mergeWithRecover(err, recover()) }()
	return wf(ctx)
}

// Block executes the worker function with a context that will never
// expire and returns the error. Use with caution
func (wf Worker) Block() error { return wf.Run(internal.BackgroundContext) }

// Observe runs the worker function, and observes the error (or nil
// response). Panics are converted to errors for both the worker
// function but not the observer function.
func (wf Worker) Observe(ctx context.Context, ob Observer[error]) {
	ob(wf.Safe(ctx))
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
	go func() { defer close(out); Blocking(out).Send().Ignore(ctx, wf.Safe(ctx)) }()
	return out
}

// Future runs the worker function in a go routine and returns a
// new fun.Worker which will block for the context to expire or the
// background worker to complete, which returns the error from the
// background request.
func (wf Worker) Future(ctx context.Context) Worker {
	out := wf.Signal(ctx)
	return func(ctx context.Context) error { return internal.MergeErrors(MakeFuture(out)(ctx)) }
}

func (wf Worker) Background(ctx context.Context, ob Observer[error]) {
	go func() { ob(wf.Safe(ctx)) }()
}

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
func (wf Worker) Must() WaitFunc   { return func(ctx context.Context) { InvariantMust(wf(ctx)) } }
func (wf Worker) Ignore() WaitFunc { return func(ctx context.Context) { _ = wf(ctx) } }

func (wf Worker) If(cond bool) Worker { return wf.When(func() bool { return cond }) }
func (wf Worker) When(cond func() bool) Worker {
	return func(ctx context.Context) error {
		if cond() {
			return wf(ctx)
		}
		return nil
	}

}
func (wf Worker) After(ts time.Time) Worker {
	return func(ctx context.Context) error { return wf.Delay(time.Until(ts))(ctx) }
}

func (wf Worker) Jitter(dur func() time.Duration) Worker { return wf.Delay(internal.Max(0, dur())) }

func (wf Worker) Delay(dur time.Duration) Worker {
	return func(ctx context.Context) error {
		timer := time.NewTimer(dur)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return wf(ctx)
		}
	}
}

func (wf Worker) Limit(in int) Worker {
	resolver := limitExec[error](in)

	return func(ctx context.Context) error { return resolver(func() error { return wf(ctx) }) }
}

func (wf Worker) TTL(dur time.Duration) Worker {
	resolver := ttlExec[error](dur)
	return func(ctx context.Context) error { return resolver(func() error { return wf(ctx) }) }
}

func (wf Worker) Lock() Worker {
	mtx := &sync.Mutex{}
	return func(ctx context.Context) error {
		mtx.Lock()
		defer mtx.Unlock()
		return wf(ctx)
	}
}
