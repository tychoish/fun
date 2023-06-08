package fun

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/internal"
)

// Operation is a type of function object that will block until an
// operation returns or the context is canceled.
type Operation func(context.Context)

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

// Run is equivalent to calling the wait function directly, except the
// context passed to the function is always canceled when the wait
// function returns.
func (wf Operation) Run(ctx context.Context) {
	wctx, cancel := context.WithCancel(ctx)
	defer cancel()

	wf(wctx)
}

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
		if err := wctx.Err(); err != nil {
			return
		}
		wf(ctx)
	}, func() { once.Do(func() {}); WhenCall(cancel != nil, cancel) }
}

func (wf Operation) Once() Operation {
	once := &sync.Once{}
	return func(ctx context.Context) { once.Do(func() { wf(ctx) }) }
}

func (wf Operation) Signal(ctx context.Context) <-chan struct{} {
	out := make(chan struct{})
	go func() { defer close(out); wf(ctx) }()
	return out
}

func (wf Operation) Future(ctx context.Context) Operation {
	sig := wf.Signal(ctx)
	return func(ctx context.Context) { WaitChannel(sig) }
}

func (wf Operation) Go(ctx context.Context) { go wf(ctx) }
func (wf Operation) Launch() Operation      { return wf.Go }

// Add starts a goroutine that waits for the Operation to return,
// incrementing and decrementing the sync.WaitGroup as
// appropriate. The execution of the wait fun blocks on Add's context.
func (wf Operation) Add(ctx context.Context, wg *WaitGroup) { wf.Background(wg)(ctx) }

func (wf Operation) StartGroup(ctx context.Context, wg *WaitGroup, n int) {
	for i := 0; i < n; i++ {
		wf.Add(ctx, wg)
	}
}

func (wf Operation) Background(wg *WaitGroup) Operation {
	return func(ctx context.Context) { wg.Add(1); go func() { defer wg.Done(); wf(ctx) }() }
}

// Block runs the Operation with a context that will never be canceled.
func (wf Operation) Block() { wf(internal.BackgroundContext) }

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

func (wf Operation) After(ts time.Time) Operation {
	return func(ctx context.Context) { wf.Delay(time.Until(ts))(ctx) }
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
func (wf Operation) When(cond func() bool) Operation   { return wf.Worker().When(cond).Ignore() }
func (wf Operation) If(cond bool) Operation            { return wf.Worker().If(cond).Ignore() }

func (wf Operation) Limit(in int) Operation {
	Invariant(in > 0, "limit must be greater than zero;", in)
	counter := &atomic.Int64{}
	mtx := &sync.Mutex{}

	return func(ctx context.Context) {
		if counter.CompareAndSwap(int64(in), int64(in)) {
			return
		}

		func() {
			mtx.Lock()
			defer mtx.Unlock()

			num := counter.Load()

			counter.CompareAndSwap(int64(in), internal.Min(int64(in), num+1))
		}()
		wf(ctx)
	}
}

func (wf Operation) TTL(dur time.Duration) Operation {
	resolver := ttlExec[bool](dur)
	return func(ctx context.Context) { resolver(func() bool { wf(ctx); return true }) }
}

func (wf Operation) Lock() Operation { return wf.WithLock(&sync.Mutex{}) }

func (wf Operation) WithLock(mtx *sync.Mutex) Operation {
	return func(ctx context.Context) {
		mtx.Lock()
		defer mtx.Unlock()
		wf(ctx)
	}
}

func (wf Operation) Chain(op Operation) Operation {
	return func(ctx context.Context) { wf(ctx); op.If(ctx.Err() == nil)(ctx) }
}

func (wf Operation) PostHook(op func()) Operation { return func(ctx context.Context) { wf(ctx); op() } }
func (wf Operation) PreHook(op func(context.Context)) Operation {
	return func(ctx context.Context) { op(ctx); wf(ctx) }
}
