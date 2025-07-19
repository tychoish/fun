package fun

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tychoish/fun/fn"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/internal"
)

// Operation is a type of function object that will block until an
// operation returns or the context is canceled.
type Operation func(context.Context)

// MakeOperation converts a function that takes no arguments into
// an Operation.
func MakeOperation(in func()) Operation { return func(context.Context) { in() } }

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
	}, func() { once.Do(func() {}); ft.CallSafe(cancel) }
}

// WithContextHook returns an wrapped operation where the provided
// context hook intercepts the context that the worker function is
// called with. This makes a kind of rudamentary middleware possible.
func (wf Operation) WithContextHook(hook func(context.Context) context.Context) Operation {
	return func(ctx context.Context) { wf(hook(ctx)) }
}

// Once produces an operation that will only execute the root
// operation once, no matter how many times it's called.
func (wf Operation) Once() Operation {
	once := &sync.Once{}
	return func(ctx context.Context) { once.Do(func() { wf(ctx) }) }
}

// Signal starts the operation in a go routine, and provides a signal
// channel which will be closed when the operation is complete.
func (wf Operation) Signal(ctx context.Context) <-chan struct{} {
	out := make(chan struct{})
	go func() { defer close(out); wf(ctx) }()
	return out
}

// Launch starts the operation in a background go routine and returns
// an operation which blocks until it's context is canceled or the
// underlying operation returns.
func (wf Operation) Launch(ctx context.Context) Operation {
	sig := wf.Signal(ctx)
	return func(context.Context) { WaitChannel(sig) }
}

// Background launches the operation in a go routine. There is no panic-safety
// provided.
func (wf Operation) Background(ctx context.Context) { go wf(ctx) }

// Go provides access to the Go method (e.g. starting this
// operation in a go routine.) as a method that can be used as an
// operation itself.
func (wf Operation) Go() Operation { return wf.Background }

// Add starts a the operation in a goroutine incrementing and
// decrementing the WaitGroup as appropriate.
func (wf Operation) Add(ctx context.Context, wg *WaitGroup) { wg.Launch(ctx, wf) }

// StartGroup runs n operations, incrementing the WaitGroup to account
// for the job. Callers must wait on the WaitGroup independently.
func (wf Operation) StartGroup(ctx context.Context, n int) Operation {
	wg := &WaitGroup{}
	return wg.StartGroup(ctx, n, wf)
}

// Group makes an operation that runs n copies of the underlying
// worker, in different go routines. Work does not start until the
// resulting worker is called.
func (wf Operation) Group(n int) Operation {
	return func(ctx context.Context) { wf.StartGroup(ctx, n).Run(ctx) }
}

// Interval runs the operation with a timer that resets to the
// provided duration. The operation runs immediately, and then the
// time is reset to the specified interval after the base operation is
// completed. Which is to say that the runtime of the operation itself
// is effectively added to the interval.
func (wf Operation) Interval(dur time.Duration) Operation {
	return func(ctx context.Context) {
		timer := time.NewTimer(0)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				wf(ctx)
				timer.Reset(dur)
			}
		}
	}
}

// While runs the operation in a tight loop, until the context
// expires.
func (wf Operation) While() Operation {
	return func(ctx context.Context) {
		for {
			wf.Run(ctx)
			if ctx.Err() != nil {
				return
			}
		}
	}
}

// Wait runs the operation with a background context.
func (wf Operation) Wait() { wf(context.Background()) }

// WithRecover converts the Operation into a Worker function that catchers
// panics and returns them as errors using fun.Check.
func (wf Operation) WithRecover() Worker {
	return func(ctx context.Context) error { return ft.WithRecoverApply(wf, ctx) }
}

// Worker converts a wait function into a fun.Worker. If the context
// is canceled, the worker function returns the context's error. Does
// not handle panics, use WithRecover() to convert panics to errors
func (wf Operation) Worker() Worker {
	return func(ctx context.Context) (err error) { wf(ctx); return ctx.Err() }
}

// WithErrorHook prodces a Worker function that runs the
// operation--potentially catching a panic and converting it to an
// error--and then passes that error to the error handler, before
// returning the original error. These errors are either context
// cancilations or error that were converted from panics.
func (wf Operation) WithErrorHook(ef fn.Handler[error]) Worker {
	return wf.WithRecover().WithErrorHook(ef)
}

// Jitter wraps a Operation that runs the jitter function (jf) once
// before every execution of the resulting function, and waits for the
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

// Limit returns an operation that will only run the specified number
// of times. The resulting operation is safe for concurrent use, but
// operations can run concurrently.
func (wf Operation) Limit(in int) Operation {
	Invariant.Ok(in > 0, "limit must be greater than zero;", in)
	counter := &atomic.Int64{}

	return wf.When(func() bool {
		for {
			current := counter.Load()
			if current >= int64(in) {
				return false
			}

			if counter.CompareAndSwap(current, current+1) {
				return true
			}
		}
	})
}

// TTL runs an operation, and if the operation is called before the
// specified duration, the operation is a noop.
func (wf Operation) TTL(dur time.Duration) Operation {
	resolver := ft.Must(internal.TTLExec[bool](dur))
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

// Join combines a sequence of operations, calling the Operations in
// order as long as the context does not expire. If the context
// expires, the combined operation aborts early.
func (wf Operation) Join(ops ...Operation) Operation {
	for idx := range ops {
		wf = wf.merge(ops[idx])
	}
	return wf
}

func (wf Operation) merge(next Operation) Operation {
	return func(ctx context.Context) { wf(ctx); next.If(ctx.Err() == nil).Run(ctx) }
}

// PostHook unconditionally runs the post-hook operation after the
// operation returns, as a defer. Use the hook to run cleanup
// operations. The Operation returned from this method runs both the
// original hook, and the hook function.
func (wf Operation) PostHook(hook func()) Operation {
	return func(ctx context.Context) { defer hook(); wf(ctx) }
}

// PreHook unconditionally runs the hook operation before the
// underlying operation. Use Operaiton.Once() operations for the hook
// to initialize resources for use by the operation, or without Once
// to provide reset semantics. The Operation returned from this method
// runs both the original hook, and the hook function.
func (wf Operation) PreHook(hook Operation) Operation {
	return func(c context.Context) { hook(c); wf(c) }
}
