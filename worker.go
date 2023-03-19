package fun

import (
	"context"
	"time"

	"github.com/tychoish/fun/internal"
)

// WorkerFunc represents a basic function used in worker pools and
// other similar situations
type WorkerFunc func(context.Context) error

// Block executes the worker function with a context that will never
// expire and returns the error. Use with caution
func (wf WorkerFunc) Block() error { return wf(internal.BackgroundContext) }

// Observe runs the worker function passing the error output to the
// observer function. Only non-nil errors are observed.
func (wf WorkerFunc) Observe(ctx context.Context, ob func(error)) {
	if err := wf.Run(ctx); err != nil {
		ob(err)
	}
}

// Signal runs the worker function in a background goroutine and
// returns the error in an error channel, that returns when the
// worker function returns. If Singal is called with a canceled
// context the worker is still executed (with that context.)
func (wf WorkerFunc) Singal(ctx context.Context) <-chan error {
	out := make(chan error)
	go func() { defer close(out); out <- wf.Run(ctx) }()

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
func (wf WorkerFunc) ObserveWait(ob func(error)) WaitFunc {
	return func(ctx context.Context) { wf.Observe(ctx, ob) }
}

// WithTimeout executes the worker function with the provided timeout
// using a new context.
func (wf WorkerFunc) WithTimeout(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(internal.BackgroundContext, timeout)
	defer cancel()
	return wf(ctx)
}

// Run is equivalent to calling the worker function directly, except
// the context passed to it is canceled when the worker function returns.
func (wf WorkerFunc) Run(ctx context.Context) error {
	wctx, cancel := context.WithCancel(ctx)
	defer cancel()
	return wf(wctx)
}

// ObserveWorkerFuncs runs each worker function present in the
// iterator running until the iterator stops provding values. This is
// a non-blocking function, and the returned waitfunction blocks until
// all Workers *and* the iterator have completed.
//
// Only non-nil error outputs from the worker functions are passed to
// the observer. Panics are not handled. This worker pool is
// unbounded.
//
// Use itertool.Channel, itertool.Slice, and itertool.Variadic to
// produce iterators from standard types for use with this function.
func ObserveWorkerFuncs(ctx context.Context, iter Iterator[WorkerFunc], ob func(error)) WaitFunc {
	return ObserveAll(ctx, iter, func(fn WorkerFunc) { ob(fn(ctx)) })
}

// ObserveWorkerPool creates a worker pool of the specified size and
// dispatches worker functions from the iterator until the iterator is
// closed and all worker functions have returned. The context passed
// to the worker functions is always a decedent of the context passed
// to ObserveWorkerPool.
//
// This operation is non-blocking. Callers must call the WaitFunc to
// ensure all work is complete.
//
// Only non-nil error outputs from the worker functions are passed to
// the observer. Panics in workers are not handled. If the size is <=
// 0 then a size of 1 is used.
//
// Use itertool.Channel, itertool.Slice, and itertool.Variadic to
// produce iterators from standard types for use with this function.
func ObserveWorkerPool(ctx context.Context, size int, iter Iterator[WorkerFunc], ob func(error)) WaitFunc {
	wf := ObservePool(ctx, size, iter, func(fn WorkerFunc) { ob(fn(ctx)) })
	return func(ctx context.Context) { ob(wf(ctx)) }
}

// ObservePool processes the items in the worker pool, passing them
// all the observer function. The iterator is read serially and then
// the items are processed in parallel in the specified (size) number
// of goroutines. Panics in observer functions are not handled. Use
// itertool.ObserveParallel for similar (and safer!) and more
// configurable functionality.
//
// Errors are encountered closing the iterator
func ObservePool[T any](ctx context.Context, size int, iter Iterator[T], observe func(T)) WorkerFunc {
	var err error

	wg := &WaitGroup{}
	pipe := make(chan T)
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(pipe)
		for iter.Next(ctx) {
			select {
			case pipe <- iter.Value():
			case <-ctx.Done():
				err = iter.Close()
				return
			}
		}
		err = iter.Close()
	}()
	if size <= 0 {
		size = 1
	}
	for i := 0; i < size; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				op, err := ReadOne(ctx, pipe)
				if err != nil {
					return
				}
				observe(op)
			}
		}()
	}

	return func(ctx context.Context) error {
		wg.Wait(ctx)

		// if the waitgroup returns we know that nothing could
		// be mutating the error
		return err
	}
}
