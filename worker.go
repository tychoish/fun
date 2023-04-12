package fun

import (
	"context"
	"time"

	"github.com/tychoish/fun/internal"
)

// WorkerFunc represents a basic function used in worker pools and
// other similar situations
type WorkerFunc func(context.Context) error

// Run is equivalent to calling the worker function directly, except
// the context passed to it is canceled when the worker function returns.
func (wf WorkerFunc) Run(ctx context.Context) error {
	wctx, cancel := context.WithCancel(ctx)
	defer cancel()
	return wf(wctx)
}

func (wf WorkerFunc) Safe(ctx context.Context) (err error) {
	defer func() { err = mergeWithRecover(err, recover()) }()
	return wf.Run(ctx)
}

// Block executes the worker function with a context that will never
// expire and returns the error. Use with caution
func (wf WorkerFunc) Block() error { return wf.Run(internal.BackgroundContext) }

// Observe runs the worker function, and observes the error. Panics
// are converted to errors for both the worker function but not the
// observer function.
func (wf WorkerFunc) Observe(ctx context.Context, ob Observer[error]) {
	if err := wf.Safe(ctx); err != nil {
		ob(err)
	}
}

// Signal runs the worker function in a background goroutine and
// returns the error in an error channel, that returns when the
// worker function returns. If Singal is called with a canceled
// context the worker is still executed (with that context.)
//
// A value, possibly nil, is always sent through the channel, though
// the WorkerFunc runs in a different go routine, a panic handler will
// convert panics to errors.
func (wf WorkerFunc) Singal(ctx context.Context) <-chan error {
	out := make(chan error)
	go func() {
		defer close(out)
		out <- wf.Safe(ctx)
	}()

	return out
}

// MustWait converts a Worker function into a wait function; however,
// if the worker produces an error MustWait converts the error into a
// panic.
func (wf WorkerFunc) MustWait() WaitFunc {
	return func(ctx context.Context) { InvariantMust(wf.Run(ctx)) }
}

// ObserveWait converts a worker function into a wait function,
// passing any error to the observer function. Only non-nil errors are
// observed.
func (wf WorkerFunc) ObserveWait(ob Observer[error]) WaitFunc {
	return func(ctx context.Context) { wf.Observe(ctx, ob) }
}

// WithTimeout executes the worker function with the provided timeout
// using a new context.
func (wf WorkerFunc) WithTimeout(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(internal.BackgroundContext, timeout)
	defer cancel()
	return wf(ctx)
}
