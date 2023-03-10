package fun

import (
	"context"
	"sync"
	"time"

	"github.com/tychoish/fun/internal"
)

// WaitFunc is a type of function object that will block until an
// operation returns or the context is canceled.
type WaitFunc func(context.Context)

// Block executes the WaitFunc with a context that will never
// expire. Use with extreme caution.
func (wf WaitFunc) Block() { wf(internal.BackgroundContext) }

// WithTimeout runs the WaitFunc with an explicit timeout.
func (wf WaitFunc) WithTimeout(timeout time.Duration) {
	ctx, cancel := context.WithTimeout(internal.BackgroundContext, timeout)
	defer cancel()
	wf(ctx)
}

// Signal runs the WaitFunc in a goroutine and returns a signal
// channel that is canceled when the function completes. Useful for
// bridging the gap between interfaces and integration that use
// channels and functions.
//
// Callers are responsble for handling the (potential) panic in the WaitFunc.
func (wf WaitFunc) Signal(ctx context.Context) <-chan struct{} {
	out := make(chan struct{})
	go func() { defer close(out); wf(ctx) }()
	return out
}

// WithTimeoutSignal executes the WaitFunc as in WithTimeout, but
// returns a singal channel that is closed when the task completes.
//
// Callers are responsble for handling the (potential) panic in the WaitFunc.
func (wf WaitFunc) WithTimeoutSignal(timeout time.Duration) <-chan struct{} {
	ctx, cancel := context.WithTimeout(internal.BackgroundContext, timeout)
	sig := make(chan struct{})
	go func() { defer close(sig); defer cancel(); wf(ctx) }()
	return sig
}

// BlockSignal runs the WaitFunc in a background goroutine and
// returns a signal channel that is closed when the operation completes.
// As in Block() the WaitFunc is passed a background context that is
// never canceled.
//
// Callers are responsble for handling the (potential) panic in the WaitFunc.
func (wf WaitFunc) BlockSignal() <-chan struct{} {
	ctx, cancel := context.WithCancel(internal.BackgroundContext)
	sig := make(chan struct{})
	go func() { defer close(sig); defer cancel(); wf(ctx) }()
	return sig
}

// WaitBlocking is a convenience function to use simple blocking
// functions into WaitFunc objects. Because these WaitFunc functions
// do not resepct the WaitFunc context, use with care and caution.
func WaitBlocking(fn func()) WaitFunc { return func(context.Context) { fn() } }

// WaitBlockingObserve is a convenience function that creates a
// WaitFunc that wraps a simple function that returns a single value,
// and observes that output with the observer function.
//
// Because these WaitFunc functions do not resepct the WaitFunc
// context, use with care and caution.
func WaitBlockingObserve[T any](observe func(T), wait func() T) WaitFunc {
	return func(context.Context) { observe(wait()) }
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

// WaitObserve passes the output of the channel into the observer
// function and then returns. If the context is canceled the output of
// the channel is not observed.
//
// WaitObserve consumes and observes, at most, one item from the
// channel. Callers must call the WaitFunc.
func WaitObserve[T any](observe func(T), ch <-chan T) WaitFunc {
	return func(ctx context.Context) {
		val, err := ReadOne(ctx, ch)
		if err != nil {
			return
		}
		observe(val)
	}
}

// WaitObserveAll passes the output of the channel into the observer
// function, waiting for the input channel to be closed or the
// WaitFunc's context to be canceled. WaitObserveAll does not begin
// processing the channel until the WaitFunc is called.
func WaitObserveAll[T any](observe func(T), ch <-chan T) WaitFunc {
	return WaitObserveAllCtx(func(_ context.Context, in T) { observe(in) }, ch)
}

// WaitObserveAllCtx passes the output of the channel into the observer
// function with a context, waiting for the input channel to be closed or the
// WaitFunc's context to be canceled. WaitObserveAll does not begin
// processing the channel until the WaitFunc is called.
func WaitObserveAllCtx[T any](observe func(context.Context, T), ch <-chan T) WaitFunc {
	return func(ctx context.Context) {
		for {
			val, err := ReadOne(ctx, ch)
			if err != nil {
				return
			}
			observe(ctx, val)
		}
	}
}

// WaitChannel converts a channel (typically, a `chan struct{}`) to a
// WaitFunc. The WaitFunc blocks till it's context is canceled or the
// channel is either closed or returns one item.
func WaitChannel[T any](ch <-chan T) WaitFunc { return WaitObserve(func(T) {}, ch) }

// WaitContext wait's for the context to be canceled before
// returning. The WaitFunc that's return also respects it's own
// context. Use this WaitFunc and it's own context to wait for a
// context to be cacneled with a timeout, for instance.
func WaitContext(ctx context.Context) WaitFunc { return WaitChannel(ctx.Done()) }

// WaitAdd starts a goroutine that waits for the WaitFunc to return,
// incrementing and decrementing the sync.WaitGroup as
// appropriate. This WaitFunc blocks on WaitAdd's context.
func WaitAdd(ctx context.Context, wg *WaitGroup, fn WaitFunc) {
	wg.Add(1)

	go func() {
		defer wg.Done()
		fn(ctx)
	}()
}

// WaitMerge starts a goroutine that blocks on each WaitFunc provided
// and returns a WaitFunc that waits for all of these goroutines to
// return. The constituent WaitFunc are passed WaitMerge's context,
// while the returned WaitFunc respects its own context.
//
// Use itertool.Variadic, itertool.Slice, or itertool.Channel to
// convert common container types/calling patterns to an iterator.
//
// In combination with erc.CheckWait, you can use WaitMerge to create
// and pubsub.Queue or pubsub.Deque blocking iterators to create
// worker pools.
func WaitMerge(ctx context.Context, iter Iterator[WaitFunc]) WaitFunc {
	wg := &WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		Observe(ctx, iter, func(fn WaitFunc) { WaitAdd(ctx, wg, fn) })
	}()

	return wg.Wait
}
