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
	wg := &WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for iter.Next(ctx) {
			wg.Add(1)
			go func(wf WorkerFunc) { defer wg.Done(); wf.Observe(ctx, ob) }(iter.Value())
		}
	}()

	return wg.Wait
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
	wg := &WaitGroup{}
	pipe := make(chan WorkerFunc)
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(pipe)
		for iter.Next(ctx) {
			select {
			case pipe <- iter.Value():
			case <-ctx.Done():
				ob(iter.Close())
				return
			}
		}
		ob(iter.Close())
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
				op.Observe(ctx, ob)
			}
		}()
	}

	return wg.Wait
}
