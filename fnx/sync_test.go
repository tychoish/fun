package fnx

import (
	"context"
	"math/rand"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/fn"
)

func TestWaitGroup(t *testing.T) {
	t.Parallel()
	t.Run("MultipleWaiters", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		wg := &WaitGroup{}
		wg.Add(100)
		start := time.Now()
		firstCase := make(chan struct{})
		go func() { wg.Wait(ctx) }()

		go func() {
			defer close(firstCase)

			wg.Wait(ctx)
		}()

		runtime.Gosched()

		secondCase := make(chan struct{})
		go func() {
			defer close(secondCase)
			nctx, ncancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
			defer ncancel()
			wg.Wait(nctx)
		}()
		runtime.Gosched()

		<-secondCase

		timeoutDur := time.Since(start)
		if timeoutDur > 50*time.Millisecond {
			t.Error("timeout waiter took too long", timeoutDur)
		}
		time.Sleep(10 * time.Millisecond)
		cancel()

		<-firstCase

		blockingDur := time.Since(start)
		if blockingDur-timeoutDur > 50*time.Millisecond {
			t.Error("blocking waiter deadlocked", blockingDur, timeoutDur)
		}
	})
	t.Run("BusyBlocking", func(t *testing.T) {
		t.Parallel()

		wg := &WaitGroup{}
		const num = 100
		wg.Add(100)
		waits := make([]chan struct{}, 100)
		for i := range num {
			ch := make(chan struct{})
			waits[i] = ch
			go func(ch chan struct{}) {
				defer close(ch)
				wg.Wait(context.Background())
			}(ch)
		}

		workers := &sync.WaitGroup{}
		for i := range num {
			workers.Go(func() {
				defer wg.Done()
				time.Sleep(time.Duration(rand.Int63n(100)+1) * time.Millisecond)
			})
			if i%10 == 0 {
				runtime.Gosched()
			}
		}
		workers.Wait()
		awaitAll(t, waits)
	})
	t.Run("BusyBlockingMixed", func(t *testing.T) {
		t.Parallel()

		wg := &WaitGroup{}
		const num = 100
		wg.Add(100)
		waits := make([]chan struct{}, 100)
		for i := range num {
			ch := make(chan struct{})
			waits[i] = ch
			if i%4 == 0 {
				go func(ch chan struct{}) {
					defer close(ch)
					_ = wg.Worker().Wait()
				}(ch)
			} else if i%2 == 0 {
				go func(ch chan struct{}) {
					defer close(ch)
					wg.Operation().Wait()
				}(ch)
			} else {
				go func(ch chan struct{}, num int) {
					defer close(ch)
					ctx, cancel := context.WithTimeout(
						context.Background(),
						time.Duration(num*2)*time.Millisecond,
					)
					defer cancel()
					wg.Wait(ctx)
				}(ch, i)
			}
		}

		workers := &sync.WaitGroup{}
		for i := range num {
			workers.Go(func() {
				defer wg.Done()
				time.Sleep(time.Duration(rand.Int63n(100)+1) * time.Millisecond)
			})
			if i%10 == 0 {
				runtime.Gosched()
			}
		}
		workers.Wait()
		awaitAll(t, waits)
	})

	t.Run("Lock", func(t *testing.T) {
		count := 0
		thunk := fn.MakeFuture(func() int { count++; return 42 }).Lock()

		ctx := t.Context()

		// tempt the race detector.
		wg := &WaitGroup{}
		wg.Group(128, func(context.Context) {
			check.Equal(t, thunk(), 42)
		}).Run(ctx)
		wg.Wait(ctx)

		check.Equal(t, count, 128)
	})
	t.Run("WithLock", func(t *testing.T) {
		count := 0
		thunk := fn.MakeFuture(func() int { count++; return 42 }).WithLock(&sync.Mutex{})

		ctx := t.Context()

		// tempt the race detector.
		wg := &WaitGroup{}
		wg.Group(128, func(context.Context) {
			check.Equal(t, thunk(), 42)
		}).Run(ctx)

		wg.Wait(ctx)
		check.Equal(t, count, 128)
	})
}

// awaitAll fails the test if the waiters do not resolve; the bound is
// a deadlock guard, not a performance assertion.
func awaitAll(t *testing.T, waits []chan struct{}) {
	t.Helper()
	deadline := time.After(10 * time.Second)
	for _, ch := range waits {
		select {
		case <-ch:
		case <-deadline:
			t.Fatal("waiters did not resolve")
		}
	}
}
