package fun

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
		for i := 0; i < num; i++ {
			ch := make(chan struct{})
			waits[i] = ch
			go func(ch chan struct{}) {
				defer close(ch)
				wg.Wait(context.Background())
			}(ch)
		}

		for i := 0; i < num; i++ {
			go func() {
				defer wg.Done()
				time.Sleep(time.Duration(rand.Int63n(100)+1) * time.Millisecond)
			}()
			if i%10 == 0 {
				runtime.Gosched()
			}
		}
		time.Sleep(200 * time.Millisecond)
		waitStart := time.Now()
		for _, ch := range waits {
			<-ch
		}
		dur := time.Since(waitStart)
		if dur > 50*time.Millisecond {
			t.Error("took too long for waiters to resolve", dur)
		}
	})
	t.Run("BusyBlockingMixed", func(t *testing.T) {
		t.Parallel()

		wg := &WaitGroup{}
		const num = 100
		wg.Add(100)
		waits := make([]chan struct{}, 100)
		for i := 0; i < num; i++ {
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

		for i := 0; i < num; i++ {
			go func() {
				defer wg.Done()
				time.Sleep(time.Duration(rand.Int63n(100)+1) * time.Millisecond)
			}()
			if i%10 == 0 {
				runtime.Gosched()
			}
		}
		time.Sleep(101 * time.Millisecond)
		waitStart := time.Now()
		for _, ch := range waits {
			<-ch
		}
		dur := time.Since(waitStart)
		if dur > 25*time.Millisecond {
			t.Error("took too long for waiters to resolve", dur)
		}
	})

	t.Run("Lock", func(t *testing.T) {
		count := 0
		thunk := fn.MakeFuture(func() int { count++; return 42 }).Lock()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// tempt the race detector.
		wg := &WaitGroup{}
		wg.DoTimes(ctx, 128, func(context.Context) {
			check.Equal(t, thunk(), 42)
		})
		wg.Wait(ctx)

		check.Equal(t, count, 128)
	})
	t.Run("WithLock", func(t *testing.T) {
		count := 0
		thunk := fn.MakeFuture(func() int { count++; return 42 }).WithLock(&sync.Mutex{})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// tempt the race detector.
		wg := &WaitGroup{}
		wg.DoTimes(ctx, 128, func(context.Context) {
			check.Equal(t, thunk(), 42)
		})

		wg.Wait(ctx)
		check.Equal(t, count, 128)
	})
}
