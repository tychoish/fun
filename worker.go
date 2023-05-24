package fun

import (
	"context"
	"sync"

	"github.com/tychoish/fun/internal"
)

// WorkerFunc represents a basic function used in worker pools and
// other similar situations
type WorkerFunc func(context.Context) error

func MakeFuture(ch <-chan error) WorkerFunc {
	return func(ctx context.Context) error {
		return internal.MergeErrors(BlockingReceive(ch).Read(ctx))
	}
}

// Run is equivalent to calling the worker function directly, except
// the context passed to it is canceled when the worker function returns.
func (wf WorkerFunc) Run(ctx context.Context) error {
	wctx, cancel := context.WithCancel(ctx)
	defer cancel()
	return wf(wctx)
}

// Safe runs the worker function and converts the worker function to a
// panic to an error.
func (wf WorkerFunc) Safe(ctx context.Context) (err error) {
	defer func() { err = mergeWithRecover(err, recover()) }()
	return wf(ctx)
}

// Block executes the worker function with a context that will never
// expire and returns the error. Use with caution
func (wf WorkerFunc) Block() error { return wf.Run(internal.BackgroundContext) }

// Observe runs the worker function, and observes the error (or nil
// response). Panics are converted to errors for both the worker
// function but not the observer function.
func (wf WorkerFunc) Observe(ctx context.Context, ob Observer[error]) {
	ob(wf.Safe(ctx))
}

// Signal runs the worker function in a background goroutine and
// returns the error in an error channel, that returns when the
// worker function returns. If Signal is called with a canceled
// context the worker is still executed (with that context.)
//
// A value, possibly nil, is always sent through the channel, though
// the WorkerFunc runs in a different go routine, a panic handler will
// convert panics to errors.
func (wf WorkerFunc) Signal(ctx context.Context) <-chan error {
	out := make(chan error)
	go func() { defer close(out); Blocking(out).Send().Ignore(ctx, wf.Safe(ctx)) }()
	return out
}

// Future runs the worker function in a go routine and returns a
// new WorkerFunc which will block for the context to expire or the
// background worker to complete.
func (wf WorkerFunc) Future() WorkerFunc {
	return func(ctx context.Context) error {
		return MakeFuture(wf.Signal(ctx))(ctx)
	}
}

func (wf WorkerFunc) Background(ctx context.Context, ob Observer[error]) {
	go func() { ob(wf.Safe(ctx)) }()
}

func (wf WorkerFunc) Once() WorkerFunc {
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
func (wf WorkerFunc) Wait(ob Observer[error]) WaitFunc {
	return func(ctx context.Context) { wf.Observe(ctx, ob) }
}

// Must converts a Worker function into a wait function; however,
// if the worker produces an error Must converts the error into a
// panic.
func (wf WorkerFunc) Must() WaitFunc   { return func(ctx context.Context) { InvariantMust(wf(ctx)) } }
func (wf WorkerFunc) Ignore() WaitFunc { return func(ctx context.Context) { _ = wf(ctx) } }
