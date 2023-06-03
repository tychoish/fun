package fun

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tychoish/fun/internal"
)

// WaitFunc is a type of function object that will block until an
// operation returns or the context is canceled.
type WaitFunc func(context.Context)

// WaitChannel converts a channel (typically, a `chan struct{}`) to a
// WaitFunc. The WaitFunc blocks till it's context is canceled or the
// channel is either closed or returns one item.
func WaitChannel[T any](ch <-chan T) WaitFunc {
	return func(ctx context.Context) {
		select {
		case <-ctx.Done():
		case <-ch:
		}
	}
}

// WaitContext wait's for the context to be canceled before
// returning. The WaitFunc that's return also respects it's own
// context. Use this WaitFunc and it's own context to wait for a
// context to be cacneled with a timeout, for instance.
func WaitContext(ctx context.Context) WaitFunc { return WaitChannel(ctx.Done()) }

// WaitForGroup converts a sync.WaitGroup into a fun.WaitFunc.
//
// This operation will leak a go routine if the WaitGroup
// never returns and the context is canceled. To avoid a leaked
// goroutine, use the fun.WaitGroup type.
func WaitForGroup(wg *sync.WaitGroup) WaitFunc {
	sig := make(chan struct{})

	go func() { defer close(sig); wg.Wait() }()

	return WaitChannel(sig)
}

// WaitMerge returns a WaitFunc that, when run, processes the incoming
// iterator of WaitFuncs, starts a go routine running each, and wait
// function and then blocks until all operations have returned, or the
// context passed to the output function has been canceled.
func WaitMerge(iter Iterator[WaitFunc]) WaitFunc {
	return func(ctx context.Context) {
		wg := &WaitGroup{}

		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = Observe(ctx, iter, func(fn WaitFunc) { fn.Add(ctx, wg) })
		}()

		wg.Wait(ctx)
	}
}

// Run is equivalent to calling the wait function directly, except the
// context passed to the function is always canceled when the wait
// function returns.
func (wf WaitFunc) Run(ctx context.Context) {
	wctx, cancel := context.WithCancel(ctx)
	defer cancel()

	wf(wctx)
}

func (wf WaitFunc) Once() WaitFunc {
	once := &sync.Once{}
	return func(ctx context.Context) { once.Do(func() { wf(ctx) }) }
}

func (wf WaitFunc) Signal(ctx context.Context) <-chan struct{} {
	out := make(chan struct{})
	go func() { defer close(out); wf(ctx) }()
	return out
}

func (wf WaitFunc) Future(ctx context.Context) WaitFunc {
	sig := wf.Signal(ctx)
	return func(ctx context.Context) { WaitChannel(sig) }
}

func (wf WaitFunc) Start() WaitFunc { return func(ctx context.Context) { go wf(ctx) } }

// Add starts a goroutine that waits for the WaitFunc to return,
// incrementing and decrementing the sync.WaitGroup as
// appropriate. The execution of the wait fun blocks on Add's context.
func (wf WaitFunc) Add(ctx context.Context, wg *WaitGroup) { wf.Background(wg)(ctx) }

func (wf WaitFunc) Background(wg *WaitGroup) WaitFunc {
	return func(ctx context.Context) { wg.Add(1); go func() { defer wg.Done(); wf(ctx) }() }
}

// Block runs the WaitFunc with a context that will never be canceled.
func (wf WaitFunc) Block() { wf.Run(internal.BackgroundContext) }

// Safe converts the WaitFunc into a Worker function that catchers
// panics and returns them as errors using fun.Check.
func (wf WaitFunc) Safe() Worker {
	return func(ctx context.Context) error { return Check(func() { wf(ctx) }) }
}

// Worker converts a wait function into a fun.Worker. If the context
// is canceled, the worker function returns the context's error.
func (wf WaitFunc) Worker() Worker {
	return func(ctx context.Context) (err error) { wf(ctx); return ctx.Err() }
}

func (wf WaitFunc) After(ts time.Time) WaitFunc {
	return func(ctx context.Context) { wf.Delay(time.Until(ts))(ctx) }
}

// Jitter wraps a WaitFunc that runs the jitter function (jf) once
// before every execution of the resulting fucntion, and waits for the
// resulting duration before running the WaitFunc operation.
//
// If the function produces a negative duration, there is no delay.
func (wf WaitFunc) Jitter(dur func() time.Duration) WaitFunc { return wf.Worker().Jitter(dur).Ignore() }

// Delay wraps a WaitFunc in a function that will always wait for the
// specified duration before running.
//
// If the value is negative, then there is always zero delay.
func (wf WaitFunc) Delay(dur time.Duration) WaitFunc { return wf.Worker().Delay(dur).Ignore() }
func (wf WaitFunc) When(cond func() bool) WaitFunc   { return wf.Worker().When(cond).Ignore() }
func (wf WaitFunc) If(cond bool) WaitFunc            { return wf.Worker().If(cond).Ignore() }

func (wf WaitFunc) Limit(in int) WaitFunc {
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

func (wf WaitFunc) TTL(dur time.Duration) WaitFunc {
	resolver := ttlExec[bool](dur)
	return func(ctx context.Context) { resolver(func() bool { wf(ctx); return true }) }
}

func (wf WaitFunc) Lock() WaitFunc {
	mtx := &sync.Mutex{}
	return func(ctx context.Context) {
		mtx.Lock()
		defer mtx.Unlock()
		wf(ctx)
	}
}

func (wf WaitFunc) Chain(next WaitFunc) WaitFunc {
	return func(ctx context.Context) { wf(ctx); next.If(ctx.Err() != nil)(ctx) }
}
