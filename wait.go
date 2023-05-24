package fun

import (
	"context"
	"sync"

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

// Add starts a goroutine that waits for the WaitFunc to return,
// incrementing and decrementing the sync.WaitGroup as
// appropriate. The execution of the wait fun blocks on Add's context.
func (wf WaitFunc) Add(ctx context.Context, wg *WaitGroup) { wf.Background(wg)(ctx) }

func (wf WaitFunc) Background(wg *WaitGroup) WaitFunc {
	return func(ctx context.Context) { wg.Add(1); go func() { defer wg.Done(); wf(ctx) }() }
}

// Block runs the WaitFunc with a context that will never be canceled.
func (wf WaitFunc) Block() { wf.Run(internal.BackgroundContext) }

// Safe is catches panics and returns them as errors using
// fun.Check. This method is also a fun.WorkerFunc and can be used
// thusly.
func (wf WaitFunc) Safe(ctx context.Context) error { return Check(func() { wf(ctx) }) }

// Worker converts a wait function into a WorkerFunc. If the context
// is canceled, the worker function returns the context's error. The
// worker function also captures the wait functions panic and converts
// it to an error.
func (wf WaitFunc) Worker() WorkerFunc {
	return func(ctx context.Context) (err error) {
		return internal.MergeErrors(wf.Safe(ctx), ctx.Err())
	}
}

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

// WaitMerge starts a goroutine that blocks on each WaitFunc provided
// and returns a WaitFunc that waits for all of these goroutines to
// return. The constituent WaitFunc are passed WaitMerge's context,
// while the returned WaitFunc respects its own context.
//
// Use itertool.Variadic, itertool.Slice, or itertool.Channel to
// convert common container types/calling patterns to an iterator.
func WaitMerge(ctx context.Context, iter Iterator[WaitFunc]) WaitFunc {
	wg := &WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = Observe(ctx, iter, func(fn WaitFunc) { fn.Add(ctx, wg) })

	}()

	return wg.Wait
}
