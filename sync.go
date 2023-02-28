package fun

import (
	"context"
	"sync"

	"github.com/tychoish/fun/internal"
)

// Wait returns when the all waiting items are done, *or* the context
// is canceled. This operation will leak a go routine if the WaitGroup
// never returns and the context is canceled.
//
// fun.Wait(wg) is equivalent to fun.WaitGroup(wg)(ctx)
func Wait(ctx context.Context, wg *sync.WaitGroup) { WaitGroup(wg)(ctx) }

// WaitFunc is a type of function object that will block until an
// operation returns or the context is canceled.
type WaitFunc func(context.Context)

// Block executes the wait function with a context that will never
// expire. Use with extreme caution.
func (wf WaitFunc) Block() { wf(internal.BackgroundContext) }

// WaitGroup converts a WaitGroup into a fun.WaitFunc.
//
// This operation will leak a go routine if the WaitGroup
// never returns and the context is canceled.
func WaitGroup(wg *sync.WaitGroup) WaitFunc {
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
	return func(ctx context.Context) { doWaitObserve(ctx, observe, ch) }
}

// WaitObserveAll passes the output of the channel into the observer
// function, waiting for the input channel to be closed or the
// WaitFunc's context to be canceled. WaitObserveAll does not begin
// processing the channel until the WaitFunc is called.
func WaitObserveAll[T any](observe func(T), ch <-chan T) WaitFunc {
	return func(ctx context.Context) {
		for {
			if doWaitObserve(ctx, observe, ch) {
				return
			}
		}
	}
}

func doWaitObserve[T any](ctx context.Context, observe func(T), ch <-chan T) bool {
	select {
	case <-ctx.Done():
		return true
	case obj, ok := <-ch:
		if !ok || ctx.Err() != nil {
			return true
		}
		observe(obj)
	}
	return false
}

// WaitChannel converts a channel (typically, a `chan struct{}`) to a
func WaitChannel[T any](ch <-chan T) WaitFunc { return WaitObserve(func(T) {}, ch) }

// WaitContext wait's for the context to be canceled before
// returning. The WaitFunc that's return also respects it's own
// context. Use this WaitFunc and it's own context to wait for a
// context to be cacneled with a timeout, for instance.
func WaitContext(ctx context.Context) WaitFunc { return WaitChannel(ctx.Done()) }

// WaitAdd starts a goroutine that waits for the WaitFunc to return,
// incrementing and decrementing the sync.WaitGroup as
// appropriate. This WaitFunc blocks on WaitAdd's context.
func WaitAdd(ctx context.Context, wg *sync.WaitGroup, fn WaitFunc) {
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
// User itertool.Variadic, itertool.Slice, or itertool.Channel to
// convert common container types/calling patterns to an iterator.
func WaitMerge(ctx context.Context, iter Iterator[WaitFunc]) WaitFunc {
	wg := &sync.WaitGroup{}

	for iter.Next(ctx) {
		WaitAdd(ctx, wg, iter.Value())
	}

	return WaitGroup(wg)
}
