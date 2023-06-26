package fun

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/internal"
)

// Operation is a type of function object that will block until an
// operation returns or the context is canceled.
type Operation func(context.Context)

// BlockingOperation converts a function that takes no arguments into
// an Operation.
func BlockingOperation(in func()) Operation { return func(context.Context) { in() } }

// WaitChannel converts a channel (typically, a `chan struct{}`) to a
// Operation. The Operation blocks till it's context is canceled or the
// channel is either closed or returns one item.
func WaitChannel[T any](ch <-chan T) Operation {
	return func(ctx context.Context) {
		select {
		case <-ctx.Done():
		case <-ch:
		}
	}
}

// WaitContext wait's for the context to be canceled before
// returning. The Operation that's return also respects it's own
// context. Use this Operation and it's own context to wait for a
// context to be cacneled with a timeout, for instance.
func WaitContext(ctx context.Context) Operation { return WaitChannel(ctx.Done()) }

// WaitForGroup converts a sync.WaitGroup into a fun.Operation.
//
// This operation will leak a go routine if the WaitGroup
// never returns and the context is canceled. To avoid a leaked
// goroutine, use the fun.WaitGroup type.
func WaitForGroup(wg *sync.WaitGroup) Operation {
	sig := make(chan struct{})
	go func() { defer close(sig); wg.Wait() }()
	return WaitChannel(sig)
}

// WaitMerge returns a Operation that, when run, processes the incoming
// iterator of Operations, starts a go routine running each, and wait
// function and then blocks until all operations have returned, or the
// context passed to the output function has been canceled.
func WaitMerge(iter *Iterator[Operation]) Operation {
	return func(ctx context.Context) {
		wg := &WaitGroup{}

		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = iter.Observe(ctx, func(fn Operation) { fn.Add(ctx, wg) })
		}()

		wg.Wait(ctx)
	}
}

// Run is equivalent to calling the operation directly
func (wf Operation) Run(ctx context.Context) { wf(ctx) }

// WithCancel creates a Operation and a cancel function which will
// terminate the context that the root Operation is running
// with. This context isn't canceled *unless* the cancel function is
// called (or the context passed to the Operation is canceled.)
func (wf Operation) WithCancel() (Operation, context.CancelFunc) {
	var wctx context.Context
	var cancel context.CancelFunc
	once := &sync.Once{}
	return func(ctx context.Context) {
		once.Do(func() { wctx, cancel = context.WithCancel(ctx) })
		Invariant.IsTrue(wctx != nil, "must start the operation before calling cancel")
		wf(wctx)
	}, func() { once.Do(func() {}); ft.SafeCall(cancel) }
}

// Once produces an operation that will only execute the root
// operation once, no matter how many times it's called.
func (wf Operation) Once() Operation {
	once := &sync.Once{}
	return func(ctx context.Context) { once.Do(func() { wf(ctx) }) }
}

// Sginal starts the operation in a go routine, and provides a signal
// channel which will be closed when the operation is complete.
func (wf Operation) Signal(ctx context.Context) <-chan struct{} {
	out := make(chan struct{})
	go func() { defer close(out); wf(ctx) }()
	return out
}

// Future starts the operation in a background go routine and returns
// an operation which blocks until it's context is canceled or the
// underlying operation returns.
func (wf Operation) Future(ctx context.Context) Operation {
	sig := wf.Signal(ctx)
	return func(ctx context.Context) { WaitChannel(sig) }
}

// Go launches the operation in a go routine. There is no panic-safety
// provided.
func (wf Operation) Go(ctx context.Context) { go wf(ctx) }

// Launch provides access to the Go method (e.g. starting this
// operation in a go routine.) as a method that can be used as an
// operation itself.
func (wf Operation) Launch() Operation { return wf.Go }

// Add starts a goroutine that waits for the Operation to return,
// incrementing and decrementing the sync.WaitGroup as
// appropriate. The execution of the wait fun blocks on Add's context.
func (wf Operation) Add(ctx context.Context, wg *WaitGroup) { wf.Background(wg)(ctx) }

// StartGroup runs n groups, incrementing the waitgroup as
// appropriate.
func (wf Operation) StartGroup(ctx context.Context, wg *WaitGroup, n int) {
	ft.DoTimes(n, func() { wf.Add(ctx, wg) })
}

// Backgrounds creates a new operation which, when the resulting
// operation is called, starts the root operation in the background
// and returns immediately. use the wait group, or it's Operation to
// block on the completion of the background execution.
func (wf Operation) Background(wg *WaitGroup) Operation {
	return func(ctx context.Context) { wg.Add(1); go func() { defer wg.Done(); wf(ctx) }() }
}

// Block runs the Operation with a context that will never be canceled.
func (wf Operation) Block() { wf(context.Background()) }

// Safe converts the Operation into a Worker function that catchers
// panics and returns them as errors using fun.Check.
func (wf Operation) Safe() Worker {
	return func(ctx context.Context) error { return ers.Check(func() { wf(ctx) }) }
}

// Worker converts a wait function into a fun.Worker. If the context
// is canceled, the worker function returns the context's error.
func (wf Operation) Worker() Worker {
	return func(ctx context.Context) (err error) { wf(ctx); return ctx.Err() }
}

// Jitter wraps a Operation that runs the jitter function (jf) once
// before every execution of the resulting fucntion, and waits for the
// resulting duration before running the Operation operation.
//
// If the function produces a negative duration, there is no delay.
func (wf Operation) Jitter(dur func() time.Duration) Operation {
	return wf.Worker().Jitter(dur).Ignore()
}

// Delay wraps a Operation in a function that will always wait for the
// specified duration before running.
//
// If the value is negative, then there is always zero delay.
func (wf Operation) Delay(dur time.Duration) Operation { return wf.Worker().Delay(dur).Ignore() }

// After provides an operation that will only run if called after the
// specified clock time. When called after this time, the operation
// blocks until that time passes (or the context is canceled.)
func (wf Operation) After(ts time.Time) Operation { return wf.Worker().Delay(time.Until(ts)).Ignore() }

// When runs the condition function, and if it returns true,
func (wf Operation) When(cond func() bool) Operation { return wf.Worker().When(cond).Ignore() }

// If provides a static version of the When that only runs if the
// condition is true, and is otherwise a noop.
func (wf Operation) If(cond bool) Operation { return wf.Worker().If(cond).Ignore() }

// Limit provides an operation that will, no matter how many times is
// called, will only run once. The resulting operation is safe for
// concurrent use. Operations are launched serially (to maintain the
// counter,) but the operations themselves can run concurrently.
func (wf Operation) Limit(in int) Operation {
	Invariant.OK(in > 0, "limit must be greater than zero;", in)
	counter := &atomic.Int64{}
	mtx := &sync.Mutex{}

	return wf.When(func() bool {
		if counter.CompareAndSwap(int64(in), int64(in)) {
			return false
		}

		mtx.Lock()
		defer mtx.Unlock()

		num := counter.Load()
		if num >= int64(in) {
			return false
		}

		return counter.CompareAndSwap(num, internal.Min(int64(in), num+1))
	})
}

// TTL runs an operation, and if the operation is called before the
// specified duration, the operation is a noop.
func (wf Operation) TTL(dur time.Duration) Operation {
	resolver := ttlExec[bool](dur)
	return func(ctx context.Context) { resolver(func() bool { wf(ctx); return true }) }
}

// Lock constructs a mutex that ensure that the underlying operation
// (when called through the output operation,) only runs within the
// scope of the lock.
func (wf Operation) Lock() Operation { return wf.WithLock(&sync.Mutex{}) }

// WithLock ensures that the underlying operation, when called through
// the output operation, will holed the mutex while running.
func (wf Operation) WithLock(mtx *sync.Mutex) Operation {
	return func(ctx context.Context) {
		mtx.Lock()
		defer mtx.Unlock()
		wf(ctx)
	}
}

// Join runs the first operation, and then if the context has not
// expired, runs the second operation.
func (wf Operation) Join(op Operation) Operation {
	return func(ctx context.Context) { wf(ctx); op.If(ctx.Err() == nil).Run(ctx) }
}

// PostHook unconditionally runs the post-hook operation after the
// operation returns. Use the hook to run cleanup operations.
func (wf Operation) PostHook(hook func()) Operation {
	return func(ctx context.Context) { wf(ctx); hook() }
}

// PreHook unconditionally runs the hook operation before the
// underlying operation. Use Operaiton.Once() operations for the hook
// to initialize resources for use by the operation, or without Once
// to reset.
func (wf Operation) PreHook(hook Operation) Operation {
	return func(c context.Context) { hook(c); wf(c) }
}
